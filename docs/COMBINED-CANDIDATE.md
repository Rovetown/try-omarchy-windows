# Combined release candidate

This branch combines #86 (installation moves), #91 (Omarchy 4.0.3), and #92
(persistent guest upgrades). The published release and its default payload pins
remain unchanged.

## Payload identity

The guest extends the image validated by #92 with lock authentication repair
and the current signed package transaction. It uses runtime `4.0.3-3`,
compatibility revision 16, Chromium `153.0.8010.36-1`, Hyprland `0.56.2-3` and
gpu-screen-recorder `6.1.2-1`. The combined payload retains published WINQ-EMU
Alpha 10; the source-built runtime is a separate hardware-validation gate.
The earlier image passed a full 4.0.2 to 4.0.3 upgrade and five boot checks.

The combined payload checksum-list SHA256 is:

```text
bec7e21f74364e1137203861c6c91f6f4ac271dfa08fc7511598a517a86a44fa
```

It includes the guest's checked artifacts and the runtime/source archives from
`guest-build/runtime.lock.json`. This is a candidate identity, not a published
release URL.

## Signed test launcher

The existing **Sign runtime test launcher** workflow now also accepts
`payload_kind=candidate`. Supply the checksum-list digest above and leave
`release_url` empty. It builds the selected source revision, runs native Windows
tests, embeds the candidate checksum pin, signs the executable, verifies its
Authenticode signature, and records the source/payload/launcher identities in
`test-launcher.json` beside the artifact.

For offline testing, place the executable beside a `payload` folder containing
the authenticated artifacts and run:

```powershell
.\TryOmarchy-runtime-test.exe -portable
```

For standard-installation and move tests, use a local payload server and an
explicit test data directory:

```powershell
.\TryOmarchy-runtime-test.exe -dir 'D:\Omarchy Candidate' -no-update `
  -release http://TEST-HOST:18081/payload `
  -sums-sha256 bec7e21f74364e1137203861c6c91f6f4ac271dfa08fc7511598a517a86a44fa
```

The release environment currently permits only `master`. No branch policy was
changed as part of this work. The authorized candidate-branch exception was
rejected because `btsouth` lacks repository-admin rights. Signing run
[34682745697](https://github.com/omacom/try-omarchy-windows/actions/runs/34682745697)
was blocked before signing by that branch policy. An administrator must allow
the exact candidate branch, or signing must follow review and merge to `master`.
The workflow uploads a test artifact and does not create or publish a release.

## Validation status

- Combined Linux race tests, vet, and Windows cross-build passed.
- All 228 native launcher tests passed on Windows 11 Enterprise build 26200.
  The idle-download test now holds the response open until cancellation instead
  of depending on a short server sleep, which was unreliable on the busy VM.
- Guest patch reconstruction, 80 guest tests, and 13 release-helper tests passed.
- All four PowerShell workflow blocks passed Windows PowerShell syntax checks.
- The unsigned combined launcher booted Omarchy 4.0.3 to its desktop in the
  Windows VM. Terminal and shared-folder access worked, with no failed user
  services. QEMU retried without audio, so this does not validate audio.
- A full-size Windows move exposed a file-path error in the periodic free-space
  check. The check now uses the destination directory. A 33 MiB regression test
  failed before the fix and passed afterward.
- Settings rejected a move when NTFS reported only 8.1 GiB available for a
  13.3 GiB copy. The source remained intact and no move journal was committed.
  The dedicated test volume was expanded before retrying.
- With the fix, Settings moved the full installation from `D:\Omarchy Candidate`
  to `D:\TryOmarchyCandidate`. Copy verification and activation completed, the
  source was retained, and the relocated guest reported ready on its next boot.
  A saved document kept its exact SHA-256, Omarchy reported 4.0.3, and there
  were no failed user services. Launching the original path used the moved
  installation and retained a document created after the move. Cleanup refused
  a changed original settings file, then succeeded after it was restored. The
  moved guest booted afterward with both documents unchanged and an unrelated
  file outside the installation preserved. These storage checks used the prior
  payload checksum `0d379bcb4a38be357fa07bdc58af676a6248e7d17961eb86212b9f4f32fa1830`.
- Personalized Linux VM testing exposed a missing PAM profile: the lock action
  returned `missing-pam` and left the desktop unlocked. Patch 0047 runs the
  pinned upstream lock setup in fresh images, packages its configuration with
  backup semantics, and repairs only missing profiles on existing guests.
  Wrong-password rejection and successful unlock were observed. Booting the
  new initramfs repaired the same existing personalized disk; a custom policy
  survived both repair and the runtime upgrade to `4.0.3-3`.
- The final image passed its boot smoke test, including PAM ownership, exact
  profile contents, runtime version, browser policy, media tools and update
  repository. The personalized guest also completed the normal Omarchy update
  to the refreshed package versions with no Hyprland configuration errors.
  Its orphan-removal and reboot prompts were answered during the test.
- A Windows payload update was deliberately interrupted immediately after
  activation, before guest readiness. The next launch restored the prior
  receipt and payload, booted successfully, cleared the pending state, and
  preserved both saved documents. This sequence used the revision 15 payload,
  before the notification retry fix. It tests guest payload recovery using an
  unsigned launcher; it does not validate signed launcher-update rollback.
- Windows also booted and committed the updated payload after recovery,
  preserving both documents. Its repaired lock screen rejected an incorrect
  password and unlocked with the correct one. A startup race in the update
  notice surfaced during this test: notifications were not yet available.
  Patch 0049 bounds retries and records delivery only on success. With the
  shell deliberately stopped and restarted later, delivery succeeded and
  no user services remained failed.
- The Windows VM has Hypervisor Platform enabled, but the full Hyper-V feature
  is disabled. This is nested-VM evidence, not physical-hardware acceptance.

Physical AMD/Intel/NVIDIA, full Hyper-V, sleep/resume, audio, DPI, and signed
update/rollback acceptance remain required before declaring the release ready.
