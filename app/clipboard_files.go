package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// File clipboard transfers are snapshots, never moves. Large selections should
// use the shared folder instead. Both peers enforce these limits independently.
const (
	maxClipboardArchiveBytes = 16 << 20
	maxClipboardFileBytes    = 64 << 20
	maxClipboardFileEntries  = 1024
	maxClipboardCacheBytes   = 256 << 20
)

func clipboardFileName(name string) bool {
	if name == "" || len(name) > 1024 || !utf8.ValidString(name) || strings.ContainsAny(name, "\\:\x00\r\n") || strings.HasPrefix(name, "/") {
		return false
	}
	for _, part := range strings.Split(strings.TrimSuffix(name, "/"), "/") {
		if part == "" || part == "." || part == ".." || strings.TrimRight(part, " .") != part || strings.ContainsAny(part, "<>\"|?*") {
			return false
		}
		for _, c := range part {
			if c < 32 {
				return false
			}
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CONIN$" || base == "CONOUT$" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
			return false
		}
	}
	return true
}

func inspectClipboardArchive(data []byte) (*zip.Reader, error) {
	if len(data) == 0 || len(data) > maxClipboardArchiveBytes {
		return nil, fmt.Errorf("file selection exceeds the 16 MiB clipboard limit; use the shared folder")
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	if len(z.File) == 0 || len(z.File) > maxClipboardFileEntries {
		return nil, fmt.Errorf("clipboard supports 1 to %d entries", maxClipboardFileEntries)
	}
	seen := map[string]bool{}
	spelling := map[string]string{}
	var size uint64
	for _, f := range z.File {
		name := strings.TrimSuffix(f.Name, "/")
		if !clipboardFileName(f.Name) || (f.Mode().Type() != 0 && !f.FileInfo().IsDir()) || f.Flags&1 != 0 {
			return nil, fmt.Errorf("unsupported clipboard file name or type")
		}
		for prefix := name; prefix != "."; prefix = path.Dir(prefix) {
			key := strings.ToLower(prefix)
			if prior, ok := spelling[key]; ok && prior != prefix {
				return nil, fmt.Errorf("clipboard directory names differ only by case")
			}
			spelling[key] = prefix
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate clipboard name")
		}
		seen[key] = f.FileInfo().IsDir()
		if f.UncompressedSize64 > maxClipboardFileBytes || size > maxClipboardFileBytes-f.UncompressedSize64 {
			return nil, fmt.Errorf("clipboard files exceed 64 MiB")
		}
		size += f.UncompressedSize64
	}
	for name := range seen {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if isDir, exists := seen[parent]; !exists || !isDir {
				return nil, fmt.Errorf("clipboard file used as a directory")
			}
		}
	}
	return z, nil
}

type clipboardArchiveBuffer struct{ bytes.Buffer }

func (b *clipboardArchiveBuffer) Write(p []byte) (int, error) {
	if len(p) > maxClipboardArchiveBytes-b.Len() {
		return 0, fmt.Errorf("file selection exceeds the 16 MiB clipboard limit; use the shared folder")
	}
	return b.Buffer.Write(p)
}

func packClipboardFiles(paths []string) ([]byte, error) {
	if len(paths) == 0 || len(paths) > maxClipboardFileEntries {
		return nil, fmt.Errorf("too many clipboard files")
	}
	var out clipboardArchiveBuffer
	z := zip.NewWriter(&out)
	count := 0
	var total int64
	for _, source := range paths {
		source = filepath.Clean(source)
		root, err := os.OpenRoot(filepath.Dir(source))
		if err != nil {
			return nil, err
		}
		err = func() error {
			defer root.Close()
			base := filepath.Base(source)
			return fs.WalkDir(root.FS(), base, func(name string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				count++
				if count > maxClipboardFileEntries {
					return fmt.Errorf("too many clipboard entries")
				}
				if !clipboardFileName(name) {
					return fmt.Errorf("file name cannot be transferred between Windows and Linux")
				}
				info, err := d.Info()
				if err != nil {
					return err
				}
				if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
					return fmt.Errorf("links and special files must be copied through the shared folder")
				}
				header := &zip.FileHeader{Name: name, Method: zip.Deflate}
				if info.IsDir() {
					header.Name += "/"
					header.SetMode(0700 | os.ModeDir)
				} else {
					header.SetMode(0600)
				}
				w, err := z.CreateHeader(header)
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				f, err := root.Open(name)
				if err != nil {
					return err
				}
				defer f.Close()
				actual, err := f.Stat()
				if err != nil {
					return err
				}
				if !actual.Mode().IsRegular() || !os.SameFile(info, actual) {
					return fmt.Errorf("clipboard source changed during copy")
				}
				n, err := io.Copy(w, io.LimitReader(f, maxClipboardFileBytes-total+1))
				total += n
				if err != nil {
					return err
				}
				if total > maxClipboardFileBytes {
					return fmt.Errorf("clipboard files exceed 64 MiB; use the shared folder")
				}
				return nil
			})
		}()
		if err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	if _, err := inspectClipboardArchive(out.Bytes()); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func unpackClipboardFiles(data []byte, cache string) (paths []string, err error) {
	z, err := inspectClipboardArchive(data)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(cache, 0700); err != nil {
		return nil, err
	}
	var used int64
	count := 0
	err = filepath.WalkDir(cache, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		count++
		if count > 16384 {
			return fmt.Errorf("clipboard cache is full; remove old copies from %s", cache)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("clipboard cache contains a link")
		}
		if !d.IsDir() {
			i, e := d.Info()
			if e != nil {
				return e
			}
			used += i.Size()
			if used > maxClipboardCacheBytes {
				return fmt.Errorf("clipboard cache is full; remove old copies from %s", cache)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var incoming uint64
	for _, f := range z.File {
		incoming += f.UncompressedSize64
	}
	if count+len(z.File)+1 > 16384 || uint64(used)+incoming > maxClipboardCacheBytes {
		return nil, fmt.Errorf("clipboard cache is full; remove old copies from %s", cache)
	}
	dir, err := os.MkdirTemp(cache, "copy-")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(dir)
		}
	}()
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	top := map[string]bool{}
	for _, f := range z.File {
		name := strings.TrimSuffix(f.Name, "/")
		first := strings.SplitN(name, "/", 2)[0]
		if !top[first] {
			paths = append(paths, filepath.Join(dir, first))
			top[first] = true
		}
		if f.FileInfo().IsDir() {
			err = root.MkdirAll(name, 0700)
			if err != nil {
				return nil, err
			}
			continue
		}
		if err = root.MkdirAll(path.Dir(name), 0700); err != nil {
			return nil, err
		}
		var output *os.File
		output, err = root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		input, e := f.Open()
		if e != nil {
			output.Close()
			return nil, e
		}
		n, e := io.Copy(output, io.LimitReader(input, int64(f.UncompressedSize64)+1))
		input.Close()
		closeErr := output.Close()
		if e != nil {
			return nil, e
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if uint64(n) != f.UncompressedSize64 {
			return nil, fmt.Errorf("clipboard archive size mismatch")
		}
	}
	return paths, nil
}
