# Next-preview performance audit

Baseline source: v0.0.14-preview, `7fb4fb1`. Local measurements: 2026-09-06,
Linux amd64, Intel Core i7-14700F. These are algorithm benchmarks on a shared
desktop, not Windows end-to-end results.

## Sparse zero detection

Restore previously checked each byte with a scalar loop before deciding whether
to write or seek over an empty region. The new helper compares against a shared
zero buffer using `bytes.Equal`, retaining a fast return when the first byte is
nonzero. The move copier uses the same helper. Both versions allocate zero bytes
per scan.

The benchmark retains the original loop as its `before` comparator. Five
100 ms samples per case, medians below:

| Input | Before | After | Reduction |
| --- | ---: | ---: | ---: |
| 32 KiB, all zero | 7,841 ns | 321.1 ns | 95.9% |
| 32 KiB, final byte nonzero | 10,147 ns | 324.5 ns | 96.8% |
| 1 MiB, all zero | 313,544 ns | 9,876 ns | 96.9% |
| 1 MiB, final byte nonzero | 310,360 ns | 10,355 ns | 96.7% |

The 32 KiB zero case ranged from 7,683 to 10,063 ns before and 293.8 to 327.3 ns
after. For 1 MiB it ranged from 301,019 to 327,013 ns before and 9,778 to 10,245 ns
after. Inputs beginning with a nonzero byte remained around 0.5 ns in this
microbenchmark; differences at that scale are not meaningful end-to-end gains.

Reproduce from `app`:

```sh
go test -run '^$' -bench BenchmarkSparseZeroScan -benchmem -benchtime=100ms -count=5
```

These numbers do not imply that complete restores are 24 to 32 times faster.
ZIP decompression, hashing, sparse-file operations, and storage I/O still take
time. Measure full operations on Windows before making a product-level claim.

## Other audit results

- Reclaim already uses `bytes.Equal`; replacing that comparison was unnecessary.
  Its reader now rejects short reads before examining a partially filled buffer.
- Runtime receipt parsing had three copies of the same decoding logic. A shared,
  bounded reader removes that duplication while callers retain their distinct
  trust checks. No startup speed claim is made for this cleanup.
- Clipboard polling and QMP supervision remain unchanged. Their existing
  deduplication and recovery behavior need Windows measurements before changing
  wakeups or transport behavior.
- Reclaim still reads the full logical disk. Allocated-range scanning remains a
  candidate for a separate measured Windows change.
- Moving an installation reads data for space estimation, copying, and
  verification. This provides a verified transfer without a temporary ZIP;
  its throughput still needs Windows measurement.

## Remaining measurements

Record the exact launcher, guest, and runtime hashes, host configuration,
logical disk capacity, and allocated size. Compare at least five equivalent
runs of baseline and candidate for cold/warm launch, backup, restore, reclaim,
and moves. Record launcher and QEMU idle CPU separately after the guest settles,
memory use, elapsed time, and disk I/O. Test populated and mostly empty disks,
GPU rendering and CPU fallback. Native Windows and physical-hardware results
must remain distinct from these Linux microbenchmarks.
