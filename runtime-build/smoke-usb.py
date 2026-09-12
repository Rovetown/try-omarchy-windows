#!/usr/bin/env python3
"""Verify explicit USB attachment errors without claiming host hardware."""
import argparse
import json
from pathlib import Path
import socket
import subprocess
import time


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("qemu", type=Path)
    args = parser.parse_args()
    with socket.socket() as reservation:
        reservation.bind(("127.0.0.1", 0))
        port = reservation.getsockname()[1]
    process = subprocess.Popen([str(args.qemu.resolve()), "-L", str(args.qemu.resolve().parent / "share"), "-machine", "q35,accel=tcg", "-nodefaults", "-display", "none", "-S", "-m", "128", "-qmp", f"tcp:127.0.0.1:{port},server=on,wait=off"], stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
    try:
        deadline = time.monotonic() + 20
        connection = None
        while time.monotonic() < deadline and process.poll() is None:
            try:
                connection = socket.create_connection(("127.0.0.1", port), timeout=2)
                break
            except OSError:
                time.sleep(.1)
        if connection is None:
            raise RuntimeError("QEMU did not open its control socket")
        connection.settimeout(20)
        with connection, connection.makefile("rb") as stream:
            if "QMP" not in json.loads(stream.readline()):
                raise RuntimeError("missing QMP greeting")
            sequence = 0
            def call(command, arguments=None, expect_error=False):
                nonlocal sequence
                sequence += 1
                request = {"execute": command, "id": sequence}
                if arguments is not None:
                    request["arguments"] = arguments
                connection.sendall(json.dumps(request).encode() + b"\n")
                while True:
                    reply = json.loads(stream.readline())
                    if "event" in reply:
                        continue
                    if reply.get("id") != sequence:
                        raise RuntimeError("uncorrelated QMP reply")
                    if expect_error != ("error" in reply):
                        raise RuntimeError(f"unexpected {command} result: {reply}")
                    return reply.get("return", reply.get("error"))
            call("qmp_capabilities")
            properties = call("device-list-properties", {"typename": "usb-host"})
            if "auto-reconnect" not in {p["name"] for p in properties}:
                raise RuntimeError("runtime lacks verified explicit USB attachment")
            call("device_add", {"driver": "qemu-xhci", "id": "usb-smoke"})
            # libusb bus numbers are uint8_t, so this cannot select real hardware.
            failure = call("device_add", {"driver": "usb-host", "id": "usb-missing", "bus": "usb-smoke.0", "hostbus": 65535, "hostaddr": 127, "hostport": "127", "vendorid": 65535, "productid": 0, "auto-reconnect": False}, expect_error=True)
            if "failed to find host usb device" not in failure["desc"]:
                raise RuntimeError(f"wrong explicit-attachment failure: {failure}")
            objects = call("qom-list", {"path": "/machine/peripheral"})
            if "usb-missing" in {o["name"] for o in objects}:
                raise RuntimeError("failed attachment left a device object")
            call("quit")
            process.wait(timeout=10)
        print("ok - USB controller, explicit attachment failure, and failed-device cleanup")
    finally:
        if process.poll() is None:
            process.kill()
        process.wait()
        if process.stderr:
            detail = process.stderr.read().decode(errors="replace")
            if process.returncode:
                print(detail[-4096:])


if __name__ == "__main__":
    main()
