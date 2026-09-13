package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Normal installations can be converted directly. Keep the existing verified
// materialization path for QCOW2 sources and their factory-image dependencies.
func stagePortableData(dir, data, stage, tool string, report backupProgress) error {
	for _, name := range []string{payloadUpdateStateFilename, updateStateFilename} {
		if _, err := os.Lstat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			return fmt.Errorf("finish the pending update before creating a portable copy")
		}
	}
	disk, err := inspectInstallationDisk(dir)
	if err != nil {
		return err
	}
	if disk.Format != "raw" {
		archive := filepath.Join(stage, "source.zip")
		if err := writeVMBackupProgress(dir, archive, report); err != nil {
			return err
		}
		if err := restoreVMBackupProgress(archive, data, report); err != nil {
			return err
		}
		return makeRestoredDiskPortable(data, tool, report)
	}
	return stageRawPortableData(dir, data, disk, tool, report)
}

func stageRawPortableData(dir, data string, disk installationDisk, tool string, report backupProgress) error {
	source, err := openBackupDisk(disk.Path)
	if err != nil {
		return fmt.Errorf("close Omarchy before creating a portable copy: %w", err)
	}
	defer source.Close()
	// Use the backup allowlist, excluding checkpoints, retained recovery disks,
	// host-specific state and unrelated files. The inventory hashes every file
	// and budgets nonzero 64 KiB blocks rather than virtual disk capacity.
	files, required, err := inventoryMoveFiltered(dir, source, report, func(name string) bool {
		name = filepath.ToSlash(name)
		return name == "guest" || name == "runtime" || name == "vm" || backupNameAllowed(name)
	})
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, entry := range files {
		if !entry.Directory {
			seen[filepath.ToSlash(entry.Name)] = true
		}
	}
	if err := requiredBackupFiles(seen); err != nil {
		return err
	}
	// Account for QCOW2 metadata at full virtual capacity. Data clusters are
	// already covered by the nonzero-block inventory above; no raw copy or zip
	// is staged. QEMU's default cluster size is explicitly fixed to match it.
	cmd := exec.CommandContext(setupContext(), tool, "measure", "--output=json", "-O", "qcow2", "-o", "cluster_size=65536", "--size", fmt.Sprint(disk.VirtualBytes))
	configureDiskTool(cmd)
	var detail diskToolErrors
	cmd.Stderr = &detail
	output, err := cmd.Output()
	if err != nil {
		if setupCancelled() {
			return errSetupCancelled
		}
		return fmt.Errorf("measuring portable disk: %w: %s", err, detail.String())
	}
	var measurement struct {
		FullyAllocated int64 `json:"fully-allocated"`
	}
	if err := json.Unmarshal(output, &measurement); err != nil || measurement.FullyAllocated < disk.VirtualBytes || measurement.FullyAllocated > backupMaxBytes {
		return fmt.Errorf("invalid portable disk size estimate")
	}
	required += measurement.FullyAllocated - disk.VirtualBytes
	if err := requireDiskSpace(filepath.Dir(data), required); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(data, "vm"), 0700); err != nil {
		return err
	}
	for _, entry := range files {
		target := filepath.Join(data, entry.Name)
		if entry.Directory {
			if err := os.MkdirAll(target, 0700); err != nil {
				return err
			}
			continue
		}
		if filepath.ToSlash(entry.Name) == "vm/disk.raw" {
			continue
		}
		if err := copyMoveFile(dir, target, entry, source, report); err != nil {
			return err
		}
	}
	// Windows' exclusive source handle must close before qemu-img can open it.
	// The caller retains the installation lifecycle lock; QEMU also takes image
	// locks for conversion and comparison. No command writes the source disk.
	if err := source.Close(); err != nil {
		return err
	}
	portable := filepath.Join(data, "vm", "disk.qcow2")
	if report != nil {
		report(0, disk.VirtualBytes, "Creating and verifying compact portable disk")
	}
	for _, args := range [][]string{
		{"convert", "-f", "raw", "-O", "qcow2", "-o", "cluster_size=65536", disk.Path, portable},
		{"compare", "-f", "raw", "-F", "qcow2", disk.Path, portable},
	} {
		if err := checkSetupCancelled(); err != nil {
			return err
		}
		cmd := exec.CommandContext(setupContext(), tool, args...)
		configureDiskTool(cmd)
		var detail diskToolErrors
		cmd.Stderr = &detail
		if err := cmd.Run(); err != nil {
			if setupCancelled() {
				return errSetupCancelled
			}
			return fmt.Errorf("creating portable disk: %w: %s", err, detail.String())
		}
	}
	f, err := os.OpenFile(portable, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	err = f.Sync()
	f.Close()
	if err != nil {
		return err
	}
	converted, err := inspectInstallationDisk(data)
	if err != nil {
		return err
	}
	if converted.Backing != "" || converted.VirtualBytes != (disk.VirtualBytes+511)/512*512 {
		return fmt.Errorf("portable disk has unexpected size or dependencies")
	}
	return nil
}
