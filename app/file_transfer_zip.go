package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Check ZIP's directory bounds before its parser allocates entry objects. File
// data may be large; filenames and metadata must remain a small bounded index.
func validateTransferZipDirectory(reader io.ReaderAt, size int64, maxEntries int) error {
	if size < 22 || maxEntries < 1 {
		return fmt.Errorf("invalid transfer archive")
	}
	tail := make([]byte, min(size, int64(65535+22)))
	if _, err := reader.ReadAt(tail, size-int64(len(tail))); err != nil {
		return err
	}
	index := bytes.LastIndex(tail, []byte{'P', 'K', 5, 6})
	if index < 0 || index+22 > len(tail) || index+22+int(binary.LittleEndian.Uint16(tail[index+20:])) != len(tail) {
		return fmt.Errorf("missing transfer archive directory")
	}
	footer := tail[index:]
	if binary.LittleEndian.Uint16(footer[4:]) != 0 || binary.LittleEndian.Uint16(footer[6:]) != 0 || binary.LittleEndian.Uint16(footer[8:]) != binary.LittleEndian.Uint16(footer[10:]) {
		return fmt.Errorf("multi-volume transfer archives are invalid")
	}
	count := uint64(binary.LittleEndian.Uint16(footer[10:]))
	directoryBytes := uint64(binary.LittleEndian.Uint32(footer[12:]))
	offset := uint64(binary.LittleEndian.Uint32(footer[16:]))
	end := uint64(size - int64(len(tail)) + int64(index))
	var locator []byte
	if end >= 20 {
		locator = make([]byte, 20)
		if _, err := reader.ReadAt(locator, int64(end)-20); err != nil {
			return err
		}
	}
	hasLocator := len(locator) == 20 && bytes.Equal(locator[:4], []byte{'P', 'K', 6, 7})
	needsZIP64 := count == 0xffff || directoryBytes == 0xffffffff || offset == 0xffffffff
	if needsZIP64 && !hasLocator {
		return fmt.Errorf("missing ZIP64 locator")
	}
	if hasLocator {
		if binary.LittleEndian.Uint32(locator[4:]) != 0 || binary.LittleEndian.Uint32(locator[16:]) != 1 {
			return fmt.Errorf("invalid ZIP64 locator")
		}
		position := binary.LittleEndian.Uint64(locator[8:])
		if position > end-20 || end-20-position < 56 {
			return fmt.Errorf("invalid ZIP64 directory position")
		}
		record := make([]byte, 56)
		if _, err := reader.ReadAt(record, int64(position)); err != nil {
			return err
		}
		length := binary.LittleEndian.Uint64(record[4:])
		if !bytes.Equal(record[:4], []byte{'P', 'K', 6, 6}) || length < 44 || length > 4096 || position+12+length != end-20 || binary.LittleEndian.Uint32(record[16:]) != 0 || binary.LittleEndian.Uint32(record[20:]) != 0 || binary.LittleEndian.Uint64(record[24:]) != binary.LittleEndian.Uint64(record[32:]) {
			return fmt.Errorf("invalid ZIP64 directory")
		}
		if (count != 0xffff && count != binary.LittleEndian.Uint64(record[32:])) || (directoryBytes != 0xffffffff && directoryBytes != binary.LittleEndian.Uint64(record[40:])) || (offset != 0xffffffff && offset != binary.LittleEndian.Uint64(record[48:])) {
			return fmt.Errorf("inconsistent ZIP64 directory")
		}
		count = binary.LittleEndian.Uint64(record[32:])
		directoryBytes = binary.LittleEndian.Uint64(record[40:])
		offset = binary.LittleEndian.Uint64(record[48:])
		end = position
	}
	if count == 0 || count > uint64(maxEntries) || directoryBytes > 32<<20 || offset > end || directoryBytes != end-offset {
		return fmt.Errorf("transfer archive directory exceeds its bounds")
	}
	actual := uint64(0)
	header := make([]byte, 46)
	for cursor := offset; cursor < end; {
		if end-cursor < 46 || actual >= count {
			return fmt.Errorf("transfer archive entry count mismatch")
		}
		if _, err := reader.ReadAt(header, int64(cursor)); err != nil {
			return err
		}
		nameBytes := uint64(binary.LittleEndian.Uint16(header[28:]))
		extraBytes := uint64(binary.LittleEndian.Uint16(header[30:]))
		commentBytes := uint64(binary.LittleEndian.Uint16(header[32:]))
		length := 46 + nameBytes + extraBytes + commentBytes
		if !bytes.Equal(header[:4], []byte{'P', 'K', 1, 2}) || nameBytes == 0 || nameBytes > 1024 || extraBytes > 4096 || commentBytes > 4096 || length > end-cursor {
			return fmt.Errorf("invalid transfer archive entry metadata")
		}
		actual++
		cursor += length
	}
	if actual != count {
		return fmt.Errorf("transfer archive entry count mismatch")
	}
	return nil
}
