# Source-built runtime validation

September 13 physical laptop results are recorded in
[WINDOWS-LAPTOP-ACCEPTANCE-2026-09-13.md](WINDOWS-LAPTOP-ACCEPTANCE-2026-09-13.md).
That round found r7 shared-folder Unicode/timestamp failures and Unicode
installation-path failures. The r8 sharing correction passes on the laptop;
r9 path correction passes restored Unicode-path boot. Multi-display failures
produced r10/r11/r12 corrections; r12 three-output power cycling and secondary
rendering pass initial physical checks, with endurance and input acceptance ongoing. Default mpv Vulkan
playback also fails on this AMD configuration, although explicit OpenGL plays.
These findings keep the public pin unchanged.

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
