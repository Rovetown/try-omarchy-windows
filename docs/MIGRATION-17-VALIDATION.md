# Migration and disk growth validation

Tested September 12, 2026. Migration fixes are merged in
[PR #100](https://github.com/omacom/try-omarchy-windows/pull/100).
The v0.0.15-preview draft still contains compatibility revision 16 and does not
include these fixes. A new signed release candidate is required.

## Candidate

- Guest builder commit: `3c212ba36236e339436b32e7bce0303302834476`.
- Omarchy 4.0.3, runtime package 4.0.3-3, compatibility revision 17.
- Compressed image SHA256: `c43fb175502648c2035645a20ae83fbc857c9de780d5a881b3258d4df425d313`.
- Compressed size: 2,001,497,960 bytes, below GitHub's 2 GiB asset limit.
- Test payload SHA256SUMS digest: `3d67e65a906fd516f11ab8ba3cdf5d9bbbaa3135dfdc50179a209805eb8a6d7d`.

The Windows checks used the previously signed v0.0.15 launcher with an explicit
local payload override. Its embedded release pin still identifies revision 16;
these results do not establish acceptance of a newly signed release.

## Passed

- 40 guest overlay tests, 15 release helper tests, and the guest contract.
- Headless first boot and a package update against the current repositories.
- Upgrade of the existing Windows test guest from revision 16 to 17. The
  launcher committed the update after guest userspace reported ready.
- Growth to 40 GiB: the guest reported 42,949,672,960 block-device bytes and
  42,164,764,672 filesystem bytes. The existing persistence file retained its
  SHA256, and no user services were failed.
- Lowering the requested capacity to 24 GiB on the next launch retained the
  40 GiB disk, the same filesystem size, and the unchanged persistence file.
- Export using the installed revision 17 script. Two restores into an isolated
  home preserved native monitor files, the native Omarchy runtime location,
  and separate backups containing the original shell configuration.
- Clean guest shutdown.

Package and theme commands were stubbed during the isolated restore. The tests
also cover missing helpers and failed package/theme commands reporting an
incomplete restore, and linked Hyprland directories requiring manual review.

## Remaining

Restore onto a fresh physical Omarchy installation, including real package and
theme restoration. Repeat acceptance with the final signed candidate.

The Windows host was a Windows 11 Enterprise VM using CPU rendering and an
audio fallback. Physical GPU, audio, full Hyper-V, remote input, sleep/resume,
and mixed-DPI acceptance remain open.
