#!/usr/bin/env python3
"""Exercise the runtime's Windows filename and timestamp helpers on real files."""
import argparse
from pathlib import Path
import shlex
import subprocess
import tempfile

parser = argparse.ArgumentParser()
parser.add_argument("source", type=Path)
args = parser.parse_args()
source = args.source.read_text(encoding="utf-8")
start = source.index("static int win32_error_to_posix(")
end = source.index("/*\n * build_ads_name", start)
helpers = source[start:end]
harness = r'''
#include <assert.h>
#include <errno.h>
#include <stdint.h>
#include <stdio.h>
#include <time.h>
#include <sys/stat.h>
#include <windows.h>
#include <glib.h>
#include <glib/gstdio.h>
''' + helpers + r'''
static uint64_t ticks(FILETIME ft) {
    return ((uint64_t)ft.dwHighDateTime << 32) | ft.dwLowDateTime;
}
int main(void) {
    const char *path = "share-caf\xc3\xa9-\xe6\x97\xa5\xe6\x9c\xac.txt";
    HANDLE file = create_file_utf8(path, GENERIC_READ | GENERIC_WRITE,
        FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE, NULL,
        CREATE_NEW, FILE_ATTRIBUTE_NORMAL, NULL);
    assert(file != INVALID_HANDLE_VALUE);
    assert(file_attributes_utf8(path) != INVALID_FILE_ATTRIBUTES);
    struct stat st;
    assert(stat_utf8(path, &st) == 0);
    struct timespec ts[2] = {{1700000000, 123456700}, {1700000001, 765432100}};
    assert(set_times_win32(file, ts) == 0);
    FILETIME at, mt, before, after;
    assert(GetFileTime(file, NULL, &at, &mt));
    uint64_t original_at = ticks(at), original_mt = ticks(mt);
    assert(original_at == 116444736000000000ULL + 1700000000ULL*10000000 + 1234567);
    assert(original_mt == 116444736000000000ULL + 1700000001ULL*10000000 + 7654321);
    ts[0].tv_nsec = UTIME_OMIT;
    ts[1].tv_nsec = UTIME_NOW;
    GetSystemTimeAsFileTime(&before);
    assert(set_times_win32(file, ts) == 0);
    GetSystemTimeAsFileTime(&after);
    assert(GetFileTime(file, NULL, &at, &mt));
    assert(ticks(at) == original_at);
    assert(ticks(mt) >= ticks(before) && ticks(mt) <= ticks(after));
    original_mt = ticks(mt);
    ts[1].tv_nsec = UTIME_OMIT;
    assert(set_times_win32(file, ts) == 0);
    assert(GetFileTime(file, NULL, &at, &mt));
    assert(ticks(at) == original_at && ticks(mt) == original_mt);
    ts[1].tv_nsec = 1000000000;
    assert(set_times_win32(file, ts) == -1 && errno == EINVAL);
    assert(GetFileTime(file, NULL, &at, &mt));
    assert(ticks(mt) == original_mt);
    GetSystemTimeAsFileTime(&before);
    assert(set_times_win32(file, NULL) == 0);
    GetSystemTimeAsFileTime(&after);
    assert(GetFileTime(file, NULL, &at, &mt));
    assert(ticks(at) >= ticks(before) && ticks(at) <= ticks(after));
    assert(ticks(at) == ticks(mt));
    assert(CloseHandle(file));
    assert(delete_file_utf8(path));
    assert(file_attributes_utf8(path) == INVALID_FILE_ATTRIBUTES);
    return 0;
}
'''
flags = shlex.split(subprocess.check_output(
    ["pkg-config", "--cflags", "--libs", "glib-2.0"], text=True))
with tempfile.TemporaryDirectory(prefix="try-omarchy-share-test-") as directory:
    root = Path(directory)
    (root / "test.c").write_text(harness, encoding="utf-8")
    subprocess.run(["gcc", "-std=gnu11", "-O2", str(root / "test.c"),
                    "-o", str(root / "test.exe"), *flags], check=True)
    subprocess.run([str(root / "test.exe")], cwd=root, check=True)
print("ok - Windows Unicode file APIs; explicit, current, omitted and invalid times")
