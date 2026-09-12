#!/usr/bin/env python3
"""Compile the pinned Windows close helper against handle-flag semantics."""
import argparse
from pathlib import Path
import subprocess
import tempfile

parser = argparse.ArgumentParser()
parser.add_argument("source", type=Path)
args = parser.parse_args()
source = args.source.read_text()
start = source.index("int qemu_close_socket_osfhandle(int fd)")
end = source.index("\nint qemu_close_wrap", start)
helper = source[start:end]
fixture = r'''
#include <assert.h>
#include <errno.h>
#include <stdint.h>
typedef intptr_t SOCKET;
typedef intptr_t HANDLE;
typedef unsigned long DWORD;
#define HANDLE_FLAG_PROTECT_FROM_CLOSE 2
#define __try1(handler) if (1)
#define __except1 else
static DWORD handle_flags;
static int fd_closed;
static SOCKET _get_osfhandle(int fd) { assert(fd == 7); return 41; }
static int GetHandleInformation(HANDLE h, DWORD *flags) {
    assert(h == 41); *flags = handle_flags; return 1;
}
static int SetHandleInformation(HANDLE h, DWORD mask, DWORD flags) {
    assert(h == 41); handle_flags = (handle_flags & ~mask) | (flags & mask); return 1;
}
static int fixture_close(int fd) {
    assert(fd == 7);
    assert(handle_flags & HANDLE_FLAG_PROTECT_FROM_CLOSE);
    fd_closed = 1; errno = EBADF; return -1;
}
#define close fixture_close
'''
fixture += helper
fixture += r'''
int main(void) {
    for (DWORD initial = 0; initial < 4; initial++) {
        handle_flags = initial;
        fd_closed = 0;
        assert(qemu_close_socket_osfhandle(7) == 0);
        assert(fd_closed);
        assert(handle_flags == initial);
    }
    return 0;
}
'''
with tempfile.TemporaryDirectory() as temp:
    c = Path(temp) / "socket-close.c"
    exe = Path(temp) / "socket-close.exe"
    c.write_text(fixture)
    subprocess.run(["gcc", "-std=c11", "-Wall", "-Wextra", "-Werror", str(c), "-o", str(exe)], check=True)
    subprocess.run([str(exe)], check=True)
print("ok - Windows socket close restores both zero and nonzero handle flags")
