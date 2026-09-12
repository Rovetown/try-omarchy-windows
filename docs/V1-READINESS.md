# v1 readiness

The Windows app and the Mac app now live under Omacom. A stable Windows release
still needs the checks below; the open work is tracked in one place, GitHub
issue #77. A passing build does not establish hardware or
upgrade reliability.

The [v0.0.15 candidate report](PREVIEW-15-VALIDATION.md) records the signed
Windows VM checks and the remaining physical acceptance.
The [migration and disk growth report](MIGRATION-17-VALIDATION.md) records
subsequent revision 17 testing. Those fixes are merged but are not in the
existing signed draft.

## Current guest completion work

- [x] Complete the Omarchy 4.0.3 software candidate, including personalized setup,
  lock authentication and existing-guest browser repair (#89). Merged in #93;
  see [candidate evidence](COMBINED-CANDIDATE.md).
- [ ] Reproduce the package lock report (#90) and verify update interruption
  behavior. The candidate build now rejects a locked package database.
- [x] Provide and validate in-place upgrades for the locally pinned Omarchy
  runtime package on existing guests, preserving files and configuration.
  Exact signed-candidate acceptance remains below.

See [the September completion audit](audits/2026-09-12-completion.md) for source
changes, configuration differences and the remaining acceptance work.

## Runtime and release validation

- [ ] Validate the source-built runtime on physical Windows hardware, including full Hyper-V. Record archive hashes and results using
  [RUNTIME-VALIDATION.md](RUNTIME-VALIDATION.md).
- [x] Pin the source-built runtime and matching source archive in
  `guest-build/runtime.lock.json`. The published archives match source revision 3
  and the current build recipe. Physical matrix acceptance remains open.
- [ ] Verify signed draft preparation and publication using
  [RELEASING.md](RELEASING.md). A successful signing check alone does not
  establish that the full release workflow works.
- [ ] Test a copied pre-transfer installation through the signed update path
  into the current candidate. Verify redirects, preserved files, and rollback
  separately from preview-to-stable migration.
- [ ] Prepare release notes and verify public download links, signatures, and
  versions after publication.

## Shipped features requiring candidate regression checks

- [#34](https://github.com/omacom/try-omarchy-windows/pull/34): stable updates,
  including a bridge for installations that skip preview releases.
- [#35](https://github.com/omacom/try-omarchy-windows/pull/35): disk capacity,
  in-place growth, and Windows free-space information.
- [#29](https://github.com/omacom/try-omarchy-windows/pull/29): official artwork,
  current resources, and splash icon handling.
- First-run install-location selection shipped in v0.0.12-preview. Repeat its
  checks on the candidate; moving an existing installation is separate work in
  [NEXT-RELEASE.md](NEXT-RELEASE.md).

## Release gates

- [x] Reproduce and resolve the configuration report in #32, or document its
  confirmed cause and supported fix. Fixed in the v0.0.13 image (guest patch 0033).
- [ ] Test the signed candidate on the Windows hardware matrix, including
  the source runtime, full Hyper-V, and remote input.
- [ ] Record fresh install, existing guest upgrade, interrupted update, and
  forced rollback. Include an old preview that skips the bridge and
  reaches stable through both signed update feeds.
- [ ] Verify disk growth inside the guest, unchanged user files, and unchanged
  capacity after lowering the setting or rolling back the launcher.
- [ ] Restore a configuration export onto a fresh physical Omarchy install.
  Confirm package and theme restoration and exclusion of VM-specific state.
- [x] Finish stopped-VM backup/restore and a clear reset flow before presenting
  the guest as suitable for persistent work. Shipped in v0.0.12. Configuration export is not a VM
  backup.
- [ ] Exercise sleep/resume, mixed-DPI resize, audio-device changes, and a long
  session. Record idle CPU and whether launch alone activates the microphone.
- [ ] Update user documentation to describe the tested release, including how
  existing guests receive Omarchy OS updates separately from launcher updates.

Use [TESTING.md](TESTING.md) for reports. Each gate needs evidence for the exact
candidate version and runtime, not only an earlier preview. Keep release
publication separate from code review and merging.

## Scope

v1 should provide a dependable way to try Omarchy, keep a trial setup, and take
its configuration to a full installation. Prioritize reliability, storage,
recovery, and understandable controls.

Image clipboard shipped in v0.0.13-preview. Better file transfers, camera
bridging, Windows ARM64, and additional portable launchers remain later work.
Booting an existing physical installation is outside the v1 scope.
