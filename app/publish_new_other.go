//go:build !windows && !linux

package main

import "fmt"

func publishNewDirectory(from, to string) error {
	return fmt.Errorf("atomic no-replacement publication requires Windows or Linux")
}
