# Resume here

Updated September 14, 2026, after merged PRs #115, #117 and #118.
The user is working toward an official v1 and approved a focused reliability,
recovery and hardware-validation plan. Use [V1-READINESS.md](V1-READINESS.md) and
[issue #77](https://github.com/omacom/try-omarchy-windows/issues/77) for scope and
remaining gates. The older full-feature plan is historical, not the v1 requirement.

## Repository and release state

- Repository: `omacom/try-omarchy-windows`; primary checkout:
  `/home/bts/Projects/try-omarchy-windows`; base branch: `master`.
- Latest implementation merge: `b35c98b989ffd1cedf1d40a6f459b092b60b8ef1`
  ([#118](https://github.com/omacom/try-omarchy-windows/pull/118)). Start from current
  `origin/master`, which also contains this handoff refresh; inspect local status
  before switching branches. Other checkouts can have unrelated work.
- Public Latest remains [v0.0.18-preview](https://github.com/omacom/try-omarchy-windows/releases/tag/v0.0.18-preview),
  published September 13. No newer signed launcher or public release was made.
- Unreleased master includes #113/#114 portable-copy and Windows publication-lock
  improvements, plus #118's runtime ownership repair. The launcher still pins v18.
- No next release tag has been selected. Use [RELEASING.md](RELEASING.md) to prepare
  an exact signed candidate; source merges and guest CI artifacts are not releases.

## Completed in the September 14 sessions

- **#115 merged:** corrected README, compatibility, v1 readiness and test guidance;
  reconciled #77. Closed #87/#89 with shipped-feature/fix instructions and #88 in
  favor of the concrete v1 tracker. #111 received rebase/validation feedback.
- **#117 merged:** investigated #90 using verified v18 artifacts. Fresh image/boot
  has no pacman lock; normal system-package updates succeeded. Killing a real
  pacman fixture in a pre-transaction hook leaves a stale lock across reboot;
  controlled recovery and final reboot preserve data. See
  [package recovery evidence](PACKAGE-RECOVERY-2026-09-14.md).
- **#118 merged; #116 closed:** fixed duplicate Neovim helper ownership. Runtime
  `4.0.3-4` packages only materialized Omarchy commands and preserves the dependency's
  helper file, symlink and administrator edits through upgrade. Compatibility
  revision **21** delivers the corrected repository to existing guests even on
  the same kernel. Patches 0066–0068 contain the fix, five reviewed Arch pin updates,
  and the delivery revision bump. See [ownership evidence](RUNTIME-OWNERSHIP-2026-09-14.md).
- Validation passed: final required PR CI; 114 guest tests (one optional skip);
  complete factory image build/boot; direct v18 package upgrade; all five normal
  updater boots (seed, update, reboot, old external image, return to candidate).
  Database, helper ownership/hashes/symlink, installed packages and user data pass.
  These are Linux/KVM and CI results, not new physical Windows acceptance.

## Reusable candidate and local evidence

The complete guest candidate comes from
[CI run 34904145068](https://github.com/omacom/try-omarchy-windows/actions/runs/34904145068),
built on application commit `9bc4f52`. Artifact `guest-candidate`, ID `10372531418`,
expires September 21, 2026 at 22:31 UTC; a verified local copy is retained.
Archive SHA256:
`ca37c4575df787632f3e7e28dcb83879aadc272df6eafdc0369937f7caf3cb27`.
The factory is Omarchy 4.0.3, runtime `4.0.3-4`, compatibility 21, kernel
`7.2.4-arch1-2`. Runtime r15 for Windows remains the published v18 binary; this
work changed the guest package, not the Windows QEMU runtime.

| Local path beneath `/home/bts/Projects/try-omarchy-evidence/` | Contents |
| --- | --- |
| `issue116-candidate/` | Verified complete new guest artifacts, including decompressed rootfs, metadata and checksums |
| `issue116-full-upgrade/` | Successful five-boot normal-updater test, retained disk and logs |
| `issue116-run04/` | Successful direct package-upgrade/reboot test, copied fixture scripts and disk |
| `issue90-v18/artifacts/` | Verified published v18 baseline, including decompressed rootfs |
| `issue90-v18/run02/` | Successful package-lock interruption/recovery test and retained disk |
| `issue90-v18/builder/` | Reconstructed locked guest builder with patches through 0068 applied |
| `issue116-build-success.log` | Full successful factory-build/boot CI log |

Detailed per-file and log hashes are in the two evidence documents linked above.
Earlier `issue116-run01`–`run03` and `issue90-v18/run01` retain failed investigation
runs; they are not successful candidate evidence. Verify checksums before reuse.

At handoff, no local `qemu-system-x86_64` test process remains. Linux workspace
free space was about 20 GiB; recheck before creating images. The Windows laptop
was not contacted or changed in these sessions, so its older free-space and
process observations are not current facts.

Reproduction entry points:

- `scripts/release/smoke-package-recovery.py`: controlled lock interruption;
  [instructions](GUEST-UPGRADES.md#package-lock-interruption-test).
- `scripts/release/smoke-runtime-ownership.py`: direct packaging/upgrade regression;
  [instructions](RUNTIME-OWNERSHIP-2026-09-14.md#reproduction-and-evidence).
- `scripts/release/smoke-guest-upgrade.py`: normal updater and five-boot preservation;
  [instructions](GUEST-UPGRADES.md#validation). Fixtures now check Neovim helper
  ownership, hashes/symlink and database integrity too.
- `scripts/release/smoke-guest.py` now defaults to compatibility 21 and runtime
  `4.0.3-4`, and checks a clean database. It intentionally fails older defective
  v18 factory-image expectations; use the dedicated baseline-aware runners above
  for historical-image investigation.

## Remaining work and next steps

1. **#119: audit the missing Neovim skeleton template.** v18 lacks packaged
   `/etc/skel/.config/nvim/lua/plugins/theme.lua`; this is distinct from the fixed
   helper ownership. User-visible breakage is not yet established. Investigate
   expected package/default behavior and new-user theming before choosing a fix;
   preserve existing users' personal configuration.
2. **#90: remaining interruption/recovery investigation.** The original reporter's
   stale-lock cause is unknown. Tests cover SIGKILL before package writes, not
   power loss during extraction or scriptlets. Keep active locks protected; no
   automatic lock deletion was added.
3. **Next signed Windows candidate.** Include unreleased master changes and test
   actual launcher/payload upgrades, interruption, forced rollback, both signed
   feeds, pre-transfer installs and old previews skipping the stable bridge.
   Do not call the guest package regression a Windows update/rollback pass.
4. **Physical coverage.** Intel/NVIDIA, full Hyper-V/Core Ultra, advertised Windows
   versions, sleep/resume, mixed-DPI displays, remote input, device switching and
   microphone behavior remain open. Use [TESTING.md](TESTING.md). Also complete
   native Omarchy export/restore acceptance and final support/distribution docs.

Open issues at this checkpoint: #77, #90, #119. Open PR: #111 (borderless), still
optional for v1 and requiring rebase/review/Windows validation. Recheck GitHub
before acting; counts and states can change.

Portable mode stays experimental. Webcam capture, accelerated RAM resume,
arbitrary-app drops, bridged networking, ARM64 and booting a physical install
remain outside the accepted v1 scope. Do not resume the old eight-feature plan
as though all of it blocks v1.

## Earlier Windows evidence and recovery context

Read [the September 13 laptop acceptance](WINDOWS-LAPTOP-ACCEPTANCE-2026-09-13.md)
for physical AMD evidence and the final signed v18 publication record. Earlier
sections describe intermediate failures; use the later explicit retests.

The Windows record includes cleanup operations rejected by automatic approval
review. Their exact targets are retained there; do not retry those deletions
through another route. Re-inventory the host and recoverable data before further
large portable tests. A current session's user instructions take precedence over
historical plans, but old evidence is not permission for new publication, cleanup
or messages to other people.

[Archived session notes](SESSION-RESUME-2026-09-13.md) and
[Windows lab notes](WINDOWS-SESSION-HANDOFF.md) preserve paths and historical
recovery context. Use this document and the v1 tracker for the current direction.
