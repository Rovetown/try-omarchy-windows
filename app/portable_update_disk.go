package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// Detach a legacy factory-backed disk before publishing a different factory
// image. The independent disk remains usable with either old or new boot files.
func preparePortablePayloadTransition(cfg *config, release, manifest string) error {
	if !cfg.portable {
		return nil
	}
	if _, err := os.Lstat(cfg.disk); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	oldRelease, oldManifest, ok := installReceiptIdentity(cfg.guestDir)
	if !ok {
		return fmt.Errorf("portable image receipt is missing; restore the matching installation before updating")
	}
	if releaseLocationsEquivalent(oldRelease, release) && normalizedSHA256(oldManifest) == normalizedSHA256(manifest) {
		return nil
	}
	disk, err := inspectInstallationDisk(cfg.dir)
	if err != nil {
		return err
	}
	if disk.Backing == "" {
		return nil
	}
	if _, err := (checkpointStore{installation: cfg.dir}).Create("Before portable update "+time.Now().Format("2006-01-02 15:04"), nil); err != nil {
		return fmt.Errorf("preserving the portable installation before update: %w", err)
	}
	tool := "qemu-img"
	if runtime.GOOS == "windows" {
		tool += ".exe"
	}
	return detachPortableDisk(cfg.dir, disk, filepath.Join(cfg.dir, "runtime", "bin", tool), publishMoveFile)
}

func detachPortableDisk(dir string, disk installationDisk, tool string, publish func(string, string) error) error {
	if disk.Format != "qcow2" || disk.Backing == "" {
		return nil
	}
	// Reuse the verified conversion path. Its temporary raw disk is independent
	// of the old factory and is retained until the replacement has been compared.
	raw, cleanup, err := materializeInstallationDiskWithTool(filepath.Join(dir, "vm"), disk, tool)
	if err != nil {
		return err
	}
	defer cleanup()
	converted := filepath.Join(filepath.Dir(raw), "independent.qcow2")
	cmd := exec.CommandContext(setupContext(), tool, "convert", "-f", "raw", "-O", "qcow2", raw, converted)
	configureDiskTool(cmd)
	var detail diskToolErrors
	cmd.Stderr = &detail
	if err := cmd.Run(); err != nil {
		if setupCancelled() {
			return errSetupCancelled
		}
		return fmt.Errorf("detaching portable disk: %w: %s", err, detail.String())
	}
	cmd = exec.CommandContext(setupContext(), tool, "compare", "-f", "raw", "-F", "qcow2", raw, converted)
	configureDiskTool(cmd)
	if err := cmd.Run(); err != nil {
		if setupCancelled() {
			return errSetupCancelled
		}
		return fmt.Errorf("verifying independent portable disk: %w", err)
	}
	f, err := os.OpenFile(converted, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	err = f.Sync()
	f.Close()
	if err != nil {
		return err
	}
	if err := checkSetupCancelled(); err != nil {
		return err
	}
	// The checkpoint already retains the original bootable state. Replace only
	// the active disk in one filesystem operation, with no missing-disk interval.
	if err := publish(converted, disk.Path); err != nil {
		return err
	}
	return nil
}
