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
