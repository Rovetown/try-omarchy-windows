# Windows session handoff

Updated September 12, 2026. Start here before further release testing.

Use **v0.0.17-preview** for the next combined testing round. It includes file
clipboard, reclaim controls and scrolling Settings, with compatibility revision
18. Read [PREVIEW-17-VALIDATION.md](PREVIEW-17-VALIDATION.md) for exact artifacts
and completed checks, and [COMPLETION-CANDIDATE.md](COMPLETION-CANDIDATE.md) for
the test matrix. Signed v16 does not include these features.

## Current release state

The signed v0.0.17-preview candidate remains an unpublished draft. Its launcher
source is `3869987b35f556e779f132c722c258fd14bf5687`. PR #106 contains the feature
implementation, #107 pins the guest, and #108 fixes Settings scrolling. Public latest
remains v0.0.14-preview as last checked. Recheck live state before acting.

- Omarchy 4.0.3, runtime package 4.0.3-3, guest compatibility revision 18.
- Launcher SHA256: `5b53cfbe4cbe551bb2ef54f4aa643d109e8c2d123c08564293f4d04c545b84a3`.
- SHA256SUMS digest: `dd880c291739ae3ea9311109568cb0e8008c3f0ba77c9bcaeed5609f1f719e1d`.
- Raw rootfs SHA256: `96bf7666837654e1bb7db4abdedeff379d7b344181c0a71ce73bd61c18d0a26d`.
- [Guest preparation](https://github.com/omacom/try-omarchy-windows/actions/runs/34713001736) and [signing](https://github.com/omacom/try-omarchy-windows/actions/runs/34713854531) succeeded.

Read [RUNTIME-VALIDATION.md](RUNTIME-VALIDATION.md) for physical coverage and
[RELEASING.md](RELEASING.md) before publication. NEXT-RELEASE.md is an older
planning snapshot. The following v16 results are baseline evidence, not a
substitute for the combined v17 Windows round.

## Earlier v16 Windows results

Fresh signed installation, existing-guest upgrade, reboot, sandboxed Chromium,
configuration export, and persistence checks passed in a nested Windows VM.
Real guest-to-guest restore installed cowsay and selected Catppuccin, preserved
destination monitor configuration and runtime link, and retained separate
backups across repeated restores. Physical native Omarchy and AUR restoration
were not tested.

The production signed update helper was tested with authenticated local files:
interruption before guest readiness restored the exact v15 launcher and guest;
retry committed v16; low-space failure recovered safely. The document, package,
and theme survived. This was not a complete automatic public feed download.
Fourteen production-key authentication cases passed with Go's race detector.
Thirty-two current public v14 feed/asset requests passed through official and
legacy repository names, including tagged/latest routes. These do not prove
public v16 delivery or old-preview-to-stable migration.

Independent evidence review and all three PR #104 CI checks passed.

## Important: the nested Windows lab is in recovery configuration

Enabling Microsoft-Hyper-V-All prevented the outer Windows VM from finishing
boot, before Try Omarchy could start. Windows Recovery successfully disabled
the full role, but normal hypervisor startup still stalled. Windows was restored
to a bootable state with `hypervisorlaunchtype Off`. SSH and the candidate
launcher hash were verified afterward at 14:24 EDT.

Final feature state: Microsoft-Hyper-V-All and Microsoft-Hyper-V-Hypervisor
Disabled; HypervisorPlatform Enabled; hypervisor startup Off. Accelerated
nested guest testing is unavailable in this state. Do not simply enable startup
again: that already reproduced the boot stall. This is a lab limitation, not a
proven candidate defect or a full Hyper-V pass. All candidate disks were retained.

Recovery used WinRE Command Prompt: offline DISM to disable the full role, then
`bcdedit /set {default} hypervisorlaunchtype off`, followed by `wpeutil reboot`.
These are records of changes to the disposable lab, not instructions to alter
a physical test PC. No physical test PC was provided in the previous session.

## Resume on physical Windows

1. Confirm `gh api user --jq .login` is `btsouth`. Always use that GitHub account.
2. Read the current release and validation documents. Download the exact signed
   launcher from the v17 draft or signing run above and all draft assets with authenticated
   GitHub access. Verify the launcher and manifest digests above and Windows
   Authenticode before running. Do not rebuild and silently substitute a binary.
3. Follow RELEASING.md's local asset server procedure. Draft URLs return 404 to
   the anonymous launcher. Use a separate test directory and a copied, stopped
   guest; preserve the only copy of any real installation.
4. Record Windows build, CPU/GPU/driver, launcher/runtime hashes and feature
   state. Run GPU/fallback, input/clipboard/sharing, audio/device changes,
   window/fullscreen/resize/mixed-DPI, reboot/poweroff, idle CPU/microphone,
   sleep/resume, remote input and long-session checks. Include full Hyper-V on
   suitable hardware and the required graphics matrix.
5. Restore an export onto a fresh native Omarchy installation. Check real
   packages/themes and excluded VM-specific state. AUR acceptance remains open.
6. Resolve failures with focused changes, independent review, and exact-candidate
   retesting. Keep the draft unpublished until pre-publication hardware gates pass.
7. Check anonymous v17 assets and tagged/latest signed feeds during controlled
   publication. Separately prove pre-transfer updates and old-preview-to-stable
   migration through both feeds before v1. Preserve the signed legacy repository
   URL used by the update bridge; do not rewrite it as a cosmetic cleanup.

The user authorized necessary testing and logical progress without repeated
permission requests. Basecamp/to-do posting remains deferred. Linear must be
accessed through Toolport. Public text should be short, human, and contain no
em dashes. Never weaken production signature, TLS, or repository checks for tests.

## Retained lab evidence and installations

On the Linux host, the working repository is
`/home/bts/Projects/try-omarchy-release-finalization`; other worktrees were left
alone. Evidence and helper scripts are under
`/home/bts/Windows/try-omarchy-combined`, shared into Windows as
`\\host.lan\Data\try-omarchy-combined`. This handoff is also copied there.
The lab is Docker container `omarchy-windows`, SSH on loopback port 2222 via
`scripts/vmtest/winps.sh`. Do not print credentials from its environment.

Key evidence filenames:

- `preview17-asset-validation.json`, `preview17-signed-metadata-validation.json`, `preview17-signature.json`
- `preview17-settings-validation.json`, `preview17-linux-evidence/`, `signed-preview17/`
- `preview16-asset-validation.json`, `preview16-signed-metadata-validation.json`, `preview16-signature.json`
- `preview16-fresh-result.txt`, `preview16-upgrade-result.txt`, `preview16-reboot-result.txt`
- `real-restore-result.txt`, `preview16-authenticated-draft-check.json`
- `preview16-low-space-check.json`, `preview16-low-space-rollback-state.json`
- `preview16-transaction-interrupted.json`, `preview16-forced-rollback-state.json`, `preview16-forced-rollback-result.txt`
- `preview16-transaction-committed.json`, `preview16-transaction-success-result.txt`
- `hyperv-lab-recovery.json`, `hyperv-final-recovery.png`

Independent authentication evidence is in
`/tmp/try-omarchy-signed-feed-review.4aNk55`; public-route evidence is in
`/tmp/try-omarchy-public-feed-review.1IOOcY`. These are local temporary paths,
not portable GitHub artifacts.

Windows test installations, all retained:

- `D:\TryOmarchyRollback16`: successfully committed signed v16 after rollback/retry; clean guest shutdown before lab role changes.
- `D:\TryOmarchyPreview16Fresh`: signed v16 fresh/reboot tests, cowsay and Catppuccin installed for export.
- `D:\TryOmarchyCandidate`: upgraded v16 with original persistence documents.
- `D:\TryOmarchyPreview15Fresh`: earlier revision 17/disk-growth tests.
- Signed launchers: `D:\CandidateTools\TryOmarchy-preview15.exe`, `TryOmarchy-preview16.exe` and `TryOmarchy-preview17.exe`.
- `D:\TryOmarchyCompletionUI`: isolated Settings test folder, with no guest disk.

Dedicated D: test volume was expanded from 64 to 96 GiB without deleting files.
Its host image is `/data/windows-vm/try-omarchy-candidate-20260912.img`.
Do not confuse it with the Windows C: disk or resize it again without inspection.
Windows guest share is `C:\Users\bts\Omarchy Shared`, mounted at `/mnt/host`.

For PowerShell evidence, split `Get-Content -Raw` into plain strings before JSON
serialization. Serializing decorated Get-Content objects stalled the harness.
Use interactive scheduled tasks for GUI launches and verify result files after
QMP keystrokes. The prepared `check-full-hyperv.sh` did not run successfully;
its presence is not evidence of a full Hyper-V pass.
