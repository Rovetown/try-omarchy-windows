#!/usr/bin/env python3
"""Compile the runtime's real USB path formatter against boundary fixtures."""
import argparse
from pathlib import Path
import subprocess
import tempfile

parser = argparse.ArgumentParser()
parser.add_argument("source", type=Path)
args = parser.parse_args()
source = args.source.read_text(encoding="utf-8")
start = source.index("static int usb_host_get_port(")
end = source.index("\nstatic void usb_host_libusb_error", start)
function = source[start:end]
harness = r'''
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <assert.h>
#define LIBUSB_API_VERSION 0x01000102
typedef struct { int unused; } libusb_device;
static int count;
static int libusb_get_port_numbers(libusb_device *dev, uint8_t *path, int capacity) {
    (void)dev;
    memset(path, 255, capacity);
    return count;
}
'''+function+r'''
int main(void) {
    unsigned char guarded[50];
    char *port = (char *)guarded + 1;
    memset(guarded, 0xa5, sizeof(guarded));
    count = 7;
    assert(usb_host_get_port(NULL, port, 16) == 0);
    assert(port[0] == 0);
    assert(guarded[0] == 0xa5);
    for (int i = 17; i < sizeof(guarded); i++) assert(guarded[i] == 0xa5);
    assert(usb_host_get_port(NULL, port, 32) == 27);
    assert(strcmp(port, "255.255.255.255.255.255.255") == 0);
    count = 0;
    assert(usb_host_get_port(NULL, port, 16) == 0 && port[0] == 0);
    count = -1;
    assert(usb_host_get_port(NULL, port, 16) == 0 && port[0] == 0);
    port[0] = 'x';
    assert(usb_host_get_port(NULL, port, 0) == 0 && port[0] == 'x');
    return 0;
}
'''
with tempfile.TemporaryDirectory(prefix="try-omarchy-usb-test-") as directory:
    path = Path(directory)
    (path / "test.c").write_text(harness, encoding="utf-8")
    subprocess.run(["gcc", "-std=c11", "-O2", str(path / "test.c"), "-o", str(path / "test.exe")], check=True)
    subprocess.run([str(path / "test.exe")], check=True)
print("ok - USB port path truncation, empty paths, and exact output")
