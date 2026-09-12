package main

import "testing"

func BenchmarkSparseZeroScan(b *testing.B) {
	for _, size := range []int{32 << 10, 1 << 20} {
		for _, kind := range []string{"zeros", "last-byte", "first-byte"} {
			data := make([]byte, size)
			if kind == "last-byte" {
				data[len(data)-1] = 1
			}
			if kind == "first-byte" {
				data[0] = 1
			}
			b.Run(fmtSize(size)+"/"+kind+"/before", func(b *testing.B) {
				b.SetBytes(int64(size))
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					benchmarkZeroResult = scalarZeroBytes(data)
				}
			})
			b.Run(fmtSize(size)+"/"+kind+"/after", func(b *testing.B) {
				b.SetBytes(int64(size))
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					benchmarkZeroResult = zeroBytes(data)
				}
			})
		}
	}
}

var benchmarkZeroResult bool

func scalarZeroBytes(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}
func fmtSize(n int) string {
	if n == 32<<10 {
		return "32KiB"
	}
	return "1MiB"
}

func TestZeroBytesMatchesScalarAcrossBoundaries(t *testing.T) {
	for _, size := range []int{0, 1, 4095, 4096, moveBlockSize - 1, moveBlockSize, moveBlockSize + 1, 2*moveBlockSize + 3} {
		b := make([]byte, size)
		if !zeroBytes(b) {
			t.Fatalf("zero buffer of %d", size)
		}
		for _, pos := range []int{0, size / 2, size - 1} {
			if pos < 0 || pos >= size {
				continue
			}
			b[pos] = 1
			if zeroBytes(b) {
				t.Fatalf("missed nonzero byte %d/%d", pos, size)
			}
			b[pos] = 0
		}
	}
}
