# v0.0.16-preview candidate validation

Tested September 12, 2026. The release remains an unpublished draft.
Omarchy 4.0.3 is the current upstream release.

## Exact candidate

- Launcher source: `aa66dea9fe5ac95ede0dfaf358f55936f00205f1`.
- [Guest preparation](https://github.com/omacom/try-omarchy-windows/actions/runs/34707042607) and [signing](https://github.com/omacom/try-omarchy-windows/actions/runs/34707526208) passed on master.
- Launcher SHA256: `b5046d62f0c74d643be0abae3482a223034b5284e231e42bda30c0658c38f128`.
- Payload SHA256SUMS digest: `76bd981fe756653ccd79e4ebd22afbc4c20a7442946d500ce8de7864fd8c06f9`.
- Compressed rootfs SHA256: `4e57a8151e8c5079c8ff3939f975baefa9d75439ae31529b397375ca37dca2fe`.
- Compressed rootfs size: 2,002,388,343 bytes, below GitHub's 2 GiB asset limit.
- Omarchy 4.0.3, runtime package 4.0.3-3, compatibility revision 17.

The downloaded assets match their authenticated checksums. Their build spec and
package lock match the locally tested revision 17 build; both runtime archives
match the repository lock. Windows reports a valid Authenticode signature and
v0.0.16 version resources. The update metadata signature verifies against the
embedded public key and identifies the same launcher and payload hashes.

## Windows VM results

- Fresh installation reached the desktop with compatibility revision 17 and
  the expected packages. Chromium started headlessly with its sandbox enabled;
  the package database was unlocked and no user services were failed.
- Reboot preserved the fresh-install test file with the same SHA256. The
  second boot had no failed user services or package lock, and shutdown was clean.
- Upgraded the existing v0.0.15 test installation. The launcher committed the
  payload after guest userspace reported ready, and both existing document
  hashes remained unchanged.
- The upgraded guest had no failed user services or package database lock.
- The installed revision 17 exporter produced an archive that restored twice
  into an isolated home, preserving native monitor settings, the native runtime
  location, and original configuration backups.

The isolated restore used stubbed package and theme commands. It does not
establish native Omarchy package or theme restoration.

## Remaining acceptance

The Windows host was a Windows 11 Enterprise VM using CPU rendering and an
audio fallback. Physical GPU/audio, full Hyper-V, remote input, sleep/resume,
mixed-DPI, and long-session acceptance remain open. Restore onto a fresh
physical Omarchy installation still needs real package and theme validation.

The [v0.0.15 report](PREVIEW-15-VALIDATION.md) records the earlier signed
interruption and rollback checks. Those results are not a repeat against the
v0.0.16 payload. The [revision 17 report](MIGRATION-17-VALIDATION.md) records disk
growth and preservation after lowering the setting with a local payload.

Draft assets were served locally for the Windows tests. Anonymous pre-transfer
updates and preview-to-stable migration through both signed feeds still need
end-to-end acceptance. Keep the draft unpublished until the release gates in
[V1-READINESS.md](V1-READINESS.md) are satisfied.
