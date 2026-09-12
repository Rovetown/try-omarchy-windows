# v0.0.15-preview candidate validation

Validated September 12, 2026. The release remains an unpublished draft.

## Exact candidate

- Source: `ef831a6395b37feb5a40dc75cb12ce18bb911c9b`.
- [Guest preparation](https://github.com/omacom/try-omarchy-windows/actions/runs/34702836511): build, first boot, asset sizes and uploads passed.
- [Signed launcher and update metadata](https://github.com/omacom/try-omarchy-windows/actions/runs/34703526225): passed.
- SHA256SUMS: `f07eaaf01969ab5db75ab50f9f6a3a311108973bee94b83e05966e96ec9effd5`.
- Signed executable: `23c8e408086c33b2d9696458ee8e81bd122cc111aa381a93ce3283b941e48d66`.
- Omarchy 4.0.3, local runtime package 4.0.3-3, compatibility revision 16.
- All downloaded draft assets match their checksums. Authenticode is valid;
  the executable version is v0.0.15-preview. The update metadata signature was
  verified against the launcher's embedded Ed25519 public key.

## Windows VM results

Tests used Windows 11 Enterprise build 26200, CPU rendering, a dedicated NTFS
test disk, and separate existing and fresh installations. Audio required the
launcher's fallback in this nested VM.

- Interrupted activation before guest commit restored the previous payload.
  It booted successfully and both existing document hashes were unchanged.
- A subsequent uninterrupted upgrade committed the exact draft payload.
  Both documents remained unchanged and no user services failed.
- Fresh installation unpacked the signed candidate's pinned payload, completed
  setup and reached the desktop.
- Chromium started headlessly with its sandbox enabled and rendered a blank page.
- The package database was unlocked. Installed package versions matched the
  draft, including libgcrypt 1.12.4-1 and libportal 0.11.0-1.
- Guest reboot returned to readiness, the new document retained its exact
  hash, and guest poweroff exited the launcher cleanly.

## Release-size fix

The previous compression produced a 2.41 GB asset, above GitHub's 2 GiB limit.
A 256 MiB zstd window reduced the same test filesystem to 2.00 GB. The final
CI archive is 2,002,385,597 bytes. The workflow now checks sizes before creating
any draft.

The decoder uses normal-memory mode to avoid repeatedly copying the larger
history window. On the same test image, Linux decoding took 11.9 seconds
versus 12.3 seconds with the previous compression. Windows decoding took
21.4 seconds with 523 MiB peak working set. Both produced the identical raw
filesystem hash. These are decoder measurements, not complete install timings.

## Remaining acceptance

This does not establish physical GPU, audio, full Hyper-V, remote-input,
sleep/resume or mixed-DPI acceptance. Complete the physical checks in
[RUNTIME-VALIDATION.md](RUNTIME-VALIDATION.md) before publication.

Signed payload rollback was tested using the signed launcher and local copies
of authenticated draft assets. The anonymous pre-transfer launcher download
path and preview-to-stable bridge still need separate end-to-end acceptance.
Configuration export onto a fresh full Omarchy installation also remains open.
