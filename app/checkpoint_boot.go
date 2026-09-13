package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const checkpointBootFilename = ".snapshot-first-boot"

func markCheckpointBoot(dir string) error {
	file, err := os.OpenFile(filepath.Join(dir, "vm", checkpointBootFilename), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = file.WriteString("1\n")
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

// Preserve restored boot files through one successful guest boot, just as an
// interrupted payload update does. Explicit release flags remain authoritative.
func pinCheckpointBoot(dir string, explicit map[string]bool, guestRelease, guestManifest, runtimeRelease, runtimeManifest *string) (bool, error) {
	path := filepath.Join(dir, "vm", checkpointBootFilename)
	if err := validateMovePath(path); err != nil {
		return false, err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Size() != 2 {
		return false, fmt.Errorf("invalid snapshot boot marker")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if string(data) != "1\n" {
		return false, fmt.Errorf("invalid snapshot boot marker")
	}
	for _, flag := range []string{"release", "sums-sha256", "runtime-release", "runtime-sums-sha256"} {
		if explicit[flag] {
			return false, nil
		}
	}
	if err := pinRestoredPayloads(dir, guestRelease, guestManifest, runtimeRelease, runtimeManifest); err != nil {
		return false, err
	}
	return true, nil
}

func commitCheckpointBoot(dir string) {
	if err := os.Remove(filepath.Join(dir, "vm", checkpointBootFilename)); err != nil && !os.IsNotExist(err) {
		logf("recording successful snapshot boot: %v", err)
	}
}
