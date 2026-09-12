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

## Additional recovery and restore checks

- Restored an export from the signed candidate onto a separate fresh Windows
  guest using real package and theme commands. The archive carried `cowsay`
  and Catppuccin. Both were restored successfully, while destination-specific
  monitor configuration, the runtime link, and original backups were preserved.
  A second restore retained separate backups and skipped the installed package.
- Exercised the signed launcher's staged update helper with the authenticated
  v16 executable and the exact signed v15 executable as its previous version.
  Interrupted after QMP connected, before userspace readiness, while both
  launcher and guest update markers were pending. Recovery restored both v15
  hashes, cleared the markers, and booted the preserved guest. Its document,
  installed package, and theme survived.
- Retried the staged update without interruption. Both v16 components committed
  after guest readiness, their hashes matched, and the same guest data survived.
- A low-space attempt refused the guest update before replacing its payload.
  Launcher recovery restored v15 and the guest remained usable. The dedicated
  Windows test volume was then expanded without deleting earlier evidence.

The initial isolated-home restore used stubbed commands; the later guest-to-guest
restore exercised the real repository package installer and theme selector.
It did not test AUR installation or restoration onto physical native Omarchy.

## Update authentication and public routes

- Fourteen cases exercised production `fetchUpdateManifest` with the unchanged
  embedded public key and genuine v15/v16 signatures. Genuine metadata passed;
  altered metadata/signatures, truncated responses, and HTTP 404 responses were
  rejected. The tests passed with Go's race detector.
- The signed v15 launcher accepted genuine v16 metadata served locally, then
  encountered the expected HTTP 404 at the authenticated GitHub draft URL.
  Its executable and guest receipt remained unchanged, with no pending update.
  Booting with the production v15 URL also encountered a draft 404 because this
  test installation had been prepared from a local asset server.
- Current public v14 assets passed through official and legacy repository names,
  with tagged and latest URLs. All 32 requests succeeded after redirects. Both
  signed feeds, the pinned manifest, and executable hashes agreed. Rootfs
  availability was checked with HEAD; the image was not downloaded in this check.

The staged transaction used locally authenticated files and an explicit local
payload URL. It does not establish the complete automatic update path from an
unpublished draft. No production trust key, repository allowlist, or TLS trust
was changed for these tests.

## Full Hyper-V lab attempt

Enabling `Microsoft-Hyper-V-All` required a Windows restart. The nested Windows
lab then remained at the boot screen with SSH unavailable, before Try Omarchy
could start. This attempt does not establish candidate compatibility or a
candidate regression. Full Hyper-V acceptance still needs a suitable Windows
host. The earlier tests used Windows Hypervisor Platform without the full role.

## Remaining acceptance

The Windows host was a Windows 11 Enterprise VM using CPU rendering and an
audio fallback. Physical GPU/audio, full Hyper-V, remote input, sleep/resume,
mixed-DPI, and long-session acceptance remain open. Restore onto a fresh
physical Omarchy installation still needs real package and theme validation.

The [revision 17 report](MIGRATION-17-VALIDATION.md) records disk growth and
preservation after lowering the setting with a local payload.

Draft assets were served locally for the Windows tests. Anonymous delivery of
the v16 assets and final tagged/latest feeds must be checked during controlled
publication, since drafts are not anonymously downloadable. Preview-to-stable
migration by an old executable through both signed feeds still needs end-to-end
acceptance before v1. Keep pre-publication hardware gates in
[V1-READINESS.md](V1-READINESS.md) open until they are tested on suitable machines.
