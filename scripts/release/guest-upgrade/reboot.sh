#!/bin/bash
set -euxo pipefail
systemctl is-active try-omarchy-update-repository.service
systemctl is-active try-omarchy-system-ownership.service
[[ $(pacman -Q try-omarchy-runtime) == "try-omarchy-runtime $CANDIDATE_RUNTIME" ]]
[[ $(cat /usr/share/omarchy/version) == "$CANDIDATE_VERSION" ]]
sha256sum -c "$HOME/upgrade-preserve.sha256"
for path in /etc /usr /usr/lib /usr/share; do
  [[ $(stat -c '%u:%g' "$path") == 0:0 ]]
done
sudo pacman -Qk try-omarchy-runtime
sudo systemctl restart try-omarchy-update-repository.service
systemctl is-active try-omarchy-update-repository.service
[[ ! -e /var/lib/pacman/db.lck ]]
sync
