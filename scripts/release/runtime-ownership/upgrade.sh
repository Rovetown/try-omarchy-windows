#!/bin/bash
# Run only through the disposable-guest runner.
set -euxo pipefail
grep -qw omarchy.qemu=1 /proc/cmdline
[[ $(pacman -Q try-omarchy-runtime | cut -d ' ' -f2) == 4.0.3-3 ]]
nvim_status=0
pacman -Qk omarchy-nvim > "$HOME/nvim-before.log" 2>&1 || nvim_status=$?
printf '%s\n' "$nvim_status" > "$HOME/nvim-before.status"
cat "$HOME/nvim-before.log"
helpers=(/usr/bin/omarchy-nvim-refresh /usr/bin/omarchy-nvim-setup)
printf '\n# retained administrator customization\n' | sudo tee -a /usr/bin/omarchy-nvim-refresh
sha256sum "${helpers[@]}" > "$HOME/nvim-helpers.sha256"
mkdir -p "$HOME/Documents"
printf 'runtime ownership upgrade preservation\n' > "$HOME/Documents/runtime-ownership.txt"
sha256sum "$HOME/Documents/runtime-ownership.txt" > "$HOME/runtime-ownership.sha256"
# Record the released defect before changing any package metadata.
if sudo pacman -Dk > "$HOME/ownership-before.log" 2>&1; then exit 1; fi
cat "$HOME/ownership-before.log"
grep -q "file owned by 'omarchy-nvim' and 'try-omarchy-runtime'" "$HOME/ownership-before.log"
root="$HOME/runtime-staging"
work="$HOME/runtime-build"
mkdir "$root" "$work"
sudo mkdir -p "$root/usr/bin" "$root/usr/share/licenses" "$root/var/lib/pacman" "$root/var/log"
sudo cp -a /usr/share/omarchy "$root/usr/share/"
sudo cp -a /usr/share/licenses/omarchy "$root/usr/share/licenses/"
sudo cp -a /usr/bin/omarchy /usr/bin/omarchy-* "$root/usr/bin/"
sudo cp -a /var/lib/pacman/local "$root/var/lib/pacman/"
sudo mkdir -p "$root/usr/share/try-omarchy"
sudo cp -a /usr/share/try-omarchy/runtime-config-files "$root/usr/share/try-omarchy/"
while IFS= read -r path; do
  sudo cp -a --parents "/$path" "$root/"
done < /usr/share/try-omarchy/runtime-config-files
# Model fresh registration: dependency packages are present, runtime not yet registered.
sudo pacman --root "$root" --dbpath "$root/var/lib/pacman" -Rdd --dbonly --noconfirm try-omarchy-runtime
sudo bash /mnt/host/register-omarchy-runtime.sh --root "$root" --work "$work" --spec /mnt/host/spec.json --pacman-config /etc/pacman.conf
sudo pacman --root "$root" --dbpath "$root/var/lib/pacman" -Dk
archive="$root/usr/share/try-omarchy/repo/try-omarchy-runtime-4.0.3-4-any.pkg.tar.zst"
# New runtime archive must not contain the dependency-owned helper paths.
if bsdtar -tf "$archive" | grep -E '^usr/bin/omarchy-nvim-(refresh|setup)$'; then exit 1; fi
sudo pacman -U --noconfirm "$archive"
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
for helper in "${helpers[@]}"; do
  [[ $(pacman -Qqo "$helper") == omarchy-nvim ]]
done
sync
