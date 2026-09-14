#!/bin/bash
set -euxo pipefail
work="$HOME/package-recovery-fixture"
grep -qx controlled-pretransaction-sigkill "$work/interrupted"
[[ -f /var/lib/pacman/db.lck && ! -L /var/lib/pacman/db.lck ]]
! pgrep -x pacman
sha256sum -c "$HOME/package-recovery.sha256"
# Boot-time repository publication must not delete the stale lock either.
sudo systemctl restart try-omarchy-update-repository.service
[[ -f /var/lib/pacman/db.lck ]]
if sudo pacman -U --noconfirm "$work/fixture.pkg.tar.zst" > "$work/stale.log" 2>&1; then exit 1; fi
grep -q 'unable to lock database' "$work/stale.log"
db_status=0
sudo pacman -Dk > /tmp/package-db-current.log 2>&1 || db_status=$?
[[ $db_status == "$(cat "$HOME/package-db-baseline.status")" ]]
cmp "$HOME/package-db-baseline.log" /tmp/package-db-current.log
cat /tmp/package-db-current.log
# This test knows exactly which transaction created the lock and killed its
# entire service cgroup before reboot. Never apply this removal blindly.
command -v fuser
set +e
sudo fuser /var/lib/pacman/db.lck
holder_status=$?
set -e
[[ $holder_status == 1 ]]
sudo rm /etc/pacman.d/hooks/00-try-omarchy-recovery-test.hook
sudo rm /var/lib/pacman/db.lck
sudo pacman -U --noconfirm "$work/fixture.pkg.tar.zst"
pacman -Q try-omarchy-recovery-test
sudo pacman -Qk try-omarchy-recovery-test
[[ $(cat /usr/share/try-omarchy-recovery-test/data) == 'fixture package data' ]]
db_status=0
sudo pacman -Dk > /tmp/package-db-current.log 2>&1 || db_status=$?
[[ $db_status == "$(cat "$HOME/package-db-baseline.status")" ]]
cmp "$HOME/package-db-baseline.log" /tmp/package-db-current.log
cat /tmp/package-db-current.log
sha256sum -c "$HOME/package-recovery.sha256"
[[ ! -e /var/lib/pacman/db.lck ]]
sync
