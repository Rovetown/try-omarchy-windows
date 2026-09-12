#!/usr/bin/env python3
"""Expose a disposable Windows QEMU serial socket as a local byte stream.

Put a symlink named qemu-system-x86_64 to this file on PATH before running
smoke-guest.py --accel tcg. Required environment: SSHPASS (for sshpass),
TRYOMARCHY_SMOKE_ARTIFACTS (local directory), TRYOMARCHY_WINDOWS_GUEST and
TRYOMARCHY_WINDOWS_QEMU (Windows paths). Windows SSH defaults to bts@localhost:2222.
"""
import base64
import os
import signal
import socket
import subprocess
import sys
import threading
import time
import uuid


def quote(value):
    return "'" + value.replace("'", "''") + "'"


def encoded(script):
    return base64.b64encode(script.encode("utf-16le")).decode()


def main():
    local = os.environ["TRYOMARCHY_SMOKE_ARTIFACTS"].rstrip("/")
    guest = os.environ["TRYOMARCHY_WINDOWS_GUEST"].rstrip("/\\")
    executable = os.environ["TRYOMARCHY_WINDOWS_QEMU"]
    arguments = [value.replace(local, guest) for value in sys.argv[1:]]
    with socket.socket() as reservation:
        reservation.bind(("127.0.0.1", 0))
        port = reservation.getsockname()[1]
    arguments[arguments.index("-serial") + 1] = f"tcp:127.0.0.1:{port},server=on,wait=off"
    identity = "try-omarchy-smoke-" + uuid.uuid4().hex
    arguments += ["-name", identity]
    print("Disposable Windows VM: " + identity, file=sys.stderr, flush=True)
    ssh = ["sshpass", "-e", "ssh", "-T", "-o", "ConnectTimeout=10", "-o", "LogLevel=ERROR", "-p", os.environ.get("TRYOMARCHY_VM_SSH_PORT", "2222")]
    host = os.environ.get("TRYOMARCHY_VM_SSH_HOST", "bts@127.0.0.1")
    invoke = [host, "powershell", "-NoProfile", "-NonInteractive", "-EncodedCommand"]
    command = "& " + quote(executable) + " " + " ".join(map(quote, arguments))
    process = subprocess.Popen(ssh + ["-o", "ExitOnForwardFailure=yes", "-L", f"127.0.0.1:{port}:127.0.0.1:{port}"] + invoke + [encoded(command)], stdin=subprocess.DEVNULL, stdout=sys.stderr)
    connection = None
    def terminate(_signum, _frame):
        raise SystemExit(1)
    signal.signal(signal.SIGTERM, terminate)
    try:
        deadline = time.monotonic() + 45
        while time.monotonic() < deadline and process.poll() is None:
            try:
                connection = socket.create_connection(("127.0.0.1", port), timeout=1)
                try:
                    first = connection.recv(1)
                except socket.timeout:
                    first = None
                if first is None or first:
                    if first:
                        sys.stdout.buffer.write(first)
                        sys.stdout.buffer.flush()
                    break
                connection.close()
                connection = None
            except OSError:
                if connection:
                    connection.close()
                connection = None
            time.sleep(.2)
        if connection is None:
            raise RuntimeError("Windows QEMU did not open its serial socket")
        connection.settimeout(None)
        def send():
            try:
                while data := os.read(sys.stdin.fileno(), 65536):
                    connection.sendall(data)
            except OSError:
                pass
        threading.Thread(target=send, daemon=True).start()
        while data := connection.recv(65536):
            sys.stdout.buffer.write(data)
            sys.stdout.buffer.flush()
        return process.wait(timeout=15)
    finally:
        if connection:
            connection.close()
        if process.poll() is None:
            # A parent timeout must not leave an orphaned QEMU on Windows.
            cleanup = "$ErrorActionPreference='Stop'; Get-CimInstance Win32_Process | Where-Object {$_.Name -like 'qemu-system-*' -and $_.CommandLine -like '*" + identity + "*'} | ForEach-Object {Stop-Process -Id $_.ProcessId -Force}"
            try:
                subprocess.run(ssh + invoke + [encoded(cleanup)], stdin=subprocess.DEVNULL, stdout=sys.stderr, timeout=6, check=True)
            finally:
                process.terminate()
                try:
                    process.wait(timeout=2)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()


if __name__ == "__main__":
    sys.exit(main())
