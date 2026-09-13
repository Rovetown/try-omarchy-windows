#!/usr/bin/env python3
"""Compile the actual token lookup/close/dup and mmap functions with strict OS doubles."""
import argparse
from pathlib import Path
import subprocess
import tempfile

parser = argparse.ArgumentParser()
parser.add_argument('source', type=Path, help='virglrenderer source directory')
args = parser.parse_args()
source = (args.source / 'src/mesa/util/os_file.c').read_text(encoding='utf-8')
mapping = (args.source / 'src/mman_win32.c').read_text(encoding='utf-8')

def function(text, name, result):
    start = text.index('\n' + name + '(') + 1
    brace = text.index('{', start)
    depth = 1
    end = brace + 1
    while depth:
        depth += (text[end] == '{') - (text[end] == '}')
        end += 1
    return result + ' ' + text[start:end] + '\n'

functions = ''.join(function(source, name, result) for name, result in [
    ('os_get_win32_handle_from_fd', 'HANDLE'), ('os_close_fd', 'int'), ('os_dupfd_cloexec', 'int')])
map_function = mapping[mapping.index('void *mmap('):mapping.index('\nint munmap(')]
harness = r'''
#include <assert.h>
#include <errno.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
typedef void *HANDLE;
typedef uint32_t DWORD;
#define INVALID_HANDLE_VALUE ((HANDLE)(intptr_t)-1)
#define FALSE 0
#define DUPLICATE_SAME_ACCESS 2
#define MAP_FAILED ((void *)(intptr_t)-1)
#define MAP_ANONYMOUS 0x20
#define MAP_FIXED 0x10
#define PROT_EXEC 4
static const int token = 0x40000001;
static struct { int token; HANDLE handle; } os_win32_handles[1];
static size_t os_win32_handle_count;
static int os_win32_handle_mutex, locked, crt_calls, close_calls, wrap_failure, map_calls;
static void os_win32_handle_ensure_init(void) {}
static void EnterCriticalSection(int *p) { assert(!locked); locked = 1; }
static void LeaveCriticalSection(int *p) { assert(locked); locked = 0; }
static ptrdiff_t os_win32_handle_find_index_locked(int fd) {
  assert(locked);
  return os_win32_handle_count && fd == token ? 0 : -1;
}
static intptr_t _get_osfhandle(int fd) {
  assert(fd == 7); /* A real CRT must never see a wrapped or stale token. */
  crt_calls++; return 77;
}
static int _close(int fd) { assert(fd == 7); crt_calls++; return 0; }
static int dup(int fd) { assert(fd == 7); crt_calls++; return 8; }
static int CloseHandle(HANDLE h) { assert(h && h != INVALID_HANDLE_VALUE); close_calls++; return 1; }
static bool os_fd_is_handle_token(int fd) { return os_win32_handle_count && fd == token; }
static HANDLE GetCurrentProcess(void) { return (HANDLE)1; }
static int DuplicateHandle(HANDLE p, HANDLE h, HANDLE q, HANDLE *out, int a, int b, int c) {
  assert(h == (HANDLE)123); *out = (HANDLE)456; return 1;
}
static int os_wrap_win32_handle(HANDLE h) { assert(h == (HANDLE)456); return wrap_failure ? -1 : token + 1; }
static DWORD __map_mmap_prot_page(int p) { return p; }
static DWORD __map_mmap_prot_file(int p) { return p; }
static int __map_mman_error(DWORD e, int fallback) { return fallback; }
static DWORD GetLastError(void) { return 5; }
static HANDLE CreateFileMappingW(HANDLE h, void *a, DWORD p, DWORD hi, DWORD lo, void *n) {
  map_calls++;
  if (h == (HANDLE)123) return NULL; /* already a section; borrowed */
  assert(h == (HANDLE)77 || h == INVALID_HANDLE_VALUE);
  return (HANDLE)999;
}
static void *MapViewOfFile(HANDLE h, DWORD a, DWORD hi, DWORD lo, size_t len) {
  assert(h == (HANDLE)123 || h == (HANDLE)999); return (void *)888;
}
static void *MapViewOfFileEx(HANDLE h, DWORD a, DWORD hi, DWORD lo, size_t len, void *addr) {
  assert(addr == (void *)888); return MapViewOfFile(h, a, hi, lo, len);
}
'''
tests = r'''
int main(void) {
  os_win32_handles[0].token = token;
  os_win32_handles[0].handle = (HANDLE)123;
  os_win32_handle_count = 1;
  assert(os_get_win32_handle_from_fd(token) == (HANDLE)123 && !crt_calls);
  assert(os_get_win32_handle_from_fd(token + 9) == INVALID_HANDLE_VALUE && errno == EBADF && !crt_calls);
  assert(os_get_win32_handle_from_fd(-1) == INVALID_HANDLE_VALUE && !crt_calls);
  assert(os_get_win32_handle_from_fd(7) == (HANDLE)77 && crt_calls == 1);
  crt_calls = 0;
  assert(os_dupfd_cloexec(token) == token + 1 && !crt_calls);
  wrap_failure = 1;
  assert(os_dupfd_cloexec(token) == -1 && close_calls == 1);
  assert(os_dupfd_cloexec(token + 9) == -1 && errno == EBADF && !crt_calls);
  assert(mmap(NULL, 4096, 3, 0, token, 0) == (void *)888 && close_calls == 1 && !crt_calls);
  int previous_map_calls = map_calls;
  assert(mmap(NULL, 4096, 3, 0, token + 9, 0) == MAP_FAILED && errno == EBADF && map_calls == previous_map_calls);
  assert(mmap(NULL, 4096, 3, 0, 7, 0) == (void *)888 && close_calls == 2 && crt_calls == 1);
  assert(mmap(NULL, 4096, 3, MAP_ANONYMOUS, -1, 0) == (void *)888 && close_calls == 3);
  assert(os_close_fd(token) == 0 && close_calls == 4 && !os_win32_handle_count);
  crt_calls = 0;
  assert(os_close_fd(token) == -1 && errno == EBADF && !crt_calls);
  assert(os_dupfd_cloexec(token) == -1 && errno == EBADF && !crt_calls);
  assert(os_get_win32_handle_from_fd(token) == INVALID_HANDLE_VALUE && !crt_calls);
  assert(os_close_fd(7) == 0 && crt_calls == 1);
  assert(os_dupfd_cloexec(7) == 8 && crt_calls == 2);
  assert(!locked);
  return 0;
}
'''
with tempfile.TemporaryDirectory(prefix='virgl-handles-') as temp:
    path = Path(temp)
    (path / 'test.c').write_text(harness + functions + map_function + tests, encoding='utf-8')
    subprocess.run(['cc', '-std=c11', '-o', str(path / 'test'), str(path / 'test.c')], check=True)
    subprocess.run([str(path / 'test')], check=True)
print('PASS: actual handle lookup, stale-token rejection, duplicate cleanup, and mapping ownership')
