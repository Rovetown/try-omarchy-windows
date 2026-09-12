#!/usr/bin/env python3
"""Boot the factory image and prove instant provisioning works."""

from __future__ import annotations

import argparse
import base64
import ipaddress
import socket
import json
import os
import re
import selectors
import subprocess
import sys
import time
from pathlib import Path


SUCCESS = b"TRYOMARCHY_SMOKE:omarchy:instant-trial"
# Facts the built image must satisfy, checked from inside the booted guest
# and reported on the serial console as TRYOMARCHY_FACT:<name>:<value>.
FACT_CHECKS = {
    "icon-cache": "sudo gtk-update-icon-cache -f -t /usr/share/icons/hicolor >/dev/null 2>&1 && echo yes || echo no",
    "system-ownership": "test $(stat -c %u:%g /etc) = 0:0 && test $(stat -c %u:%g /usr/lib) = 0:0 && echo yes || echo no",
    "update-repository": "systemctl is-active try-omarchy-update-repository.service 2>/dev/null || true",
    "lock-pam": "test $(stat -c %U:%G:%a /etc/pam.d/omarchy-lock-password) = root:root:644 && pacman -Qo /etc/pam.d/omarchy-lock-password >/dev/null && cmp /etc/pam.d/omarchy-lock-password /usr/share/try-omarchy/omarchy-lock-password && echo yes || echo no",
    "runtime-package": "pacman -Q try-omarchy-runtime | cut -d ' ' -f2",
    "pacman-unlocked": "test ! -e /var/lib/pacman/db.lck && test ! -L /var/lib/pacman/db.lck && echo yes || echo no",
    "omarchy-version": "cat /usr/share/omarchy/version",
    "browser-policy": "test -d /etc/chromium/policies/managed && sudo test -f /etc/sudoers.d/omarchy-theme-browser && echo yes || echo no",
    "browser-theme-unprivileged": "sudo useradd -r -M -G wheel tryomarchy-policy-check && sudo -u tryomarchy-policy-check sudo -n /usr/bin/omarchy-theme-set-browser-policy 123abc >/dev/null && grep -q 123abc /etc/chromium/policies/managed/color.json && echo yes || echo no; sudo userdel tryomarchy-policy-check >/dev/null 2>&1",
    "browser-repair": "sudo rm /etc/sudoers.d/omarchy-theme-browser && sudo mv /etc/chromium/policies/managed /etc/chromium/policies/managed.before-test && sudo /usr/local/lib/try-omarchy/repair-browser-policy >/dev/null && sudo cmp /etc/sudoers.d/omarchy-theme-browser /usr/share/try-omarchy/omarchy-theme-browser && test -d /etc/chromium/policies/managed && echo yes || echo no",
    "media-tools": "command -v pamixer >/dev/null && command -v playerctl >/dev/null && echo yes || echo no",
    "clang": "command -v clang >/dev/null 2>&1 && echo present || echo missing",
    "yay": "pacman -Q yay >/dev/null 2>&1 && echo present || echo missing",
    "omarchy-nvim": "pacman -Q omarchy-nvim >/dev/null 2>&1 && echo present || echo missing",
    "nvim-config": "test -f ~/.config/nvim/init.lua && echo present || echo missing",
    "recorder": "pacman -Q gpu-screen-recorder >/dev/null 2>&1 && echo present || echo missing",
    "foreign": "pacman -Qmq 2>/dev/null | wc -l",
    "sshd": "systemctl is-active sshd 2>/dev/null || true",
    "omarchy-repo-signed": "grep -A2 '^\\[omarchy\\]' /etc/pacman.conf | grep -q TrustAll && echo no || echo yes",
    "input-group": "id -nG | tr ' ' '\\n' | grep -qx input && echo yes || echo no",
    "compat-version": "test \"$(cat /usr/share/try-omarchy/compat-version)\" = \"19:$(uname -r)\" && echo yes || echo no",
    "kernel-modules": "test -f /usr/lib/modules/$(uname -r)/modules.dep.bin && echo yes || echo no",
    "ready-service": "systemctl is-enabled try-omarchy-ready.service 2>/dev/null || true",
}
EXPECTED_FACTS = {
    "icon-cache": "yes",
    "system-ownership": "yes",
    "update-repository": "active",
    "runtime-package": "4.0.3-3",
    "lock-pam": "yes",
    "pacman-unlocked": "yes",
    "omarchy-version": "4.0.3",
    "browser-policy": "yes",
    "media-tools": "yes",
    "browser-repair": "yes",
    "browser-theme-unprivileged": "yes",
    "clang": "present",
    "yay": "present",
    "omarchy-nvim": "present",
    "nvim-config": "present",
    "recorder": "present",
    "foreign": "0",
    "sshd": "inactive",
    "omarchy-repo-signed": "yes",
    "input-group": "no",
    "compat-version": "yes",
    "kernel-modules": "yes",
    "ready-service": "enabled",
}


def parse_facts(transcript: bytes) -> dict[str, str]:
    """Return the last real value printed for each smoke fact.

    The serial console echoes the command before its output and may attach
    terminal escape sequences to the first result, so matches can occur
    anywhere. Echoed printf placeholders are not results.
    """
    facts = {}
    for name, value in re.findall(
        r"TRYOMARCHY_FACT:([A-Za-z0-9-]+):([^\s\x1b'\"\\]+)(?=\s|\x1b|$)",
        transcript.decode("utf-8", errors="replace"),
    ):
        if "%" not in value:
            facts[name] = value
    return facts


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("artifacts", type=Path)
    parser.add_argument("--timeout", type=int, default=600)
    parser.add_argument("--package-update", action="store_true", help="also exercise pacman against current signed repositories in the disposable snapshot")
    parser.add_argument("--displays", type=int, choices=range(1, 17), help="also boot the graphical desktop and verify this many guest displays")
    parser.add_argument("--network-address", help="verify TCP and UDP forwarding through this host IPv4 address")
    parser.add_argument("--accel", choices=("kvm", "tcg"), default="kvm", help="use TCG for nested Windows runtime testing")
    parser.add_argument("--login-delay", type=float, help="wait for provisioning before the first serial login; TCG defaults to 60 seconds")
    parser.add_argument("--compat-revision", type=int, default=19, help="expected guest compatibility revision; use 18 for the signed v17 baseline")
    args = parser.parse_args()
    if not 1 <= args.compat_revision <= 999999:
        parser.error("compatibility revision is invalid")
    FACT_CHECKS["compat-version"] = f'test "$(cat /usr/share/try-omarchy/compat-version)" = "{args.compat_revision}:$(uname -r)" && echo yes || echo no'
    if args.compat_revision >= 19:
        FACT_CHECKS["file-transfer"] = "file-transfer --help >/dev/null 2>&1 && echo present || echo missing"
        EXPECTED_FACTS["file-transfer"] = "present"

    login_delay = args.login_delay if args.login_delay is not None else (60 if args.accel == "tcg" else 0)
    if login_delay < 0:
        parser.error("login delay must not be negative")
    network_ports = []
    if args.network_address:
        ipaddress.IPv4Address(args.network_address)
        for kind in (socket.SOCK_STREAM, socket.SOCK_DGRAM):
            with socket.socket(socket.AF_INET, kind) as probe:
                probe.bind((args.network_address, 0))
                network_ports.append(probe.getsockname()[1])
    if args.package_update:
        FACT_CHECKS["package-update"] = "sudo pacman -Syu --noconfirm >/tmp/tryomarchy-package-update.log 2>&1 && echo yes || { cat /tmp/tryomarchy-package-update.log >&2; echo no; }"
        EXPECTED_FACTS["package-update"] = "yes"

    if args.accel == "kvm" and (not Path("/dev/kvm").exists() or not os.access("/dev/kvm", os.R_OK | os.W_OK)):
        raise SystemExit("release smoke test requires accessible /dev/kvm")

    spec = json.loads((args.artifacts / "build-spec.json").read_text(encoding="utf-8"))
    cmdline = spec["runtime"]["kernelCommandLine"]
    cmdline = cmdline.replace("console=tty0 ", "").replace("console=hvc0", "console=ttyS0")
    cmdline += " tryomarchy.instant=1"
    cmdline += " systemd.unit=graphical.target tryomarchy.render=cpu" if args.displays else " systemd.unit=multi-user.target"
    if args.displays:
        FACT_CHECKS["guest-displays"] = ("export XDG_RUNTIME_DIR=/run/user/$(id -u); "
            "for attempt in $(seq 1 60); do instance=$(hyprctl -j instances 2>/dev/null | jq -r '.[0].instance // empty' 2>/dev/null); "
            "count=$(hyprctl -i \"$instance\" monitors -j 2>/dev/null | jq length 2>/dev/null); "
            f"test \"$count\" = {args.displays} && break; sleep 1; done; echo ${{count:-0}}")
        EXPECTED_FACTS["guest-displays"] = str(args.displays)

    command = [
        "qemu-system-x86_64",
        "-nodefaults",
        "-no-reboot",
        "-snapshot",
        "-accel",
        args.accel,
        "-machine",
        "q35",
        "-cpu",
        "host" if args.accel == "kvm" else "max",
        "-smp",
        "4",
        "-m",
        "4096",
        "-display",
        "none",
        "-monitor",
        "none",
        "-serial",
        "stdio",
        "-drive",
        f"file={args.artifacts / 'rootfs.ext4'},format=raw,if=virtio",
        "-kernel",
        str(args.artifacts / "vmlinuz-linux"),
        "-initrd",
        str(args.artifacts / "initramfs-linux.img"),
        "-append",
        cmdline,
        "-device",
        "virtio-rng-pci",
        "-netdev",
        "user,id=net0" + (f",hostfwd=tcp:{args.network_address}:{network_ports[0]}-:18080,hostfwd=udp:{args.network_address}:{network_ports[1]}-:18081" if args.network_address else ""),
        "-device",
        "virtio-net-pci,netdev=net0",
    ]

    if args.displays:
        device = {"driver": "virtio-gpu-pci", "max_outputs": args.displays,
                  "outputs": [{"name": f"Omarchy {index + 1}", "xres": 1280, "yres": 720} for index in range(args.displays)]}
        command.extend(["-device", json.dumps(device)])

    process = subprocess.Popen(
        command,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        bufsize=0,
    )
    assert process.stdin is not None and process.stdout is not None
    selector = selectors.DefaultSelector()
    selector.register(process.stdout, selectors.EVENT_READ)
    deadline = time.monotonic() + args.timeout
    transcript = bytearray()
    login_attempts = 0
    sent_command = False
    network_checked = False
    password_sent_at: float | None = None
    password_offset = 0
    last_login_prompt = -1
    pending_login_at = None
    last_password_prompt = -1

    try:
        while time.monotonic() < deadline:
            if process.poll() is not None:
                break
            events = selector.select(timeout=1)
            for key, _ in events:
                data = os.read(key.fileobj.fileno(), 65536)
                if not data:
                    continue
                sys.stdout.buffer.write(data)
                sys.stdout.buffer.flush()
                transcript.extend(data)
                if len(transcript) > 1_000_000:
                    del transcript[:-500_000]

                if args.network_address and not network_checked and b"TRYOMARCHY_NETWORK_READY\r\n" in transcript:
                    for kind, port in zip((socket.SOCK_STREAM, socket.SOCK_DGRAM), network_ports):
                        with socket.socket(socket.AF_INET, kind) as client:
                            client.settimeout(10)
                            client.connect((args.network_address, port))
                            payload = b"try-omarchy-forward-" + os.urandom(16)
                            client.sendall(payload)
                            if client.recv(4096) != payload:
                                raise RuntimeError("forwarded service returned different data")
                    network_checked = True
                    process.stdin.write(b"network-complete\n")
                    process.stdin.flush()
                    print("ok - guest TCP and UDP forwarding through " + args.network_address)

                if SUCCESS in transcript:
                    process.wait(timeout=90)
                    facts = parse_facts(bytes(transcript))
                    wrong = {name: (facts.get(name), want) for name, want in EXPECTED_FACTS.items() if facts.get(name) != want}
                    if wrong:
                        raise SystemExit(f"instant guest booted but the image facts are wrong: {wrong}")
                    if args.network_address and not network_checked:
                        raise SystemExit("network forwarding was not exercised")
                    print("ok - instant guest reached a usable trial account")
                    print("ok - image facts: " + ", ".join(f"{k}={facts[k]}" for k in sorted(facts)))
                    return

                login_prompt = transcript.rfind(b"login:")
                if login_prompt > last_login_prompt and login_attempts < 3:
                    pending_login_at = time.monotonic() + (login_delay if login_attempts == 0 else 30)
                    last_login_prompt = login_prompt

                password_prompt = transcript.rfind(b"Password:")
                if password_prompt > last_password_prompt:
                    process.stdin.write(b"omarchy\n")
                    process.stdin.flush()
                    password_sent_at = time.monotonic()
                    password_offset = len(transcript)
                    last_password_prompt = password_prompt

                if password_sent_at is not None and b"Login incorrect" in transcript[password_offset:]:
                    password_sent_at = None

            if pending_login_at is not None and time.monotonic() >= pending_login_at:
                process.stdin.write(b"omarchy\n")
                process.stdin.flush()
                login_attempts += 1
                pending_login_at = None

            if (
                password_sent_at is not None
                and b"targetuser=omarchy;" in transcript[password_offset:]
                and not sent_command
                and time.monotonic() - password_sent_at >= 3
            ):
                checks = "; ".join(
                    f"printf 'TRYOMARCHY_FACT:{name}:%s\\n' \"$({command})\"" for name, command in FACT_CHECKS.items()
                )
                if args.network_address:
                    server = """import socket, threading, time
ready = []
def echo(kind, port):
    s = socket.socket(socket.AF_INET, kind)
    s.bind(('0.0.0.0', port))
    if kind == socket.SOCK_STREAM:
        s.listen(1)
    ready.append(port)
    if kind == socket.SOCK_STREAM:
        c, _ = s.accept()
        with c:
            c.sendall(c.recv(4096))
    else:
        data, peer = s.recvfrom(4096)
        s.sendto(data, peer)
    s.close()
for kind, port in ((socket.SOCK_STREAM, 18080), (socket.SOCK_DGRAM, 18081)):
    threading.Thread(target=echo, args=(kind, port), daemon=True).start()
while len(ready) != 2:
    time.sleep(.05)
print('TRYOMARCHY_NETWORK_READY', flush=True)
time.sleep(60)
"""
                    encoded = base64.b64encode(server.encode()).decode()
                    checks = f"printf %s {encoded} | base64 -d >/tmp/network-smoke.py; python /tmp/network-smoke.py & read -r network_result; " + checks
                process.stdin.write(
                    (checks + "; ").encode()
                    + b"printf 'TRYOMARCHY_SMOKE:%s:%s\\n' \"$(id -un)\" "
                    b"\"$(cat /var/lib/try-omarchy/provision-mode 2>/dev/null)\"; "
                    b"sudo systemctl poweroff\n"
                )
                process.stdin.flush()
                sent_command = True
    finally:
        if process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()

    tail = bytes(transcript[-8000:]).decode("utf-8", errors="replace")
    raise SystemExit(f"instant guest smoke test failed\n\n{tail}")


if __name__ == "__main__":
    main()
