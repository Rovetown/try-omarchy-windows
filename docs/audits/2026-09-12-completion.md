# Release completion, September 12

Baseline: v0.0.14-preview on master, Omarchy 4.0.2 factory image, WINQ-EMU
Alpha 10. Installation moves remain in draft PR #86. This audit branch does
not change the public payload pin or publish a release.

## Current implementation

- Pin the factory desktop to Omarchy 4.0.3, including the normalized source
  digest and the refreshed Arch package transaction. Retain the notification
  fixes, correcting misplaced QML blocks and rebasing the patch with context.
  The builder now parses every backported QML file before packaging.
- Install and register the missing scoped browser, DNS and time-zone sudoers
  rules, sudo attempt settings, temporary-sudo cleanup, Kitty local-socket
  restriction, mise defaults, plocate AC restriction, multicast DNS setting,
  file-watcher and file-descriptor limits. Package configuration uses pacman's
  backup semantics so later package updates preserve administrator changes.
- Run upstream Chromium policy setup during image creation. A compatibility
  service repairs missing browser policy setup on older guests before SDDM,
  preserving an existing administrator-supplied sudoers rule.
- Include pamixer and playerctl for media controls.
- Refuse to pack a guest with a package database lock. Never automatically
  delete a running guest's package lock. Issue #90 still needs reproduction
  to identify where the reported orphaned lock originated.
- Make package-refresh automation create drafts and retain a reviewable branch
  with a run-summary link if repository policy rejects automatic PR creation.
  Explicit CI dispatch covers the workflow-token event restriction.
  Package refresh does not update the Omarchy source pin itself.

## Upstream configuration review

The builder previously copied only selected etc files. Applying the entire
bare-metal etc tree is inappropriate for this guest. The additional files above
address desktop actions and system defaults. Remaining differences need explicit
acceptance, not a claim of complete bare-metal parity:

- Bootloader, UKI, mkinitcpio, Plymouth, hardware power and USB settings: retain
  the VM boot and device configuration.
- Docker and printing services: optional packages are not currently shipped.
- SDDM and login configuration: retain first-run provisioning and VM autologin;
  verify personalized provisioning and lock/unlock with 4.0.3.
- Authentication: inspect the installed PAM profile and upstream lock setup,
  including lock-screen authentication under the candidate shell.
- SSH command environment and keepalives: evaluate alongside session-only SSH.
- Existing user shell and application configs: preserve user changes. Their
  migration must be tested separately from the new factory skeleton.

## Gates to stable

1. Fresh factory boot and personalized setup, theme changes without prompts,
   valid browser color policy, normal package update and interrupted-update
   recovery. Check both GPU and CPU paths.
2. Existing writable guests: demonstrate an in-place Omarchy version upgrade.
   The local try-omarchy-runtime package has no remote upgrade feed. Updating
   launcher integration alone does not upgrade that package. Do not promise
   that Update > Omarchy currently advances its source pin, or require a reset
   as the long-term upgrade solution.
3. Complete independent review and Windows acceptance for PR #86: moves,
   retained originals, cancellation, disconnects, low space and recovery.
4. Validate and pin source-built WINQ-EMU with matching source. AMD, Intel,
   NVIDIA, CPU fallback, full Hyper-V and nested-virtualization refusal.
5. Test exact-candidate signed feeds, skipped-preview bridge, pre-transfer
   installation, forced rollback, disk growth and persistent data.
6. Validate backup, restore, reset, reclaim and uninstall. Expose reclaim in
   normal UI only after its physical checks pass.
7. Sleep/resume, mixed DPI, audio changes, microphone activity, remote input,
   long sessions and measured idle CPU. Restore configuration to a fresh
   physical Omarchy installation.
8. Reconcile GitHub #77, Linear SBS-1101 and user documentation; prepare stable
   distribution and verify public downloads only after release authorization.

File drag-and-drop, multiple monitors, ARM64 and device passthrough remain
separate feature commitments. They are not completed by this image refresh.
