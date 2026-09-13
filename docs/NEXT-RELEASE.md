# Next release plan

Historical September 6 planning snapshot. The current release assessment and
continuation instructions are in [SESSION-RESUME.md](SESSION-RESUME.md). Do not
select v0.0.15 from this document; later draft candidates already exist.

Proposed target: **v0.0.15-preview**, following v0.0.14-preview at `7fb4fb1`.
Planning snapshot: 2026-09-06. Confirm the next available version before tagging.
This document proposes work; it does not claim implementation or validation.

Implementation status: the branch now contains the move engine and Settings
controls, interruption and data-preservation tests, a measured zero-scan
improvement, a reclaim short-read fix, shared runtime-receipt parsing, and doc
corrections. See [MOVING.md](MOVING.md) and [PERFORMANCE.md](PERFORMANCE.md).
Linux tests, Windows cross-compilation, 228 native Windows tests and a full
Windows move, stale-path launch and retained-source cleanup have passed. See
[combined candidate](COMBINED-CANDIDATE.md) for exact evidence and limitations.
Physical and signed-candidate checks remain gates. No new release is published.

The release should make existing installations easier to manage, improve
measured performance, and remove unnecessary complexity. Stable v1 remains
subject to the physical hardware and upgrade evidence in [issue #77](https://github.com/omacom/try-omarchy-windows/issues/77).

## 1. Measure before changing behavior

- [ ] Establish a repeatable baseline using the current released launcher,
  runtime, and guest, recording their hashes and host details.
- [ ] Record cold and warm launch to guest readiness, launcher and QEMU idle
  CPU separately, memory use, and disk reads/writes. Compare GPU and CPU
  rendering on the same machine and settings.
- [ ] Measure backup, restore, and reclaim time and peak memory on mostly
  empty and populated sparse disks. Record logical size, allocated bytes,
  actual bytes read/written where available, and destination free space.
- [ ] Repeat timings at least five times under comparable conditions and
  report median and range. Use longer idle samples after the desktop settles.
- [ ] Add focused Go benchmarks only for identified hot paths. Keep fixture
  benchmarks separate from Windows end-to-end results.

Audit candidates identified in the current source, not confirmed bottlenecks:

| Area | Evidence and next investigation |
| --- | --- |
| Clipboard | `app/clipboard_bridge.go` polls every 400 ms. Measure unchanged-clipboard work and image allocations before considering Windows change notifications. Preserve reconnect and echo suppression behavior. |
| Reclaim | `app/compact.go` scans the full logical disk in 1 MiB blocks, including existing holes. Evaluate allocated-range scanning and distinguish newly freed allocation from zero ranges processed. |
| Backup and restore | `app/backup.go` streams ZIP data and checksums, with zero detection on restore. Profile compression, zero scanning, progress callbacks, and I/O before changing buffers or formats. |
| Startup | Trace update checks, receipt reads, payload checks, and guest readiness separately. Preserve v0.0.14 reuse of unchanged runtime archives. |
| Background work | Review QMP supervision, agent reconnects, and guest services for unnecessary wakeups, retained goroutines, and repeated work. Keep recovery timeouts justified by behavior. |

Accept performance changes with reproducible before/after evidence and no
material regression in the other recorded metrics. Do not remove verification,
rollback, or compatibility handling to improve a benchmark.

## 2. Move an existing installation from Settings

Add a Storage section showing the current location and **Move installation…**.
Scope the first version to stopped standard installations on supported local
NTFS/ReFS drives. Use the existing Windows folder picker and progress UI.

User flow:

1. Select a destination. Show source, destination, required space, and that
   the original will be retained until the new copy has been checked.
2. Copy to a staging folder, preserve sparse allocation and receipt timestamps,
   verify the copy, and allow cancellation during copying.
3. Switch the saved location, owned shortcuts, stable launcher, and uninstall
   registration to the destination. Relaunch Settings from the new location.
4. Start Omarchy from the new location. After guest readiness is confirmed,
   offer explicit removal of the retained original and show the space it uses.

Implementation requirements:

- [ ] Inventory what must move, including guest state, runtime, settings,
  recovery folders, and launcher-owned metadata. Define handling of unrelated
  files explicitly; never silently discard them. Shared Windows folders and
  external runtime paths remain external references.
- [ ] Reuse existing validation, locking, progress, sparse writing, and owned
  shortcut helpers where their contracts fit. Avoid a temporary ZIP round trip.
- [ ] Reject an occupied destination, source/destination overlap, unsupported
  storage, unsafe reparse paths, active VM, and pending update/recovery work.
  Serialize the operation with launches, settings writes, and other mutations.
- [ ] Budget allocated data plus overhead and verification needs; handle
  free-space loss while copying. Preserve logical disk capacity and contents.
- [ ] Use a durable move journal with explicit copy, verification, activation,
  and cleanup states. Filesystem, pointer, shortcut, and registry changes cannot
  be one atomic transaction; interrupted activation must recover predictably.
- [ ] Handle Windows executable locking through a helper/handoff if needed.
  Prevent old shortcuts or a retained launcher from silently starting a stale
  guest after activation. Define behavior for explicit `-dir` and restored copies
  without changing another installation's default pointer.
- [ ] Keep the original usable before activation. After activation, designate
  the new copy as authoritative; never automatically revert to stale data after
  the user has written files in the moved guest.
- [ ] Restrict cleanup to the verified retained installation. A disconnected
  source or failed cleanup must not invalidate the working destination.

Acceptance evidence: default-to-other-drive, custom-to-custom, move back to
default, same-volume moves, spaces/Unicode/long paths, mostly empty and populated
disks, cancellation, low space, drive disconnect, termination during every
journal phase, stale shortcuts, and partial registry/shortcut update failures.
Compare guest files and disk capacity, boot the moved guest, verify normal
updates and Apps & features removal, and check that unrelated installs survive.
Storage and recovery changes need an independent review before merge.

## 3. Code and documentation cleanup

- [ ] Audit duplicated receipt parsing, path checks, and recovery/UI plumbing.
  Consolidate only where ownership and failure behavior become clearer.
- [ ] Review oversized functions and global state around launch, cancellation,
  and recovery. Extract cohesive responsibilities where it helps the move work;
  avoid a broad launcher rewrite in this release.
- [ ] Remove proven dead code, obsolete comments, unused wrappers, and tests
  that only mirror implementation. Retain old-release compatibility until a
  supported migration path and evidence justify removal.
- [ ] Correct stale `docs/BACKUP.md` space-budget descriptions and old validation
  references against the implementation and recorded results.
- [ ] Refresh `docs/V1-READINESS.md`: distinguish shipped features from untested
  behavior, remove stale image-clipboard scope wording, and fix formatting.
- [ ] Update Settings documentation and add move/recovery checks to TESTING.md.

Every cleanup should identify its concrete benefit. Smaller line counts alone
are not a release outcome, and suspected model provenance is not a reason to
remove code.

## 4. Remaining backlog

The current GitHub backlog is consolidated in #77. Linear's active project item
is SBS-1101, hardware and upgrade validation; there is no dedicated move item yet.

| Work | Treatment for this release |
| --- | --- |
| User documentation | Finish descriptions of current behavior and the new move flow. |
| Idle CPU, sleep/resume, HiDPI, audio switching, microphone activity, remote sessions | Run available physical checks; fix reproducible launcher/guest defects within scope. Record missing hardware coverage. |
| Signed updates and rollback | Exercise the exact preview candidate on a copied existing installation. Keep preview-to-stable and skipped-bridge validation as separate v1 gates. |
| Source-built runtime | Validate and pin only after its physical matrix passes. Do not combine an unvalidated runtime replacement with the storage feature. |
| Configuration export to physical Omarchy | Test on a separate fresh installation if available; leave incomplete until restored packages/theme and excluded VM state are verified. |
| Winget and stable release notes | Prepare when stable distribution details are settled; not a prerequisite for another preview. |
| Drag and drop, multiple monitors, ARM, USB passthrough, physical-install boot | Keep out of this release's implementation scope. |

## 5. Delivery order and release checks

1. Baseline measurements and focused audit findings.
2. Small, independently reviewable performance/cleanup changes with evidence.
3. Move engine and interruption tests, then Settings integration and Windows
   validation. Keep unrelated work in separate branches or worktrees.
4. Documentation, targeted backlog fixes, and one frozen candidate.
5. Run the existing CI contract: release-helper tests, Go race tests and vet,
   Windows build/vet, native Windows tests, guest contract, and release pin checks.
6. Test the candidate on physical Windows: fresh install, existing guest update,
   move and recovery, backup/restore, signed rollback, and normal session behavior.
   Re-run performance comparisons against the same baseline conditions.
7. Prepare concise release notes from completed and verified work. Inspect the
   exact public title/body before submission; publication is a separate action.
   Verify download links, signatures, assets, and versions after publication.

Do not ship the move action until its data-preservation and activation checks
pass. If it is incomplete, retain the preview candidate rather than presenting
the Settings feature as supported. Do not claim stable v1 readiness from CI.
