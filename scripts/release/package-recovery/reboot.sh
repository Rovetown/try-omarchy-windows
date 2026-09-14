#!/bin/bash
set -euxo pipefail
[[ ! -e /var/lib/pacman/db.lck && ! -L /var/lib/pacman/db.lck ]]
pacman -Q try-omarchy-recovery-test
sudo pacman -Qk try-omarchy-recovery-test
db_status=0
sudo pacman -Dk > /tmp/package-db-current.log 2>&1 || db_status=$?
[[ $db_status == "$(cat "$HOME/package-db-baseline.status")" ]]
cmp "$HOME/package-db-baseline.log" /tmp/package-db-current.log
cat /tmp/package-db-current.log
sha256sum -c "$HOME/package-recovery.sha256"
[[ $(cat /usr/share/try-omarchy-recovery-test/data) == 'fixture package data' ]]
