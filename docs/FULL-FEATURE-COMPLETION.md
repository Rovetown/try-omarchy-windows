# Full feature completion

This is the active completion scope requested on September 12, 2026. All eight
features belong to the finished product. Earlier v1 exclusions are superseded
by this plan. The signed v17 candidate remains the regression baseline.

## Architecture and delivery order

| Feature | Implementation | Acceptance |
| --- | --- | --- |
| Snapshots | Versioned checkpoint store containing disk, guest boot files, runtime identity and settings. Atomic publication, checksum verification, retained recovery state and explicit deletion. Add incremental disk storage behind the same store contract. | Create, list, restore and delete; interrupted creation and restore; full disk; corrupt data; snapshots across launcher/image upgrades; user files preserved. |
| Saved sessions | Typed QMP control and runtime capability inspection, then a versioned RAM/device-state record bound to disk checkpoint and exact machine configuration. Resume validates all dependencies before changing the active installation. | Resume application memory and documents after launcher/host restart; interrupted save; changed devices; incompatible runtime; preserved original session on failure. |
| Portable mode | Use the same installation/checkpoint inventory for creation, backup, restore and updates. Materialize backing chains before moving between storage modes. Package matching launcher, runtime and guest with verified metadata. | Build from an existing installation; switch drive letters and PCs; update without resetting data; interrupted update; restore; removal during a write. |
| Drag-and-drop | One streaming transfer service shared with clipboard, with transfer IDs, destination acceptance, collision policy, quotas, cancellation and progress. Windows OLE and guest Wayland adapters provide the drag interaction. Runtime events identify the guest display and drop coordinates. | Both directions, folders, large files, Unicode, links, repeated drops, collisions, cancellation, reconnect and full disks. Originals survive every failure. |
| Multiple monitors | Device outputs, guest layout application and per-display Windows window management. Persist stable display identities and restore layouts after monitor removal or DPI changes. | Two and three displays, GPU and CPU paths, fullscreen per display, hot unplug, mixed DPI, keyboard focus and saved layouts. |
| USB and camera | Device broker with discovery, explicit attach/release, stable identities, disconnect events and driver checks. Use libusb for supported USB interfaces and a camera media bridge where replacing the Windows driver would be inappropriate. | Storage, serial, HID and camera cases; busy devices; unplug/replug; cancellation; host access restored after detach or crash. |
| Networking | Preserve localhost forwarding; add explicit LAN exposure with owned firewall rules and adapter selection, then bridged connectivity through a supported Windows network backend. | TCP/UDP on loopback and LAN; firewall lifecycle; collisions; adapter changes; DHCP, DNS and reconnection in bridged mode. |
| ARM64 | Architecture-selected release manifests, launcher/resources, native WHPX runtime and ARM guest build. Keep architecture in receipts, backups, checkpoints and compatibility checks. | Native ARM launcher and guest; updates and recovery; CPU/GPU paths; packaging and cross-architecture rejection without data changes. |

Work starts with the control protocol and state inventory, because migration,
block jobs, device hotplug and display control must acknowledge operations and
report failures correctly. Each completed feature includes UI, error recovery,
automated regression tests and a Windows acceptance procedure.

## Source audit

The runtime source is pinned to WINQ-EMU QEMU
`2ce303cfbbc8b0e4a7a3c66e27a094a980426d73`, based on QEMU 11.0.0.

- `app/qmp.go` has a lightweight connection for launch supervision and keyboard
  forwarding. Feature operations need correlated command IDs, bounded messages,
  event handling, cancellation, deadlines and explicit QMP errors.
- `app/backup.go`, `app/reset_portable.go`, `app/move.go` and `app/qemu.go` encode
  different assumptions about raw disks and QCOW2 backing files. Consolidate
  the inventory before extending portable updates or adding active disk chains.
- `hw/display/virtio-gpu-base.c` explicitly blocks migration when virgl is
  enabled. Full GPU saved sessions therefore require an implemented and tested
  graphics-state preservation path. Removing this blocker is not a solution.
  Runtime inspection must report this accurately while that path is developed.
- QEMU already supports multiple virtio-gpu outputs and SDL windows. The
  launcher currently requests the one-output default. Audit its single-window
  focus, close guard and placement handling together with guest layouts.
- The pinned source includes ARM WHPX support in `target/arm/whpx`; our build
  selects only `x86_64-softmmu` and hardcodes x86 artifact and machine names.
  [QEMU's WHPX documentation](https://www.qemu.org/docs/master/system/whpx.html)
  specifies the ARM Windows API baseline and the `virt` machine.
- `hw/usb/host-libusb.c` includes a Windows path. Verify the packaged DLLs,
  usable driver bindings and attach permissions, then implement the broker.
- `app/forward.go` hardcodes loopback. LAN selection must remain explicit in
  saved settings and firewall rules must have stable ownership and cleanup.

## Completion tracking

- [x] Inspect launcher, guest integration and pinned runtime dependencies.
- [x] Typed QMP command transport and real-runtime regression.
- [ ] Shared installation inventory and checkpoint storage.
- [ ] Snapshots UI and recovery acceptance.
- [ ] Saved-session implementation and graphics-state preservation.
- [ ] Portable creation, updates, backup and restore.
- [ ] Streaming transfer service and drag-and-drop adapters.
- [ ] Multiple guest displays and Windows window lifecycle.
- [ ] Device broker, USB attachment and camera bridge.
- [ ] LAN forwarding, owned firewall rules and bridged networking.
- [ ] ARM64 build, release and update path.
- [ ] Integrated Windows acceptance across every feature.

A feature is checked off only after its implementation and relevant automated
checks are complete. Hardware acceptance is recorded separately against the
exact integrated candidate, with failures fed back into implementation.

## Implemented foundation, September 12

The command client now correlates acknowledgments, bounds incoming messages,
handles cancellation and reports migration blockers. Shutdown uses this client.
A real local QEMU regression exercises stop, continue and status inspection.

The checkpoint store creates verified full archives with stable identities,
publishes completed entries atomically and restores into a separate directory.
The Settings recovery row opens a native snapshot manager with create, list,
restore-as-copy, delete and cancellation. Restore verifies the archive before
publishing its destination; the current installation remains intact. Damaged
entries are isolated, and deletion refuses unexpected files or directories.

Validation completed:

- Full Go race suite on Linux.
- Windows AMD64 build and native checkpoint, backup/restore and QMP tests.
- Windows vet passes with `-unsafeptr=false`. The default analyzer reports the
  same existing Win32 pointer warnings in the baseline checkout.
- Windows interactive snapshot manager: all controls present, create and list
  refresh, independently verified archive checksum, and close via IDCANCEL.
  This used a synthetic installation, so it validates UI and archive behavior.

Continue with the shared installation inventory and portable storage support.
Checkpoint work still includes incremental storage, interrupted-stage cleanup
and integrated recovery acceptance with real guest data. Saved sessions need
the graphics-state implementation identified in the source audit.

### Portable storage implementation

The installation inventory now identifies raw disks, factory-backed QCOW2
and standalone QCOW2. Portable backups and checkpoints materialize a verified
standalone raw disk, so the same restore path works across storage modes.
Conversion uses the bundled QEMU image tool with explicit backing format,
checks the backing image digest, supports cancellation and removes failed
staging. Both the source overlay and its factory image remain intact.

Settings now enables portable backup, restore and snapshots, and offers
Portable copy. Creation archives and restores the existing installation into
private staging, converts the disk to standalone QCOW2, compares its logical
contents, copies the launcher and verified manifest, and publishes the complete
folder with relative launch scripts. It requires matching installed payloads.

Tests include real guest writes in a QCOW2 overlay followed by backup and
restore, source preservation, corruption rejection and staging cleanup.
Portable creation and disk conversion also passed against the bundled Windows
runtime using synthetic guest fixtures. Windows AMD64 and ARM64 launcher
cross-builds succeed; native ARM guest/runtime integration remains active work.

The expanded Settings controls also passed the interactive Windows test at a
500-pixel work-area height: both new buttons remained visible, 60 Tab/Shift+Tab
steps kept focus visible, and mouse scrolling stayed in position.

### Portable update implementation

Authenticated automatic updates now stage complete payload versions beside the
portable installation. The previous payload directory remains available to its
launcher during rollback. Before changing a legacy factory image, the launcher
creates a checkpoint and converts the active disk to an independent QCOW2,
verifies its contents and publishes it atomically. A publication failure keeps
the old disk unchanged.

The update helper records portable layout in its rollback state and replaces
the executable at the bundle root. A native Windows test confirms replacement,
restart from the root and retention of the previous executable. Native disk
tests verify that persistent contents survive a changed factory image. Local
HTTP tests cover payload staging, version selection, corruption and cancellation.

### Multiple display implementation

Settings and `-displays` configure up to the runtime's 16 outputs. Each output
has a distinct EDID name and initial resolution. Multi-display launches use the
bundled runtime. The window manager identifies SDL outputs separately, restores
each placement, brands each window and applies close protection to all of them.
Display placements are included in backups and checkpoints.

A graphical KVM boot of the v17 guest reported three active Hyprland monitors
and passed the guest smoke checks. Native Windows tests cover the real QEMU
primary window plus a three-window SDL-class lifecycle fixture, including
independent identities, saved placements and removal of a closed window.
Physical GPU, mixed-DPI and monitor hotplug acceptance remains part of the
integrated Windows round.

### LAN forwarding and snapshot interruption recovery

Add LAN in Settings selects a Windows adapter, protocol and guest service port.
The saved adapter GUID follows DHCP address changes while preserving a selected
IP alias when it remains available. Missing adapters and resolved port conflicts
produce actionable errors. Existing forwards stay on loopback by default.

Firewall rules have a persistent installation identity, apply to the QEMU
executable and selected ports, and allow the local subnet on private/domain
networks. Public-network access is an explicit setting. Configuration changes
and uninstall remove only that installation's rules. Native Windows tests cover
adapter discovery, rule creation, verification, removal and preservation of
another installation's rules.

Snapshot recovery now removes recognized interrupted writes and deletions under
the store lock. It preserves completed snapshots, links and unexpected files.
The recovery tests pass on Linux and Windows, including refusal to clean a store
with another operation in progress.

The guest smoke runner now exercises echo traffic over TCP and UDP through both
loopback and a selected host LAN address. Both runs passed with the v17 guest
under KVM. This verifies the QEMU forwarding path; native Windows firewall tests
and the interactive Add LAN dialog also passed. The expanded Settings window
passed the 500-pixel work-area keyboard and scrolling checks again.

### USB attachment implementation

The running VM's tray now opens a native USB device manager with discovery,
refresh, explicit attach and release. The broker checks current bus/address,
physical port and vendor/product identity, creates the guest controller as
needed and waits for removal before reporting release. Reopening the manager
reads ownership from QEMU. Unplugged attachments remain releasable.

The runtime recipe now enables libusb, includes its DLL and adds an explicit
attachment mode that checks identity and opens the device before acknowledging
success. This mode does not automatically claim a replacement after unplugging.
A source fix bounds USB port-path formatting; a compiled fixture checks long,
empty and failed paths without touching hardware. CI checks the device models,
explicit-attachment property, failed-device cleanup and packaged dependencies.

Broker regressions and the native Windows manager refresh/close test pass.
Hardware USB, driver compatibility and webcam acceptance remain in the
integrated device work. The new runtime build is separate from the signed
release baseline.

Live display tracking also detects monitor topology changes and restores an
output that is no longer visible. The native three-window fixture passes
recovery after simulated monitor removal while leaving deliberate window
placement alone when the monitor layout has not changed.

The full native Windows launcher suite passes with the bundled QEMU image tool.
Interactive display and USB manager checks run separately on the desktop.
Forwarding now probes the complete TCP/UDP binding set before configuring the
firewall, releases every probe socket, and identifies QEMU bind errors without
triggering GPU fallback or runtime rollback. Occupied-port and socket-release
regressions pass on both operating systems.

### Portable disk expansion

Portable Settings now enables disk capacity and reports the current virtual
size. Growth works for both standalone and factory-backed QCOW2 disks. It copies
the active disk into private staging beside the original, expands and checks the
copy with QEMU, verifies its size and publishes it atomically. The original stays
locked through validation. Lowering the preference never shrinks an existing
disk, and interrupted staging is cleaned under the same disk lock.

Linux and native Windows tests verify preserved contents, backing identity,
zero-filled added capacity, no shrinking, cancellation, insufficient space,
locked disks, publication failure and interrupted-stage recovery. The native
portable Settings dialog also passed saving a 64 GiB preference.

The verified current v17 guest also passed the complete smoke facts under the
Windows QEMU executable using TCG, including the trial account, package set,
update integration and compatibility version 18. The serial harness now waits
for provisioning and a confirmed login session before sending checks. A byte
stream wrapper avoids PowerShell console translation and removes only its own
Windows QEMU process on timeout; timeout cleanup passed separately.

### Active snapshot rollback

Snapshots now offers Roll back alongside Restore as copy. Rollback verifies the
archive, checks architecture, prepares the complete replacement, and retains
the previous guest, disk, runtime and settings in a separate recovery folder.
Portable disks are converted and compared before publication, preserving the
portable layout. Recovery shortcuts open the retained state.

A durable journal records the replacement inventory before any active files
move. Launch recovers an interrupted replacement before reading guest settings
or starting updates. Uncommitted replacements recover the previous state;
committed replacements retain the restored state. Recovery preserves both sides
and refuses missing or conflicting files. The snapshot catalog, launcher, host
identity and unrelated files remain in place.

Tests cover every publication and recovery interruption point, repeated rollback,
retained documents, corrupt archives, busy disks, cancellation, pending updates,
invalid journals and architecture mismatch. Native Windows tests also pass with
real QCOW2 conversion and comparison.

The interactive Windows snapshot manager also passed create, list, active rollback,
retained work, recovery shortcuts and close. The UI harness finds dialogs by the
test process and uses the observed Windows OK control.

The real runtime smoke test caught a q35 limitation: its root PCIe bus cannot
hotplug a controller. The launcher now creates the USB controller at startup;
the manager attaches and releases devices on that existing bus.

### Private runtime controls

The launcher and PowerShell tools now connect to QMP through Windows filesystem
sockets in the user's local application directory. A guest-network test confirmed
that host loopback TCP is reachable from the guest, so binding privileged controls
to loopback did not provide the required separation. Startup preserves unknown
files, refuses live runtime sockets and removes only stale control sockets.

Linux race tests and native Windows tests pass for private connections, path
validation, live-owner protection, stale recovery and the USB manager. The native
PowerShell connection, status command and key-forwarder compilation also pass.
