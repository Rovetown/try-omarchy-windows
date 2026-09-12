# v0.0.17-preview completion candidate

PR #106 merged the implementation at
`8a99aa4d214c6b5edc8efa3008df8d5a7aef67af`. This candidate extends the signed
v16 baseline. It is not included
in the existing v16 draft executable or image. Guest compatibility revision 18
is required for file clipboard transfer and the safer reclaim agent.

## Implemented

- Two-way file and folder clipboard snapshots. Explorer CF_HDROP and Wayland
  file URI selections use a bounded ZIP frame. Cut selections are copied across
  systems; the bridge never deletes originals. Windows publishes the explicit
  copy drop effect described in [Microsoft's clipboard documentation](https://learn.microsoft.com/en-us/windows/win32/shell/clipboard).
- Maximum 16 MiB compressed, 64 MiB expanded and 1,024 archive entries, including
  directories. Links, device files, unsupported Windows names, path traversal,
  directory-case aliases and corrupt archives are rejected. Use the shared
  folder for larger selections. Clipboard transfer does not require sharing.
- Received snapshots stay in a separate cache, capped at 256 MiB and 16,384
  entries. The tray opens Windows received files for saving or removal. Guest
  copies are under `~/.cache/try-omarchy/clipboard-files`. No automatic expiry
  deletes files that a clipboard owner or application may still need.
- Reclaim and status are in the tray. Only raw disks are accepted. Duplicate
  requests are refused, CLI requests require acknowledgment, preparation failure
  and disconnection have explicit status, and shutdown reports the measured
  allocated-size reduction. Preparation keeps the existing budget and reserves.
- The guest uses a private unlinked temporary file for reclaim, preserving an
  unrelated file at the old fixed path and releasing temporary storage when its
  writing processes exit.
- Settings scrolls on smaller work areas and brings keyboard-focused controls
  into view. Longer explanations wrap. Settings and the tray expose built-in
  help for resources, storage, transfers, SSH, updates and diagnostics.

## Completed development checks

Linux launcher tests passed with the race detector. Archive round trips passed
from Go to Python and Python to Go, including Unicode and binary contents.
Guest tests cover rejected paths, links, collisions, corrupt data, echo
suppression, failed clipboard publication cleanup, and reclaim preparation.
The patched guest contract passed. Windows cross-compilation and vet passed.

An opt-in native Win32 test passed on the recovered Windows lab: CF_HDROP
publication, explicit copy effect, file-content round trip, source preservation,
and return to text clipboard. It did not use an accelerated guest. Independent
review found and verified fixes for cache accounting, source-directory races,
cleanup failures and misleading reclaim metrics.

## Windows acceptance round

Use the newly built launcher and revision 18 guest together in a copied test
installation. Do not substitute the signed v16 artifact. Keep the public release
and primary installation unchanged until this candidate is accepted.

1. Copy a file, Unicode name, binary file, folder tree and empty folder in both
   directions using Explorer and Omarchy Files. Paste twice, verify hashes and
   preserve originals after Cut. Check both supported guest clipboard MIME types.
2. Copy text and images before and after files. Reconnect the bridge and restart
   the guest. Confirm no echo loop, stale clipboard overwrite or lost selection.
3. Test the limits, unsupported names/links, failed clipboard publication and
   full cache. Confirm useful errors, unchanged originals, no partial snapshot,
   and that clearing old received files restores transfer availability.
4. Run reclaim on a copied raw disk after deleting known data. Confirm pending,
   duplicate, low-space, disconnect and failure responses; wait for preparation,
   shut down, and compare allocated bytes with the displayed result. Verify
   important file hashes. Confirm portable QCOW2 refuses preparation.
5. Exercise Settings at small effective resolutions and scaling levels, using
   Tab, Shift+Tab, scroll wheel and scrollbar. Reach every control and Save/Cancel.
6. Repeat the existing hardware, update, backup/move/recovery and native export
   gates from V1-READINESS.md against the final signed candidate.

## Remaining scope

File drag-and-drop, multi-monitor guest support, snapshots, ARM64 and device
passthrough are not implemented by this change. Native file clipboard and shared
folders provide file transfer. Drag-and-drop needs a separate runtime change and
interaction design; it must not be advertised as implemented.

Physical native restoration, the package-lock incident's origin, public signed
update delivery and stable distribution acceptance remain open. Do not close
those issues from archive/unit tests. Ecosystem publication and winget submission
must use the final accepted version, signed hashes and official download URLs.
