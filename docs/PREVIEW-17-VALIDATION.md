# v0.0.17-preview candidate validation

The release is an unpublished draft. It combines file clipboard, reclaim
controls and Settings improvements with the Omarchy 4.0.3 guest. See
[COMPLETION-CANDIDATE.md](COMPLETION-CANDIDATE.md) for implementation limits
and the planned Windows acceptance round.

## Exact candidate

- Guest source: `8a99aa4d214c6b5edc8efa3008df8d5a7aef67af` (PR #106).
- Launcher source: `3869987b35f556e779f132c722c258fd14bf5687` (PR #108).
- [Guest preparation](https://github.com/omacom/try-omarchy-windows/actions/runs/34713001736) and [signing](https://github.com/omacom/try-omarchy-windows/actions/runs/34713854531) passed on master.
- Launcher SHA256: `5b53cfbe4cbe551bb2ef54f4aa643d109e8c2d123c08564293f4d04c545b84a3`.
- Payload SHA256SUMS digest: `dd880c291739ae3ea9311109568cb0e8008c3f0ba77c9bcaeed5609f1f719e1d`.
- Raw rootfs SHA256: `96bf7666837654e1bb7db4abdedeff379d7b344181c0a71ce73bd61c18d0a26d`.
- Compressed rootfs SHA256: `0f1d77237e33b3d772bc56b3bc0138e61c2b70f37e1b1ec5a181f3f3b862d09e`.
- Compressed rootfs size: 2,001,474,291 bytes, below GitHub's 2 GiB asset limit.
- Omarchy 4.0.3, runtime package 4.0.3-3, compatibility revision 18.

All downloaded payload assets match the authenticated manifest, and the
runtime archives match the runtime lock. The decompressed rootfs also matches.
Windows reports valid Authenticode and v0.0.17-preview resources. The signed
update metadata verifies against the unchanged embedded Ed25519 key and names
the same launcher and payload hashes. No release was published.

## Linux guest regression

The baseline was decompressed from the downloaded signed v16 payload and
verified against its manifest. Tests used a separate writable copy under KVM.
All five stages passed: seed user files and a system configuration change,
boot with v17 and run Omarchy Update, reboot, return to the v16 kernel and
initramfs, then return to v17. File hashes and the package inventory survived;
the local update repository stayed active and the package database was unlocked.
The deliberately test-owned package lock blocked an update without being changed.
This reproduces lock refusal, not the origin of issue #90's orphaned lock.

A sixth boot verified the revision 18 marker and exact installed hashes of the
file clipboard helper, bridge and reclaim agent. The installed file helper
packed and restored a Unicode folder, binary file and empty directory, preserved
Cut originals, and discarded only the received snapshot. User preservation
hashes still matched. This did not exercise Explorer or a graphical guest clipboard.

## Signed Settings regression

The first signed launcher exposed two Settings defects at a 500-pixel work
area: focus handling reset scrolling to the top, and the scrollbar reduced the
intended client width from 480 to 463 pixels. PR #108 restricts focus reveal to
keyboard navigation and includes scrollbar width in the window calculation.
The earlier executable is retained locally for comparison. The corrected
development build passed the native UI test: 484-pixel window in a 500-pixel
work area, 480-pixel client width, visible Save/Cancel/Help after bottom scrolling,
persistent mouse-wheel scrolling, and 60 Tab/Shift+Tab focus checks.
The final signed executable listed above passed the same full UI check.
`preview17-settings-validation.json` records its hash and measured geometry.

## Evidence and remaining acceptance

Linux assets and logs are under `/data/try-omarchy-upgrade-lab/preview17-assets`
and `preview17-upgrade-acceptance`. Signed files and Windows reports are under
`/home/bts/Windows/try-omarchy-combined/signed-preview17` and the parent directory.

The physical Windows matrix, graphical file clipboard, actual disk reclaim,
full Hyper-V, native physical restore, and public automatic update delivery
remain open. Earlier signed v16 tests remain useful baseline evidence but do
not replace regression checks against this exact candidate. Follow
[WINDOWS-SESSION-HANDOFF.md](WINDOWS-SESSION-HANDOFF.md) before testing.
