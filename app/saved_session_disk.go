package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type qmpDiskImage struct {
	Filename    string        `json:"filename"`
	VirtualSize int64         `json:"virtual-size"`
	ActualSize  *int64        `json:"actual-size"`
	Backing     *qmpDiskImage `json:"backing-image"`
}

func qmpDiskCopyBudget(image qmpDiskImage) (int64, error) {
	if image.VirtualSize <= 0 || image.VirtualSize > backupMaxBytes {
		return 0, fmt.Errorf("invalid runtime disk size")
	}
	var allocated int64
	for node, depth := &image, 0; node != nil; node, depth = node.Backing, depth+1 {
		if depth > 16 {
			return 0, fmt.Errorf("unsupported runtime backing chain")
		}
		if node.ActualSize == nil {
			return image.VirtualSize + diskSpaceReserve, nil
		}
		if *node.ActualSize < 0 || *node.ActualSize > backupMaxBytes {
			return 0, fmt.Errorf("invalid runtime disk allocation")
		}
		allocated += *node.ActualSize
		if allocated >= image.VirtualSize {
			return image.VirtualSize + diskSpaceReserve, nil
		}
	}
	return allocated + diskSpaceReserve, nil
}

// The runtime copies its open disk through the block graph. This avoids
// bypassing Windows disk locks or reading a backing chain through host files.
// The coordinator keeps the guest paused until memory capture is complete.
func copyPausedVMDisk(ctx context.Context, c *qmpClient, source, destination string, progress backupProgress) (result installationDisk, err error) {
	var state vmRuntimeStatus
	if err := c.Call(ctx, "query-status", nil, &state); err != nil {
		return result, err
	}
	if state.Running || (state.Status != "paused" && state.Status != "prelaunch") {
		return result, fmt.Errorf("pause the guest before copying its disk")
	}
	if !filepath.IsAbs(source) || !filepath.IsAbs(destination) || pathsEqual(source, destination) {
		return result, fmt.Errorf("snapshot disk paths must be separate absolute paths")
	}
	if err := validateMovePath(destination); err != nil {
		return result, err
	}
	var blocks []struct {
		Inserted struct {
			Node  string       `json:"node-name"`
			Image qmpDiskImage `json:"image"`
		} `json:"inserted"`
	}
	if err := c.Call(ctx, "query-block", nil, &blocks); err != nil {
		return result, err
	}
	node := ""
	var disk qmpDiskImage
	for _, block := range blocks {
		if pathsEqual(block.Inserted.Image.Filename, source) {
			if node != "" {
				return result, fmt.Errorf("runtime has multiple matching disks")
			}
			node = block.Inserted.Node
			disk = block.Inserted.Image
		}
	}
	if node == "" {
		return result, fmt.Errorf("the selected disk is not attached to this runtime")
	}
	budget, err := qmpDiskCopyBudget(disk)
	if err != nil {
		return result, err
	}
	if err := requireDiskSpace(filepath.Dir(destination), budget); err != nil {
		return result, err
	}
	f, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return result, err
	}
	err = setSparse(f)
	if err == nil {
		err = f.Truncate(disk.VirtualSize)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return result, err
	}
	if closeErr != nil {
		return result, closeErr
	}
	id := randomCheckpointRollbackID()[:16]
	target := "tom-save-" + id
	job := "tom-save-job-" + id
	if err := c.Call(ctx, "blockdev-add", map[string]any{"driver": "raw", "node-name": target, "file": map[string]any{"driver": "file", "filename": destination}, "discard": "unmap", "detect-zeroes": "unmap"}, nil); err != nil {
		return result, err
	}
	jobStarted := false
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if jobStarted {
			state, queryErr := queryDiskCopyJob(cleanup, c, job)
			if queryErr != nil {
				err = errors.Join(err, queryErr)
				return
			}
			if state.Status != "concluded" {
				if cancelErr := c.Call(cleanup, "job-cancel", map[string]any{"id": job}, nil); cancelErr != nil {
					// Completion may win the race with cancellation.
					latest, queryErr := queryDiskCopyJob(cleanup, c, job)
					if queryErr != nil || latest.Status != "concluded" {
						err = errors.Join(err, cancelErr, queryErr)
						return
					}
				}
				if waitErr := waitForDiskCopy(cleanup, c, job, nil, true); waitErr != nil {
					err = errors.Join(err, waitErr)
					return
				}
			}
			if dismissErr := c.Call(cleanup, "job-dismiss", map[string]any{"id": job}, nil); dismissErr != nil {
				err = errors.Join(err, dismissErr)
				return
			}
		}

		if removeErr := c.Call(cleanup, "blockdev-del", map[string]any{"node-name": target}, nil); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("snapshot disk remains attached as %s: %w", target, removeErr))
		}
	}()
	if err := c.Call(ctx, "blockdev-backup", map[string]any{"job-id": job, "device": node, "target": target, "sync": "full", "auto-dismiss": false, "on-source-error": "report", "on-target-error": "report"}, nil); err != nil {
		return result, err
	}
	jobStarted = true
	if err := waitForDiskCopy(ctx, c, job, progress, false); err != nil {
		return result, err
	}
	if err := c.Call(ctx, "job-dismiss", map[string]any{"id": job}, nil); err != nil {
		return result, err
	}
	jobStarted = false
	return installationDisk{Path: destination, Format: "raw", VirtualBytes: disk.VirtualSize}, nil
}

type diskCopyJob struct {
	ID      string  `json:"id"`
	Status  string  `json:"status"`
	Error   *string `json:"error"`
	Current int64   `json:"current-progress"`
	Total   int64   `json:"total-progress"`
}

func queryDiskCopyJob(ctx context.Context, c *qmpClient, id string) (diskCopyJob, error) {
	var jobs []diskCopyJob
	if err := c.Call(ctx, "query-jobs", nil, &jobs); err != nil {
		return diskCopyJob{}, err
	}
	for _, job := range jobs {
		if job.ID == id {
			return job, nil
		}
	}
	return diskCopyJob{}, fmt.Errorf("snapshot disk-copy job disappeared before verification")
}

func waitForDiskCopy(ctx context.Context, c *qmpClient, id string, progress backupProgress, cancelling bool) error {
	for {
		job, err := queryDiskCopyJob(ctx, c, id)
		if err != nil {
			return err
		}
		if progress != nil {
			progress(job.Current, job.Total, "Saving guest disk")
		}
		if job.Status == "concluded" {
			if job.Error != nil && !cancelling {
				return fmt.Errorf("snapshot disk copy failed: %s", *job.Error)
			}
			return nil
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
