# Source-built runtime validation

The published runtime already uses our source-built `winq-emu-alpha10-source-r3`
archives. On September 12, both public v0.0.9-preview archives were downloaded
and verified against `guest-build/runtime.lock.json`; the bundled source lock
matches the current recipe. The Alpha 10 filenames are retained for compatibility.

The Runtime workflow produces replacement test artifacts. Keep the current pin
until a replacement passes these checks on supported Windows versions.

- `qemu-system-x86_64.exe --version` reports QEMU 11.0.0.
- `qemu-system-x86_64.exe -accel help` lists WHPX.
- Try Omarchy reaches the desktop using the rebuilt ZIP.
- Venus Vulkan starts on a supported GPU, including the existing GPU probe.
- CPU rendering still takes over when the GPU probe is forced to fail.
- Keyboard input, scoped Windows key handling, clipboard, audio, and sharing work.
- The host and guest cursors stay aligned during fast movement and fullscreen.
- Windowed, fullscreen, guest reboot, guest poweroff, and relaunch all work.
- The runtime archive extracts cleanly on a fresh machine without MSYS2 installed.
- Task Manager shows no unexpected console window or extra launcher process.
- After the desktop settles, QEMU's Task Manager CPU use falls materially below
  its active-animation level and does not pin one logical processor. Animation
  and video remain smooth when display activity resumes.
- On a host that refuses nested virtualization (Intel Core Ultra laptops, or
  any machine with the full Hyper-V feature set enabled), QEMU starts and
  `qemu-stderr.log` shows the "nested virtualization unavailable" warning
  instead of `Failed to enable nested virtualization` (issue #19).

Test at least one AMD, Intel, and NVIDIA graphics configuration before changing the public pin. Record the launcher version, runtime hash, Windows build, and driver version using [TESTING.md](TESTING.md).
