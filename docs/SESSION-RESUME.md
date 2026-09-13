# Resume here

Updated September 12, 2026, after the native file-transfer and recovery round.

## Checkout and current state

- Worktree: `/home/bts/Projects/try-omarchy-full-features`.
- Branch: `codex/full-feature-completion`.
- PR: [#110](https://github.com/omacom/try-omarchy-windows/pull/110), open and draft.
- Tested code and evidence checkpoint: `d8e463e6a1ec4e6b5baae703dbd48e1674e4ba78`.
  All three required CI jobs passed; the optional guest build is a separate run.
- GitHub identity must be `btsouth`. For CLI calls use
  `GH_TOKEN="$(gh auth token --user btsouth)" gh ...`.
- Inspect status before editing. Other Try Omarchy checkouts and runtime source
  trees may contain unrelated changes; do not reset them.
- The user wants the entire eight-feature scope completed. An intermediate
  preview must not silently replace that goal. No Basecamp updates are requested.
  Access Linear only through Toolport.

Read [FULL-FEATURE-COMPLETION.md](FULL-FEATURE-COMPLETION.md) for the full scope,
[PR-110-REVIEW.md](PR-110-REVIEW.md) for evidence and
[WINDOWS-SESSION-HANDOFF.md](WINDOWS-SESSION-HANDOFF.md) for artifacts and lab setup.
The dated implementation sections in those documents are a history; their older
"next" statements can have been superseded by later results.

## Finished in the last round

- Runtime r7 reports bounded native SDL file-drop events. Actual OLE dragging
  from the Windows transfer window into SDL passed, including repeated runs.
- Windows and guest GTK transfer windows use verified streaming tickets without
  replacing the clipboard. Guest patch 0063 fixes outgoing archives being staged
  in the small `/run/user` tmpfs; archives now use the disk-backed XDG cache.
- Clipboard capability negotiation accepts late greetings on the same connection.
- Failed saved-session captures reconnect to QMP, settle their disk jobs and
  resume the original guest. Lost replies and real QEMU failure paths are tested.
- The interactive Windows suite passed 344 tests. Four Python interop tests pass
  on Linux; the native firewall test has separate earlier evidence.
- The rebuilt Omarchy 4.0.3 / compatibility-19 guest passed a real Windows TCG
  31 MiB Unicode-file round trip in both directions. Both transfer windows,
  file integrity, unchanged originals and unchanged clipboards were verified.
- Clean guest contract: 102 tests run, one native GTK opt-in skipped. That GTK
  adapter has separate native evidence. Linux race tests and Windows builds pass.

## Artifacts to reuse

Use runtime `D:\TryOmarchyFullFeaturesTest\runtime-r7` and guest
`D:\TryOmarchyFullFeaturesTest\guest19-r2`. The older `guest19` has the outgoing
staging bug. Exact hashes and build links are in the Windows handoff.

Linux guest artifacts are under
`/data/try-omarchy-feature-guest-artifacts/compat19-r2`; the raw rootfs was
stream-verified and is expanded on Windows. The directory also retains
`windows-transfer-smoke.log` and `windows-image-verification.log`.
Runtime artifacts are under `/data/try-omarchy-feature-runtime-artifacts/r7`.
Guest build [34731444397](https://github.com/omacom/try-omarchy-windows/actions/runs/34731444397)
passed all jobs. Downloaded local copies are retained beyond CI artifact expiry.

The complete guest test executable is `drop-session.test.exe`; the earlier
344-test executable is retained as `drop-session-full-tested.exe` in the Windows
lab. Use the hashes in the handoff to distinguish them. Reproduction commands
are in [scripts/vmtest/README.md](../scripts/vmtest/README.md).

The lab was left idle after successful shutdown. Its `TryOmarchySessionSuite`
scheduled task is a manual test helper, currently configured for the isolated
file-transfer fixture. Recheck task/process state before starting another test.
The nested lab has hypervisor startup disabled following its earlier boot
failure. Its TCG evidence does not prove physical WHPX, GPU or full Hyper-V
compatibility. Do not re-enable that configuration without its recovery plan.

## Concrete continuation order

1. Continue saved-session integration from `app/saved_session_lifecycle.go`,
   `saved_session_store.go`, `saved_session_disk.go` and `saved_session_runtime.go`.
   The engine is tested; production launcher save/resume controls and WHPX/virgl
   state preservation still need implementation. Keep real runtime blockers
   intact until state preservation is implemented and proven. Do not present a
   synthetic TCG memory test as accelerated desktop resume acceptance.
2. Complete the remaining planned implementations: direct drop placement into
   applications, the camera bridge, bridged networking and the native ARM64
   runtime/guest/release path. Audit checkpoint incremental storage and the
   remaining lifecycle work against the full-feature document. Cross-compiling
   the launcher alone does not establish ARM64 product support.
3. Close physical acceptance for snapshot/portable recovery, multiple displays,
   mixed DPI, USB devices, GPU behavior and full Hyper-V coexistence. Test a
   candidate combining the current launcher, r7 runtime and corrected guest.
   Feed failures back into code before marking those features complete.
4. Prepare and sign the integrated release candidate through
   [RELEASING.md](RELEASING.md), preserving the current signed baseline. Verify
   fresh installation, real existing-install upgrade, interrupted update and
   recovery against that exact signed candidate, then official download/update
   delivery before publication.

## How close is the next release?

A substantial preview is close enough to move into integrated candidate
packaging and physical acceptance if the user chooses an intermediate release.
The current components and automated coverage provide a strong starting point,
but the new feature set has not yet passed that signed candidate round.

The user's full-feature release remains an engineering project, not just a
Windows test pass away. Accelerated saved sessions are the largest uncertainty;
camera, bridged networking, direct application drops and native ARM64 also need
implementation. Do not give a completion percentage or date from test counts.

Live release state checked for this handoff: public latest is v0.0.14-preview;
v0.0.15, v0.0.16 and v0.0.17-preview are unpublished drafts. The signed v17
candidate predates PR #110 and is only a regression baseline for this work.
No new release version has been selected or published. Recheck remote state
before choosing a tag, merging or changing release metadata.
