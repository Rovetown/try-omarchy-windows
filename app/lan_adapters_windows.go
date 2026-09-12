//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

func platformLANAdapters() ([]lanAdapter, error) {
	const script = `$ErrorActionPreference='Stop'; $rows=@(Get-NetAdapter | Where-Object Status -eq Up | ForEach-Object {$adapter=$_; Get-NetIPAddress -InterfaceIndex $adapter.ifIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue | ForEach-Object {[pscustomobject]@{Name=$adapter.Name;Address=$_.IPAddress;Identity=([guid]$adapter.InterfaceGuid).ToString()}}}); ConvertTo-Json -InputObject $rows -Compress`
	cmd := exec.CommandContext(setupContext(), system32("WindowsPowerShell\\v1.0\\powershell.exe"), "-NoProfile", "-NonInteractive", "-Command", script)
	configureDiskTool(cmd)
	var data bytes.Buffer
	var detail diskToolErrors
	cmd.Stdout = &data
	cmd.Stderr = &detail
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("reading Windows network adapters: %w: %s", err, detail.String())
	}
	var result []lanAdapter
	if err := json.Unmarshal(data.Bytes(), &result); err != nil {
		return nil, err
	}
	return result, nil
}
