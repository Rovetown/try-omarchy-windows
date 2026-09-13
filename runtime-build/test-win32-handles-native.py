#!/usr/bin/env python3
"""Exercise real section handles through the built Windows renderer DLL."""
import argparse
import ctypes as c
from ctypes import wintypes as w
import json
import os
from pathlib import Path
import sys

parser = argparse.ArgumentParser()
parser.add_argument('library', type=Path)
args = parser.parse_args()
if sys.platform != 'win32':
    parser.error('This test requires native Windows')
library = args.library.resolve(strict=True)
size = 65536
invalid = c.c_void_p(-1).value
kernel = c.WinDLL('kernel32', use_last_error=True)
kernel.CreateFileMappingW.argtypes = [w.HANDLE, c.c_void_p, w.DWORD, w.DWORD, w.DWORD, w.LPCWSTR]
kernel.CreateFileMappingW.restype = w.HANDLE
kernel.CloseHandle.argtypes = [w.HANDLE]
kernel.CloseHandle.restype = w.BOOL
crt = c.CDLL('ucrtbase')
callback_type = c.CFUNCTYPE(None, w.LPCWSTR, w.LPCWSTR, w.LPCWSTR, c.c_uint, c.c_size_t)
invalid_calls = []
@callback_type
def invalid_parameter(expression, function, file, line, reserved):
    invalid_calls.append('CRT invalid parameter')
crt._set_thread_local_invalid_parameter_handler.argtypes = [c.c_void_p]
crt._set_thread_local_invalid_parameter_handler.restype = c.c_void_p
previous = crt._set_thread_local_invalid_parameter_handler(c.cast(invalid_parameter, c.c_void_p))
failures = []
tokens = []
views = []
section = None
try:
    with os.add_dll_directory(str(library.parent)):
        dll = c.CDLL(str(library))
        dll.os_wrap_win32_handle.argtypes = [w.HANDLE]
        dll.os_wrap_win32_handle.restype = c.c_int
        dll.os_get_win32_handle_from_fd.argtypes = [c.c_int]
        dll.os_get_win32_handle_from_fd.restype = w.HANDLE
        dll.os_dupfd_cloexec.argtypes = [c.c_int]
        dll.os_dupfd_cloexec.restype = c.c_int
        dll.os_close_fd.argtypes = [c.c_int]
        dll.os_close_fd.restype = c.c_int
        dll.mmap.argtypes = [c.c_void_p, c.c_size_t, c.c_int, c.c_int, c.c_int, c.c_ssize_t]
        dll.mmap.restype = c.c_void_p
        dll.munmap.argtypes = [c.c_void_p, c.c_size_t]
        dll.munmap.restype = c.c_int
        try:
            section = kernel.CreateFileMappingW(w.HANDLE(-1), None, 4, 0, size, None)
            if not section:
                raise c.WinError(c.get_last_error())
            token = dll.os_wrap_win32_handle(section)
            if token < 0:
                raise RuntimeError('Wrapping section failed')
            tokens.append(token)
            expected = section
            section = None  # token now owns it
            if dll.os_get_win32_handle_from_fd(token) != expected:
                failures.append('Token lookup returned the wrong handle')
            duplicate = dll.os_dupfd_cloexec(token)
            if duplicate < 0:
                raise RuntimeError('Duplicating token failed')
            tokens.append(duplicate)
            for current in (token, duplicate):
                view = dll.mmap(None, size, 3, 1, current, 0)
                if view in (None, invalid):
                    failures.append('Wrapped section could not be mapped')
                else:
                    views.append(view)
            for current in tuple(tokens):
                if dll.os_close_fd(current) != 0:
                    raise RuntimeError('Closing owned token failed')
                tokens.remove(current)
            if len(views) == 2:
                c.memset(views[0], 0xa5, size)
                if c.string_at(views[1], size) != b'\xa5' * size:
                    failures.append('Independent view lost shared bytes after handles closed')
            if dll.os_get_win32_handle_from_fd(token) != invalid:
                failures.append('Stale token was accepted')
            if dll.os_dupfd_cloexec(token) != -1 or dll.os_close_fd(token) != -1:
                failures.append('Stale token operation was accepted')
        finally:
            for view in views:
                if dll.munmap(view, size) != 0:
                    failures.append('View cleanup failed')
            for token in tokens:
                dll.os_close_fd(token)
            if section:
                kernel.CloseHandle(section)
finally:
    crt._set_thread_local_invalid_parameter_handler(previous)
if invalid_calls:
    failures.append(f'{len(invalid_calls)} CRT invalid-parameter calls')
print(json.dumps({'result': 'FAIL' if failures else 'PASS', 'failures': failures,
                  'views': len(views), 'bytesVerified': size if len(views) == 2 else 0,
                  'scope': 'actual renderer DLL token ownership and shared mapping; not Vulkan playback'}, indent=2))
sys.exit(1 if failures else 0)
