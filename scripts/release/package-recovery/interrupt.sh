#!/bin/bash
# Only run inside a disposable guest: intentionally SIGKILL a fixture transaction.
set -euxo pipefail
[[ ! -e /var/lib/pacman/db.lck && ! -L /var/lib/pacman/db.lck ]]
sha256sum -c "$HOME/package-recovery.sha256"
work="$HOME/package-recovery-fixture"
mkdir "$work"
mkdir -p "$work/pkg/usr/share/try-omarchy-recovery-test"
printf 'fixture package data\n' > "$work/pkg/usr/share/try-omarchy-recovery-test/data"
cat > "$work/pkg/.PKGINFO" <<'PKG'
pkgname = try-omarchy-recovery-test
pkgbase = try-omarchy-recovery-test
pkgver = 1.0-1
pkgdesc = Disposable package interruption fixture
url = https://github.com/omacom/try-omarchy-windows
builddate = 1789344000
packager = Try Omarchy recovery test
size = 21
arch = any
license = MIT
PKG
(cd "$work/pkg" && tar --owner=0 --group=0 -cf - .PKGINFO usr | zstd -o "$work/fixture.pkg.tar.zst")
sudo mkdir -p /etc/pacman.d/hooks
sudo tee /etc/pacman.d/hooks/00-try-omarchy-recovery-test.hook <<'HOOK'
[Trigger]
Operation = Install
Type = Package
Target = try-omarchy-recovery-test
[Action]
Description = Pause disposable recovery fixture before package writes
When = PreTransaction
Exec = /bin/sh -c 'touch /run/try-omarchy-recovery-hook; sleep 300'
HOOK
sudo systemd-run --unit=try-omarchy-recovery-test --property=Type=exec /usr/bin/pacman -U --noconfirm "$work/fixture.pkg.tar.zst"
for attempt in $(seq 1 60); do
  [[ -e /run/try-omarchy-recovery-hook ]] && break
  sleep 1
done
[[ -e /run/try-omarchy-recovery-hook && -f /var/lib/pacman/db.lck ]]
# A second transaction must fail without disturbing the real held lock.
sudo stat -c '%i:%s:%Y' /var/lib/pacman/db.lck > "$work/lock-before"
if sudo pacman -U --noconfirm "$work/fixture.pkg.tar.zst" > "$work/busy.log" 2>&1; then exit 1; fi
grep -q 'unable to lock database' "$work/busy.log"
sudo stat -c '%i:%s:%Y' /var/lib/pacman/db.lck > "$work/lock-after"
cmp "$work/lock-before" "$work/lock-after"
sudo systemctl kill --signal=SIGKILL --kill-whom=all try-omarchy-recovery-test.service
for attempt in $(seq 1 30); do
  pgrep -x pacman >/dev/null || break
  sleep 1
done
! pgrep -x pacman
[[ -f /var/lib/pacman/db.lck ]]
sudo journalctl -u try-omarchy-recovery-test.service --no-pager
sha256sum -c "$HOME/package-recovery.sha256"
printf 'controlled-pretransaction-sigkill\n' > "$work/interrupted"
sync
