package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"testing"
)

func transferZIPFixture(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	file, err := writer.Create("file.txt")
	if err != nil {
		t.Fatal(err)
	}
	file.Write([]byte("contents"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func transferZIP64Fixture(t *testing.T) []byte {
	t.Helper()
	data := transferZIPFixture(t)
	end := len(data) - 22
	footer := append([]byte(nil), data[end:]...)
	count := uint64(binary.LittleEndian.Uint16(footer[10:]))
	size := uint64(binary.LittleEndian.Uint32(footer[12:]))
	offset := uint64(binary.LittleEndian.Uint32(footer[16:]))
	record := make([]byte, 56)
	copy(record, "PK\x06\x06")
	binary.LittleEndian.PutUint64(record[4:], 44)
	binary.LittleEndian.PutUint16(record[12:], 45)
	binary.LittleEndian.PutUint16(record[14:], 45)
	binary.LittleEndian.PutUint64(record[24:], count)
	binary.LittleEndian.PutUint64(record[32:], count)
	binary.LittleEndian.PutUint64(record[40:], size)
	binary.LittleEndian.PutUint64(record[48:], offset)
	locator := make([]byte, 20)
	copy(locator, "PK\x06\x07")
	binary.LittleEndian.PutUint64(locator[8:], uint64(end))
	binary.LittleEndian.PutUint32(locator[16:], 1)
	binary.LittleEndian.PutUint16(footer[8:], 0xffff)
	binary.LittleEndian.PutUint16(footer[10:], 0xffff)
	binary.LittleEndian.PutUint32(footer[12:], 0xffffffff)
	binary.LittleEndian.PutUint32(footer[16:], 0xffffffff)
	result := append([]byte(nil), data[:end]...)
	result = append(result, record...)
	result = append(result, locator...)
	return append(result, footer...)
}

func TestTransferZIPDirectoryBoundsAndZIP64(t *testing.T) {
	for _, data := range [][]byte{transferZIPFixture(t), transferZIP64Fixture(t)} {
		if _, err := inspectFileArchive(bytes.NewReader(data), int64(len(data)), 100, 1024); err != nil {
			t.Fatal(err)
		}
	}
	for _, kind := range []string{"count", "size", "offset", "comment", "entry-size", "zip64-offset", "zip64-size"} {
		t.Run(kind, func(t *testing.T) {
			data := append([]byte(nil), transferZIPFixture(t)...)
			end := len(data) - 22
			switch kind {
			case "count":
				binary.LittleEndian.PutUint16(data[end+8:], 2)
				binary.LittleEndian.PutUint16(data[end+10:], 2)
			case "size":
				binary.LittleEndian.PutUint32(data[end+12:], 1<<30)
			case "offset":
				binary.LittleEndian.PutUint32(data[end+16:], 0)
			case "comment":
				binary.LittleEndian.PutUint16(data[end+20:], 1)
			case "entry-size":
				offset := binary.LittleEndian.Uint32(data[end+16:])
				binary.LittleEndian.PutUint16(data[offset+30:], 5000)
			case "zip64-offset":
				data = transferZIP64Fixture(t)
				binary.LittleEndian.PutUint64(data[len(data)-22-20+8:], 1<<50)
			case "zip64-size":
				data = transferZIP64Fixture(t)
				binary.LittleEndian.PutUint64(data[len(data)-22-20-56+40:], 1<<40)
			}
			if _, err := inspectFileArchive(bytes.NewReader(data), int64(len(data)), 100, 1024); err == nil {
				t.Fatal("accepted malformed metadata")
			}
		})
	}
}
