# Runtime command ownership repair, September 14, 2026

The candidate runtime `4.0.3-4` fixes the duplicate Neovim helper ownership in
[#116](https://github.com/omacom/try-omarchy-windows/issues/116). The runtime
package now takes commands from the materialized Omarchy command directory,
rather than collecting every `/usr/bin/omarchy-*` file. Registration and fresh
guest smoke checks reject an inconsistent package database.

## Existing-guest migration

Dropping duplicate ownership alone is insufficient: a real v18 upgrade test
showed pacman removing both helper paths when replacing the old runtime package.
The package now includes upgrade callbacks that retain the installed helpers
before removal and restore missing paths afterwards. This preserves the regular
setup script, the refresh symlink and administrator edits. A helper already
installed by another package in the transaction is left in place.

Recovery copies use a root-owned mode-0700 directory at
`/var/lib/try-omarchy/runtime-ownership-recovery`. Copies are staged before being
renamed into place. A successful restore removes its retained copy; an interrupted
transaction can retain recovery files. Reinstalling the corrected runtime runs
the restore callback again. This mechanism touches only the two previously shared
helper paths and never removes a pacman lock.

## Validation

A disposable copy of the checksum-verified v0.0.18-preview image was booted with
KVM, four vCPUs and 4 GiB RAM. The production candidate registration script built
its package from a staged copy of the released runtime and package database.
Removing only the old runtime's staged database registration modeled fresh
registration with dependency packages already installed.

The successful run verifies:

- Fresh runtime registration leaves `pacman -Dk` clean.
- The archive omits `usr/bin/omarchy-nvim-refresh` and `usr/bin/omarchy-nvim-setup`.
- Upgrading the actual guest from `4.0.3-3` to `4.0.3-4` leaves the database clean.
- Both helpers are owned only by `omarchy-nvim`; their hashes and the refresh
  symlink survive, including an administrator-style edit to the setup script.
- All 2,011 runtime package files are present and a user-document hash is unchanged.
- Another boot with the released v18 kernel/initramfs preserves those results.

The released Neovim package already reports a missing
`/etc/skel/.config/nvim/lua/plugins/theme.lua`. Tracked separately in [#119](https://github.com/omacom/try-omarchy-windows/issues/119).
The test requires its missing-file
diagnostics to remain exactly unchanged; it does not claim that package is wholly
intact. This is separate from the two helper ownership errors repaired here.

Nine unprivileged packaging/migration tests cover archive membership, invalid
command links, database-check failure, regular-file/symlink preservation, replay,
existing replacement files, a missing helper, absent shared ownership and unsafe recovery-directory
links. The complete reconstructed guest contract and existing smoke parser tests
also pass.

The local run verifies production packaging and upgrade behavior against the
released image, not a complete rebuilt factory image or Windows desktop acceptance.
The full guest build/boot CI result is recorded separately below when available.

## Reproduction and evidence

Apply the repository's complete patch series to the locked guest builder, then:

```sh
python3 scripts/release/smoke-runtime-ownership.py \
  /path/to/verified-v18-artifacts /path/to/patched-builder /path/to/new-work-directory
```

The verified artifact directory must include decompressed `rootfs.ext4`, the
kernel, initramfs and build spec. The runner keeps its disposable disk and logs;
never execute its guest fixture scripts on a valued installation.

Local evidence: `/home/bts/Projects/try-omarchy-evidence/issue116-run04`.

| Log | SHA256 |
| --- | --- |
| 01-upgrade.log | `2e19d5073d33aa8324727eeb22f49e9b4ad14d4aa229aae1b590911d76b3c808` |
| 02-reboot.log | `14375532d22250c215cab7c224069d52d2ab0577c269a519e9cb876241b5b3d3` |

Earlier runs retained the observed file-removal failure and intermediate migration
failures. The successful run above includes the preservation behavior. A subsequent
unprivileged regression also verifies that an already-missing helper does not
prevent preserving the other one.
Published v18 assets remain unchanged; this repair requires a new guest payload
and installation of the updated runtime package inside an existing guest.
