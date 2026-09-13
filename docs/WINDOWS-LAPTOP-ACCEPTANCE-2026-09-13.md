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
| CPU rendering | Pass explicit CPU path; forced probe failure pending | `cpu-render-guest.txt`; reboot and relaunch preserved file hashes |
| Native clipboard, transfer window, USB manager, display-window lifecycle | Pass for fixtures | `native-tests.jsonl`; does not prove real guest/device integration |
| Actual native OLE drag into SDL | Pass, three repetitions | `native-ole-drop-tests.jsonl` |
| LAN firewall lifecycle | Blocked by Windows permissions | `firewall-tests.jsonl`: New-NetFirewallRule returned Access is denied; not a pass |
| Real guest text clipboard | Pass both directions | `clipboard-guest-to-host.json`; Windows source and received UTF-8 files have identical hashes, including trailing LF |
| Real guest file clipboard | Pass both directions | `clipboard-folder-host-to-guest.json`, `clipboard-folder-guest-to-host.json`; 40 MiB random file, Unicode names, empty directory, matching hashes |
| Real guest image clipboard | Pass both directions | `image-clipboard-host-to-guest.json`, `image-clipboard-guest-to-host.txt`; distinct size/pixel fixtures |
| Shared folder | r7 fails; r8 fixes live retest | `r8-share-retest.txt`, `r8-share-crud.json`, host hash: Unicode read/write/rename/delete and current/explicit timestamps |
| Guest network | Pass localhost SSH and outgoing HTTPS | `shared/guest-facts.txt`; production QEMU user network |
| Physical USB | Pending hardware | `usb-host-inventory.json` exposes built-in camera and Bluetooth only |
| Audio routing | Pass host-session playback/capture lifecycle; subjective sound/device-switch tests open | `audio-tone.json`, `audio-capture.json`, `capture-byte-count.txt`, `audio-after-capture.json` |
| Backup and restore | Pass archive creation, cancellation, full disk hash verification; restored boot pending | `backup-manifest-check.json`, `restored-disk-verification.json`, `restore-cancellation.json` |
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
complete guest transfer fixture (subsequently passed), and firewall (attempted separately,
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

## Recovery and runtime-build evidence, 15:30 CDT

- PR commits fb34bb6 (Windows harness and initial evidence) and 6ad4965
  (runtime sharing fix) are pushed. All three required PR checks pass at
  6ad4965. Runtime build 34764396206 succeeded, including the compiled native
  Unicode/timestamp regression. `runtime-r8-build.log` retains the build output.
- r8 portable ZIP SHA256 is
  `7d4d4a65e175201c2e93a729c7edfa5553744340e1128832a05dffa42b7000ea`;
  source ZIP is
  `6bb3920cea129242bfbc1ab2c251b8d996d80ef2c631a331b62c588973590319`.
  Full archive/source verification passes. QEMU window executable SHA256 is
  `9c017432808a78dbac9eb7a97dca286865974de231d82ad5120f96fd7d7266a2`.
  Extracted path: `runtime-r8`. Live retesting is still required.
- Full guest transfer round trip passes on the physical laptop with exact r7
  against the separate disposable `fresh` disk: production streaming negotiated,
  both transfer windows verified, 31 MiB Unicode files matched, originals and
  both text clipboards retained. `guest-drop-roundtrip.txt` and
  `guest-drop-guest-check.txt` record the host and guest results. The standalone
  QEMU process was cleanly powered off after this test.
- Backup refuses a running launcher and creates no archive. A separate attempt
  during a deliberate source-disk hash read hit the exclusive file-lock guard;
  this was a harness overlap, not a VM that remained running.
- Backup cancellation removes the temporary archive. Low-space protection
  reports required and available GiB without publishing an archive. After space
  became available, full backup succeeded: `r7-persistent-backup.zip`,
  6,491,469,142 bytes, 204 manifest entries. Do not upload this private archive.
- The 24 GiB source disk, archived disk manifest and restored disk all have SHA256
  `84357b2abb206b6501ce6cc549b986db48a1b95e60c240d6ee36183795390c12`.
  Full restore into `Restored 世界` passed. Restore cancellation also left no
  destination or staging directory and preserved backup/source. The CLI restore
  tells users to launch with -dir; the separate recovery-UI shortcut path has
  not yet been tested.
- Fresh setup was cancelled near the end of unpacking. Downloads had already
  completed before confirmation, so interrupted-download acceptance is still
  open. Relaunch of `cancel-download` reached compatibility-19 userspace using
  r7. Its normal uninstaller removed the disposable folder and its own Apps &
  features entry; original installation, current r7 disk and shared files remain.
- Automatic approval review rejected starting a second throttled localhost
  server and separately rejected deleting the obsolete `fresh` disk/rootfs
  files. Both rejections gave only "blocked by policy". Neither action was
  retried through an alternate mechanism; the first test fixture remains.
  The distinct `cancel-download` copy was removed through its tested uninstaller.
- Six-minute GPU idle sample: mean 0.729% of total host CPU, maximum 3.785%,
  equivalent mean 8.75% of one logical processor. This includes guest display
  power saving and is not the awake-desktop idle gate.
- Direct QMP keypresses reach guest evdev, while the tested Windows automation
  injected key produced no guest event. Physical keyboard confirmation remains
  requested. No host sleep or host reboot has been performed.

## Additional live findings, 15:50 CDT

r8 fixes the original sharing failures on this physical AMD host. Native QEMU
process identity is recorded in `r8-live-process.json`. Both current-time and
explicit-time operations pass with appropriate guest permissions; Unicode
creation, enumeration, reading, rename, file deletion and directory removal
pass, including host-side checksum comparison. Text clipboard A/B/A repeats
also pass after the r8 boot.

A separate r7 failure appeared when booting the exact restored disk from
`Restored 世界`: QEMU could not open `vm/qemu.log`, then CPU fallback failed for
the same reason. The disk itself has the expected checksum. A packaged-binary
regression also reproduces Unicode path failure in r8 qemu-img. Commit 3ca83dc
adds process UTF-8 manifest settings and initializes the UCRT character locale
before file options are parsed. Numeric locale remains unchanged. This follows
Microsoft's [UCRT UTF-8 documentation](https://learn.microsoft.com/en-us/cpp/c-runtime-library/reference/setlocale-wsetlocale#utf-8-support)
and [per-process code page documentation](https://learn.microsoft.com/en-us/windows/apps/design/globalizing/use-utf8-code-page).
Recipe r9 is building in run 34766175917; it is not yet a tested fix. Public pins
remain unchanged. All three required PR checks pass at 3ca83dc.

Audio evidence uses NAudio Core/Wasapi 2.2.1 packages verified against NuGet's
catalog SHA512 hashes, installed only under the evidence tools directory.
QEMU's capture session was inactive at idle. An eight-second quiet generated
PCM tone reached the QEMU Windows render session (active, nonzero meter, then
inactive). A five-second guest capture opened and closed the Windows capture
session. A separate one-second byte-count probe delivered exactly 96,000 bytes
at 48 kHz, mono, s16. Audio was discarded, not retained. The count-limited
pw-record command returned 1 despite delivering the requested sample count;
this diagnostic exit is retained rather than silently reported as zero.
Both Windows audio sessions were inactive after capture. These facts do not
replace a human listening test, headphone switching, or visual microphone-icon
confirmation.

The guest is temporarily set to Stay Awake using its own toggle for the
five-minute awake-idle CPU sample. Restore Allow Idle after the measurement.
Host S3 and hibernate are supported. Reading host wake timers requires elevation;
no host sleep/reboot has occurred. User availability for physical keyboard,
UAC/firewall, and host sleep/wake checks was requested while testing continues.

September 13 afternoon continuation: malformed Settings repair was exercised on
`Restored 世界`. Declining preserved the exact malformed file. Accepting retained
it under `preferences-before-repair-3597579487/settings.json` with SHA256
`215579e970e8fb878cd2546bad369026d71bb6d856f125195c494112baf37c60`, then
restored defaults. The native Settings form showed automatic rendering, one
display, no shared folder and no forwards; its screenshot is retained.

The five-minute r8 awake-idle sample contains 60 readings, with mean total-host
CPU 0.611639%, maximum 2.15745%, and mean single-core-equivalent 7.33967%.
This is the awake-idle result, separate from the earlier mixed-DPMS sample.
TCP and UDP temporary QMP loopback forwards each returned the exact Unicode
payload and were removed successfully (`r8-network-loopback.json`). This does
not certify persisted launcher forwarding or LAN/firewall behavior.

Video acceptance found a new failure: default mpv Vulkan playback on Venus
(AMD Radeon Graphics) reports no suitable host-visible memory and repeated
VK_ERROR_OUT_OF_HOST_MEMORY, barely advances, and ignores SIGINT/SIGTERM.
The test process required SIGKILL. Explicit OpenGL playback reached the end
of the 15-second 1280x720/30fps clip with one dropped frame. A five-minute
OpenGL run is in progress. No mpv preference was changed to hide the default
failure. Logs are retained under the test share.

The r8 boot journal also contains one early disk write error before ext4
mounts read/write. QMP currently reports disk io-status ok. This remains an
unresolved diagnostic; the run must not be described as a clean stability pass.

Run 34766175917 built r9 but CI smoke failed because the staging directory
contained only the console QEMU binary, omitting qemu-img and the windowed
binary. The packaged archives passed verification locally. After correcting
that staging and replacing unreliable redirected-stdio QMP with the existing
bounded socket harness, all three r9 binaries pass real Unicode paths locally;
r8 fails the same test as the negative control. Commit 9c2565c contains the
harness corrections. Replacement CI run: 34767422097. The local r9 archives
are from 3ca83dc/run34766175917 and have not yet booted the restored installation.

R9 restored installation reached userspace in CPU mode at 16:10:54. The
bundled runtime update committed, installed executable SHA256 is
`0c0e4a02dc5da7838294852a1e9d9293aea6f9c22e51f7fe64064ad763f0b959`,
and both persistent fixture hashes match. Disk growth from 24 to 26 GiB
completed inside ext4. The restored boot has no disk I/O error in dmesg.
Both launcher-configured TCP and UDP loopback forwards passed. Hyprland reports
two enabled outputs; independent native-window visibility still needs proof.
Reclaim prepared 5448 MiB, completed after clean shutdown, and the native result
reported 5.4 GiB less allocated Windows disk space. Persistent files matched
before shutdown; verification after relaunch remains part of the next check.

The first two-display GPU attempt exposed a launcher bug: JSON `hostmem` was
sent as a string (`"4G"`), while QEMU requires an unsigned integer byte count.
The launcher now carries that setting as uint64 and its regression checks JSON
typing. It also retains a bounded stderr tail in shell.log before fallback,
so earlier startup failures survive retry. Native reproduction proved the
original argument failure and the retained diagnostic.

After correcting the argument, GPU startup progressed further and exposed a
runtime assertion in surface_gl_create_texture: a disabled secondary output
had destroyed its shader but retained scanout_mode. A delayed disable attempted
to recreate a texture with that null shader. Recipe r10 clears the scanout and
destroys its framebuffer while the context still exists, before destroying the
secondary window. The extracted real C helper regression aborts on r9 and passes
on the correction, including three disable/reactivate cycles. Physical runtime
acceptance of r10 is still pending. CPU fallback preserved the installation.
