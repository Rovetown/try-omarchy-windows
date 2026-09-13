package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// installationDisk describes the complete local disk dependency, independently
// of launch mode. Callers hold the launcher lifecycle lock during mutations.
type installationDisk struct {
	Path          string
	Format        string
	VirtualBytes  int64
	Backing       string
	BackingSHA256 string
}

func inspectInstallationDisk(dir string) (installationDisk, error) {
	var result installationDisk
	for _, format := range []string{"raw", "qcow2"} {
		name := filepath.Join(dir, "vm", "disk."+format)
		info, err := os.Lstat(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return result, err
		}
		if result.Path != "" {
			return installationDisk{}, fmt.Errorf("both raw and QCOW2 disks exist; resolve the active disk before recovery")
		}
		if err := validateMovePath(name); err != nil {
			return result, err
		}
		if !info.Mode().IsRegular() || info.Size() <= 0 {
			return result, fmt.Errorf("invalid guest disk")
		}
		result = installationDisk{Path: name, Format: format, VirtualBytes: info.Size()}
	}
	if result.Path == "" {
		return result, fmt.Errorf("no guest disk found")
	}
	if result.Format == "raw" {
		return result, nil
	}
	f, err := os.Open(result.Path)
	if err != nil {
		return result, err
	}
	defer f.Close()
	header := make([]byte, qcow2HeaderSize)
	if _, err := io.ReadFull(f, header); err != nil {
		return result, err
	}
	if binary.BigEndian.Uint32(header[32:36]) != 0 || binary.BigEndian.Uint64(header[72:80]) != 0 {
		return result, fmt.Errorf("portable disk requires repair or uses unsupported image features")
	}
	size := binary.BigEndian.Uint64(header[24:32])
	if size == 0 || size > uint64(backupMaxBytes) {
		return result, fmt.Errorf("unsupported portable disk size")
	}
	result.VirtualBytes = int64(size)
	if binary.BigEndian.Uint32(header[0:4]) != 0x514649fb || binary.BigEndian.Uint32(header[4:8]) != 3 {
		return result, fmt.Errorf("unsupported QCOW2 header")
	}
	if binary.BigEndian.Uint64(header[8:16]) == 0 && binary.BigEndian.Uint32(header[16:20]) == 0 {
		return result, nil
	}

	valid, err := qcow2OverlayMatches(result.Path, "../guest/rootfs.ext4", result.VirtualBytes)
	if err != nil {
		return result, err
	}
	if !valid {
		return result, fmt.Errorf("portable disk has an unsupported backing chain")
	}
	result.Backing = filepath.Join(dir, "guest", "rootfs.ext4")
	if err := validateMovePath(result.Backing); err != nil {
		return result, err
	}
	digest, ok := installReceiptArtifactSHA256(filepath.Join(dir, "guest"), "rootfs.ext4")
	if !ok {
		return result, fmt.Errorf("portable factory image identity is missing")
	}
	matches, err := portableBackingStateMatches(result.Path, digest)
	if err != nil {
		return result, err
	}
	if !matches {
		return result, fmt.Errorf("portable disk belongs to a different factory image")
	}
	result.BackingSHA256 = digest
	return result, nil
}
