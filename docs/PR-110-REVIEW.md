# PR #110 correctness review

Reviewed September 12, 2026. There were no posted PR comments or reviews to
resolve. The source review covered checkpoint publication and recovery,
portable updates, QMP controls, saved disk/RAM state, transfer validation,
Windows display/device controls, LAN configuration and runtime patches.

## Findings fixed

| Finding | Fix and regression coverage |
| --- | --- |
| An active upload lost its status endpoint when its idle ticket expired. | Active transfers retain status access; a stalled HTTP upload test advances its expiry and reads its status. Idle tickets still expire. |
| The memory capture limit could be mistaken for stream EOF. | Capture reads beyond the limit to detect overflow and rejects truncated/oversized streams. Boundary and connection-failure tests cover both cases. |
| Valid USB hub paths could exceed the identity buffers. | Runtime r6 expands all four buffers, including persistent device state. The formatter test checks complete seven-level paths and all caller buffer sizes. |
| Retained portable recovery shortcuts lacked their startup manifests and could select newer payloads. | Recovery copies carry authenticated guest/runtime manifests and explicit original release arguments. Tests verify offline startup receipt matching, modified-manifest rejection and batch quoting. |
| Active rollback could update the restored payload before its first boot. | A marker moves with the restored VM transaction. Startup pins its guest/runtime and selects the archived runtime until guest readiness; failed boots retain the marker. Explicit release overrides remain available. |

The first r6 build caught the remaining short persistent-state USB buffer and
failed before compilation. The corrected recipe is in build 34724701163.

## Validation

- Linux race suite passed after the application fixes.
- Windows vet and AMD64/ARM64 builds passed.
- The interactive Windows lab passed 327 tests using runtime r5, including
  native clipboard, recovery, multiple windows, USB and saved-session tests.
- The transfer metadata fuzzer completed 1,017,113 inputs in 45 seconds.
- The guest contract passed all 97 tests in CI.
- Runtime r6 build, USB/RAM smokes and archive verification passed in
  [build 34724701163](https://github.com/omacom/try-omarchy-windows/actions/runs/34724701163).
- The final interactive Windows suite passed 330 tests with r6 and zero failures.
  Three Python interop tests run on Linux; the opt-in native firewall test
  passed separately.
- The signed v17 guest booted under runtime r6 with Windows TCG and passed
  the guest smoke: Omarchy 4.0.3, runtime package 4.0.3-3, compatibility 18.
  This exercises the signed baseline guest, not the newer compatibility-19 source.
- Generated recovery batch arguments also passed an actual Windows child-process
  round trip, including percent signs, spaces, ampersands and exclamation marks.

The final Windows test executable has SHA256
`ada4f3d90efbd43817d07fa3a993c40b810eab9ca9a32324fb80703c62e10387`.
The r6 runtime archive has SHA256
`725588202dbfd6510570775d10df1edcf379641df769cf72a3cfb5e29f3423fc`.
The source and signed-baseline distinction remains documented in
[FULL-FEATURE-COMPLETION.md](FULL-FEATURE-COMPLETION.md).

## File drops and save recovery follow-up

The drop adapter review fixed guest upgrade packaging and native-window tests.
The guest compatibility overlay now includes the transfer window and desktop
entry, with GTK and Python GI declared explicitly. Windows Shell tests compare
file identities because the Shell may expand short paths or normalize their
spelling. The test checks that the returned file is the original file.

Save recovery now reconnects after interrupted QMP commands. It also settles
its own disk-copy jobs before continuing the guest. Tests exercise both lost
stop replies and connection loss during a real QEMU disk copy. Cancellation
that races a successful QMP reply still invalidates the closed connection.

The compatibility-19 image build and headless boot passed. Its downloaded
checksums pass, and a local KVM graphical boot verified the transfer window as
a real Hyprland client. The current Linux race suite, Windows vet, ARM64 build
and PR checks pass. The final interactive Windows run passed 344 tests with
runtime r7, including the actual native OLE window-to-SDL drag. Six opt-in or
Linux-only tests are recorded separately. Test executable SHA256:
`1e4640f3e6dc30a45cf07563fb76d0849f682d5a2ad679857513530ce2d03346`.
Its log is `D:\TryOmarchyFullFeaturesTest\drop-session-full.txt`.

The complete Windows guest test exposed an outgoing-transfer storage bug:
archives were staged under `/run/user`, whose tmpfs is smaller than the safety
reserve on typical VM sizes. Patch 0063 moves archives to the disk-backed XDG
cache. Its regression covers clipboard and drop sends with a deliberately small
runtime filesystem, failed connection cleanup and preserved source files. The
clean guest contract now runs 102 tests, with one native GTK opt-in skipped.

The same integration work corrected two test assumptions: use an expanded
QCOW2 disk as the launcher does, and wait for the native GTK window to map
before asserting visibility. The compact factory image alone correctly rejects
large transfers when its remaining space falls below the reserve.


### Final integrated evidence

The rebuilt guest from [run 34731444397](https://github.com/omacom/try-omarchy-windows/actions/runs/34731444397)
passed all CI jobs. Downloaded artifacts and the decompressed rootfs stream
matched their hashes; Windows independently verified the staged image. Rootfs
SHA256: `dafc29fe70e74fd6621290c0172a4d0a3b16ad237cb747a14cfa83a31fdca82a`.
Runtime r7 archive SHA256:
`d2a3c972d6837730ecb99f3b470af534f0dba8ca7faf3b2dc36be8adb5fbc3df`.

The exact image passed the Windows TCG graphical boot and a real 31 MiB
Unicode-file round trip through the production bridge. Both native transfer
windows were visible, and byte checks, original-file checks and clipboard
preservation passed. The Windows fixture completed in 154.39 seconds.
Its executable SHA256 is
`818de0b9d8db5390bb66d48ab801ccca876d297b66c19be67b51341f2d7f59a7`.
The final interactive suite separately passed 344 tests with runtime r7. Four
Python interop tests pass on Linux, the native firewall test retains its earlier
separate pass, and the guest round trip is the separate opt-in just described.

The review also fixed late transfer-capability negotiation: guests can advertise
streaming after the legacy grace period without reconnecting. The regression
uses the same connection before and after that delay. Linux race tests, Windows
vet, AMD64/ARM64 builds and the release-helper tests pass. PR #110 had no posted
comments or reviews at the final check. It remains a draft; these artifacts are
unsigned feature candidates. Hardware and unfinished feature acceptance remain
tracked in the full-feature document.
