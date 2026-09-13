package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// Versioned payloads allow the previous launcher to keep using its own manifest
// during rollback. Publishing a new payload never replaces the old directory.
func portablePayloadDirectory(root, digest string) string {
	digest = normalizedSHA256(digest)
	if validSHA256(digest) {
		candidate := filepath.Join(root, digest)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return root
}

func stagePortablePayload(root, release, digest string, client *http.Client, report downloadProgress) error {
	digest = normalizedSHA256(digest)
	if !validSHA256(digest) {
		return fmt.Errorf("invalid portable payload identity")
	}
	if err := validateMovePath(root); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	final := filepath.Join(root, digest)
	if err := validateMovePath(final); err != nil {
		return err
	}
	if _, err := os.Lstat(final); err == nil {
		sums, err := readPortableManifest(filepath.Join(final, "SHA256SUMS"), digest)
		if err != nil {
			return err
		}
		for _, name := range append([]string{runtimeZip, "rootfs.ext4.zst"}, downloadedGuestArtifacts...) {
			ok, err := verifyFileSHA256(filepath.Join(final, name), sums[name], nil)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("stored portable payload is damaged: %s", name)
			}
		}
		_, err = readGuestArtifactSizes(filepath.Join(final, "guest-manifest.json"), sums)
		return err
	} else if !os.IsNotExist(err) {
		return err
	}
	stage, err := os.MkdirTemp(root, ".payload-staging-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := downloadVerified(client, normalizedRelease(release)+"/SHA256SUMS", filepath.Join(stage, "SHA256SUMS"), digest, report); err != nil {
		return err
	}
	sums, err := readPortableManifest(filepath.Join(stage, "SHA256SUMS"), digest)
	if err != nil {
		return err
	}
	names := append([]string{runtimeZip, "rootfs.ext4.zst"}, downloadedGuestArtifacts...)
	for _, name := range names {
		if !validSHA256(sums[name]) {
			return fmt.Errorf("portable update is missing %s", name)
		}
		if err := downloadVerified(client, normalizedRelease(release)+"/"+name, filepath.Join(stage, name), sums[name], report); err != nil {
			return err
		}
	}
	if _, err := readGuestArtifactSizes(filepath.Join(stage, "guest-manifest.json"), sums); err != nil {
		return err
	}
	if err := checkSetupCancelled(); err != nil {
		return err
	}
	if _, err := os.Lstat(final); !os.IsNotExist(err) {
		return fmt.Errorf("portable payload destination appeared during staging")
	}
	return os.Rename(stage, final)
}
