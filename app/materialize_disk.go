package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type diskToolErrors struct{ bytes.Buffer }

func (w *diskToolErrors) Write(p []byte) (int, error) {
	n := len(p)
	if remaining := 4096 - w.Len(); remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = w.Buffer.Write(p)
	}
	return n, nil
}

// materializeInstallationDisk produces a standalone raw disk in private staging.
// QEMU holds its image locks during conversion. No backing file is rewritten.
func materializeInstallationDisk(dir, parent string, disk installationDisk) (string, func(), error) {
	binary := "qemu-img"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	tool := filepath.Join(dir, "runtime", "bin", binary)
	return materializeInstallationDiskWithTool(parent, disk, tool)
}

func materializeInstallationDiskWithTool(parent string, disk installationDisk, tool string) (string, func(), error) {
	if disk.Format == "raw" {
		return disk.Path, func() {}, nil
	}
	if err := requireDiskSpace(parent, disk.VirtualBytes+diskSpaceReserve); err != nil {
		return "", nil, err
	}
	verifyBacking := func() error {
		if disk.Backing == "" {
			return nil
		}
		f, err := os.Open(disk.Backing)
		if err != nil {
			return err
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, setupReader{f}); err != nil {
			return err
		}
		if hex.EncodeToString(h.Sum(nil)) != disk.BackingSHA256 {
			return fmt.Errorf("portable factory image checksum mismatch")
		}
		return nil
	}
	if err := verifyBacking(); err != nil {
		return "", nil, err
	}
	stage, err := os.MkdirTemp(parent, ".try-omarchy-materialize-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(stage) }
	output := filepath.Join(stage, "disk.raw")
	// Explicit raw backing prevents format probing and external backing chains.
	file := func(name string) map[string]any { return map[string]any{"driver": "file", "filename": name} }
	source := map[string]any{"driver": "qcow2", "file": file(disk.Path), "backing": nil}
	if disk.Backing != "" {
		source["backing"] = map[string]any{"driver": "raw", "file": file(disk.Backing)}
	}
	descriptor, _ := json.Marshal(source)
	cmd := exec.CommandContext(setupContext(), tool, "convert", "-O", "raw", "json:"+string(descriptor), output)
	configureDiskTool(cmd)
	var detail diskToolErrors
	cmd.Stderr = &detail
	err = cmd.Run()
	if err != nil && detail.Len() > 0 {
		err = fmt.Errorf("%w: %s", err, strings.TrimSpace(detail.String()))
	}
	if setupCancelled() {
		err = errSetupCancelled
	}
	if err == nil {
		info, statErr := os.Stat(output)
		if statErr != nil {
			err = statErr
		} else if info.Size() != disk.VirtualBytes {
			err = fmt.Errorf("materialized disk size differs from its source")
		}
	}
	if err == nil {
		err = verifyBacking()
	}
	if err == nil {
		f, e := os.OpenFile(output, os.O_RDWR, 0)
		if e != nil {
			err = e
		} else {
			err = f.Sync()
			f.Close()
		}
	}
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("cannot materialize portable disk: %w", err)
	}
	return output, cleanup, nil
}
