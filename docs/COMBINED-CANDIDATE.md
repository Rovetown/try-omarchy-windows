# Combined release candidate

This branch combines #86 (installation moves), #91 (Omarchy 4.0.3), and #92
(persistent guest upgrades). The published release and its default payload pins
remain unchanged.

## Payload identity

The guest is the image validated by #92, including the full 4.0.2 to 4.0.3
upgrade and five boot checks. It uses runtime `4.0.3-2`, compatibility revision 14,
and the currently published WINQ-EMU runtime. The source-built runtime is still
a separate hardware-validation gate.

The combined payload checksum-list SHA256 is:

```text
0d379bcb4a38be357fa07bdc58af676a6248e7d17961eb86212b9f4f32fa1830
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
  -sums-sha256 0d379bcb4a38be357fa07bdc58af676a6248e7d17961eb86212b9f4f32fa1830
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
- Guest patch reconstruction, 70 guest tests, and 13 release-helper tests passed.
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
- The Windows VM has Hypervisor Platform enabled, but the full Hyper-V feature
  is disabled. This is nested-VM evidence, not physical-hardware acceptance.

Physical AMD/Intel/NVIDIA, full Hyper-V, sleep/resume, audio, DPI, and signed
update/rollback acceptance remain required before declaring the release ready.
