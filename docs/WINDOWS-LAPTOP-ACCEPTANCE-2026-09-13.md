# Windows laptop acceptance, September 13, 2026

Status: in progress. This is physical Windows acceptance of PR #110, not a
release approval. The user selected the current launcher with runtime r7 and
the corrected compatibility-19 guest-r2. Existing native Omarchy testing is
outside this run's scope.

## Candidate and evidence

- Repository: `omacom/try-omarchy-windows`, branch `codex/full-feature-completion`.
- Starting commit: `a566995c6c60b7b7e98289eac44023632c28c43e`.
- Local evidence/artifacts: `C:\cssi\try-omarchy-acceptance\2026-09-13`.
- Runtime build: `34729022320`, artifact `winq-emu-alpha10-source-build`.
- Runtime ZIP SHA256: `d2a3c972d6837730ecb99f3b470af534f0dba8ca7faf3b2dc36be8adb5fbc3df`.
- Guest build: `34731444397`, artifact `guest-candidate`.
- Expected raw rootfs SHA256: `dafc29fe70e74fd6621290c0172a4d0a3b16ad237cb747a14cfa83a31fdca82a`.
- Go 1.27.1 Windows AMD64 archive verified against go.dev SHA256:
  `a3911b5e0e1b1053f25ed0675f4c1c6aad1e2bfcf253df2b9be4caabd2edd95d`.

The original `%LOCALAPPDATA%\TryOmarchy` installation is retained. No VM was
running at discovery. The two old executables in the repository were retained.
Unsigned source candidates remain separate from signed release evidence.

## Host

Windows 11 Business, build 26200; AMD Ryzen 5 5625U, 12 logical processors;
AMD Radeon Graphics driver 31.0.21921.1000. Three MS Idd virtual display devices
are also reported. Initial C: free space was approximately 62 GiB on NTFS.
Windows Hypervisor Platform and Virtual Machine Platform are enabled; the full
Hyper-V role is disabled. Detailed facts are in `host-inventory.json`.

## Results

| Check | Result | Evidence |
| --- | --- | --- |
| Runtime archive checksum and source/binary recipe verification | Pass | `runtime-build/verify.py` reports 10 required files and corresponding source verified |
| Runtime version and accelerator inventory | Pass | QEMU 11.0.0; TCG and WHPX listed; not yet a WHPX boot result |
| Compressed guest and metadata checksums | Pass | `guest-verification.json` |
| Decompressed rootfs stream checksum and length | Pass | `rootfs-stream-verification.txt`: 7,516,192,768 bytes, exact expected SHA256; no extra raw image retained |
| Current launcher build and Windows vet | Pass | Launcher SHA256 `512f728b67190e8f3d441102264780b675f7adbd406e8018e13ab4cae9fce319`; `launcher-hash.json`; vet rerun after fixes |
| Windows automated runtime suite with native UI and clipboard opt-ins | Pass, 342 top-level tests; 9 skipped | `windows-full-retest.jsonl`, `windows-full-retest.stderr.txt` |
| Release helper tests | Pass, 15 tests | `release-helper-final.txt` |
| Fresh combined-candidate WHPX desktop and GPU | Pass | `fresh-r7/vm/shell.log`, `fresh-r7/render-probe.json`; visible desktop and runtime SHA256 match |
| CPU rendering/fallback | Pending | |
| Native clipboard, transfer window, USB manager, display-window lifecycle | Pass for fixtures | `native-tests.jsonl`; does not prove real guest/device integration |
| Actual native OLE drag into SDL | Pass, three repetitions | `native-ole-drop-tests.jsonl` |
| LAN firewall lifecycle | Blocked by Windows permissions | `firewall-tests.jsonl`: New-NetFirewallRule returned Access is denied; not a pass |
| Real guest text clipboard | Pass both directions | `clipboard-guest-to-host.json`; Windows source and received UTF-8 files have identical hashes, including trailing LF |
| Real guest file clipboard | Pass both directions | `clipboard-folder-host-to-guest.json`, `clipboard-folder-guest-to-host.json`; 40 MiB random file, Unicode names, empty directory, matching hashes |
| Real guest image clipboard | Pass both directions | `image-clipboard-host-to-guest.json`, `image-clipboard-guest-to-host.txt`; distinct size/pixel fixtures |
| Shared folder | Partial pass; Unicode failure | 1 MiB read/write fixture passes; Unicode directory makes 9p listing fail with EIO; ASCII rename restores listing |
| Guest network | Pass localhost SSH and outgoing HTTPS | `shared/guest-facts.txt`; production QEMU user network |
| Physical USB | Pending hardware | `usb-host-inventory.json` exposes built-in camera and Bluetooth only |
| Audio, display lifecycle | Pending | Guest PipeWire recognizes virtio input/output; playback not yet verified |
| Snapshot, backup, portable, move and recovery acceptance | Pending | |
| Long session, idle CPU, sleep/resume | Pending | |

An initial test invocation overlapped Go archive extraction and failed to find
standard-library files. It is retained in `toolchain-extraction-race.jsonl` as a
harness preparation failure; the suite was restarted after extraction completed.

## Failures investigated and fixed

The first complete Windows run passed 332 tests, skipped 16 and failed three
saved-session RAM tests. The failures reported `Invalid argument` connecting to
the real `%LOCALAPPDATA%\TryOmarchyIPC` directory. A diagnostic reproduced the
failure with both QEMU and a direct Windows socket client. Moving the fixture
IPC to its own temporary directory succeeded. The existing IPC directory's ACL
includes restricted Codex identities. No ACL or production socket transport was
changed. `startSavedSessionTestQEMU` now owns an isolated, short IPC directory;
the saved-session retest and full suite pass, including verified RAM restoration
in a new QEMU process. This is TCG fixture evidence, not accelerated desktop
saved-session acceptance. Diagnostic observations are in `socket-diagnostic.txt`;
its logging-only PASS labels are not acceptance assertions.

Release-helper fixes use the host PATH separator for test command mocks, remove
native Python's CR from Git Bash runtime-lock rows, and write the checksum test
fixture with LF. Tests require Git's `usr\bin\bash.exe`, not its `bin\bash.exe`
wrapper, which prepends real commands ahead of the mocks. The evidence directory
contains a `tools\bin\python3` shim selecting bundled native Python. All 15
release-helper tests pass with that environment.

The nine full-suite skips are four Linux guest interop tests, two symlink tests
without the required Windows privilege, native OLE (passed separately), the
complete guest transfer fixture (pending), and firewall (attempted separately,
permission denied). None are included in the 342 passing count.

The PR's three required checks pass at the starting commit. It remains draft,
with no posted reviews or comments when checked (`pr-state.json`). Local changes
have not been pushed, and no release has been published.

## September 13 live desktop continuation

The user explicitly approved all necessary work after the first launch block.
The initial boot selected existing `C:\WINQ-EMU` by documented runtime precedence.
It reached readiness and powered off cleanly, but is not r7 acceptance evidence.
The corrected launch uses `-winq` pointing at a nonexistent test-only path so
the launcher downloads and verifies the explicitly selected r7 runtime.
The separate `fresh-r7` installation booted with WHPX and GPU at 14:24:39 CDT.
It uses 6 GiB RAM, eight vCPUs, and a 24 GiB root disk. Guest compatibility is 19.

The PowerShell QMP helper exposed two real compatibility failures under modern
.NET: variable-sized sockaddr_un buffers caused an invalid-pointer error, and
disposing the completed async result's shared wait handle broke successive
connections. The helper now supplies the full 110-byte buffer and polls bounded
async completion without touching the shared handle. Its isolated regression
passes 20 consecutive connections (`qmp-pwsh-regression.txt`). Windows PowerShell
5.1 refused the test script under execution policy; that runtime remains untested.

An ephemeral SSH key and localhost port 2244 support test commands in the isolated
guest. The private key is local test material, not evidence to upload. Persistent
fixtures live in `/home/omarchy/Acceptance`; expected file hashes are recorded in
`shared/guest-facts.txt`. Port 2244 was added through QMP for this boot and must be
re-added or configured through `-ssh` on relaunch. The shared directory is
`C:\cssi\try-omarchy-acceptance\2026-09-13\shared`.

The confirmed 9p Unicode failure is still open. Creating `Clipboard 世界` on
Windows made `ls /mnt/host` fail with Input/output error. Renaming it to
`Clipboard-ascii` restored directory enumeration. The clipboard copy containing
the same Unicode names passes in both directions. Runtime source inspection
shows ANSI CreateFile/_findfirst/_findnext use in `9p-util-win32.c`; this is a
candidate cause, not yet a tested fix. Guest-written files also showed epoch
timestamps, requiring a focused timestamp check. Original files are retained.

## Initial approval-block record

Automatic approval review rejected the candidate launch command with only
`blocked by policy`. User confirmation was requested for the exact executable,
isolated fresh directory, localhost assets, disabled updates and 24 GiB disk.
The user subsequently approved this action explicitly and the isolated launches described above succeeded. The prepared script also isolates process-local
LOCALAPPDATA beneath the evidence directory, avoiding the real installation's
IPC and host integration. The original installation remains untouched.

The localhost artifact server is running on 127.0.0.1:18080 from the evidence
directory (Python `http.server`). For continued exact-r7 testing, run `launch-candidate.ps1`,
inspect setup and desktop with the computer-use skill, then exercise the
remaining real guest Windows matrix. Keep fixture passes distinct from those
end-to-end results. Do not reboot the host without recording a resume point.

## Remaining coverage constraints

The user permits Windows testing and recreation of test artifacts. Host reboot
would terminate this session and requires a durable continuation record first.
AMD-only results cannot satisfy Intel/NVIDIA or native ARM64 hardware gates.
Full Hyper-V coexistence is not covered by the current feature configuration.
Unimplemented accelerated saved sessions, direct application drop placement,
camera bridge, bridged networking and native ARM64 product support remain the
separate engineering gaps identified in SESSION-RESUME.md. Do not infer their
completion from automated test counts or mediated transfer-window tests.

## Lifecycle and source-fix continuation, 15:00 CDT

- Exact r7 GPU session ran from 14:24:16 to clean guest poweroff at 14:52:52.
  This is approximately 28 minutes, not the one-hour gate.
- `gpu-idle-cpu.json` records 72 five-second samples. The sample includes guest
  display power saving, so it must not be described as five minutes of awake
  desktop idle. `r7-idle-display-off.jpg` and `r7-gpu-desktop.jpg` distinguish
  the powered-off display from the recovered desktop.
- Hyprland reported dpmsStatus=false. The current Lua dispatcher
  `hl.dsp.dpms({action = "on"})` restored its lock screen, and the documented
  trial password unlocked it using QMP input. Shift-only wake was inconclusive;
  normal native wake/input acceptance is still outstanding.
- A separately copied runtime with an experimental UTF-8 manifest booted with
  the same disk. This is not r7 release evidence. It did not fix Unicode 9p
  enumeration (`utf8-share-experiment.txt`) and was cleanly stopped.
- Exact r7 CPU rendering booted at 14:54:41, reached userspace ready at 14:54:53,
  and handled a guest reboot at 14:58:53 by starting a new QEMU process. The
  relaunched guest was ready at 14:59:11. Persistent text and binary hashes
  match before and after (`cpu-post-reboot-persistence.txt`). The product's
  `-ssh 2244 -ssh-key` setup works on both starts.
- `r7-share-failures.json` confirms nested Unicode enumeration EIO and current
  timestamps becoming epoch. Explicit timestamp assignment as ordinary user
  was denied by guest permissions; the root experiment sets the explicit date
  correctly, but current-time handling remains broken. Keep these distinct.
- Draft recipe r8 adds patch 0008 and a native Windows regression for Unicode
  file APIs and timestamp NOW/OMIT/explicit/invalid/NULL semantics. It is not
  yet compiled or live-tested. The public runtime lock remains unchanged.
  The manifest-only experiment must never be published as r7 or r8.
