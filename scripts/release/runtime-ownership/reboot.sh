#!/bin/bash
set -euxo pipefail
grep -qw omarchy.qemu=1 /proc/cmdline
[[ $(pacman -Q try-omarchy-runtime | cut -d ' ' -f2) == 4.0.3-4 ]]
sudo pacman -Dk
sudo pacman -Qk try-omarchy-runtime
nvim_status=0
pacman -Qk omarchy-nvim > /tmp/nvim-after.log 2>&1 || nvim_status=$?
[[ $nvim_status == "$(cat "$HOME/nvim-before.status")" ]]
cat /tmp/nvim-after.log
cmp "$HOME/nvim-before.log" /tmp/nvim-after.log
[[ $(readlink /usr/bin/omarchy-nvim-refresh) == omarchy-nvim-setup ]]
sha256sum -c "$HOME/nvim-helpers.sha256"
sha256sum -c "$HOME/runtime-ownership.sha256"
for helper in /usr/bin/omarchy-nvim-refresh /usr/bin/omarchy-nvim-setup; do
  [[ $(pacman -Qqo "$helper") == omarchy-nvim ]]
done
