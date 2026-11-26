package collector

import (
	"context"
	"testing"
)

func BenchmarkProcessCollector_GetProcesses(b *testing.B) {
	statContent := []byte("1234 (watchdog) S 1 1234 0 0 -1 4194304 0 0 0 0 100 200 0 0 0 0 1 0 1700001000 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 50")
	statusContent := []byte("Name:\twatchdog\nUid:\t1000\t1000\t1000\t1000\nThreads:\t4\n")
	statmContent := []byte("25600 10240 5120 100 0 50 0")
	cmdlineContent := []byte("/usr/bin/watchdog\x00--config\x00config.yaml")

	pids := make([]int32, 100)
	stats := make(map[int32][]byte)
	statuses := make(map[int32][]byte)
	statms := make(map[int32][]byte)
	cmdlines := make(map[int32][]byte)
	exes := make(map[int32]string)

	for i := int32(1); i <= 100; i++ {
		pids[i-1] = i
		stats[i] = statContent
		statuses[i] = statusContent
		statms[i] = statmContent
		cmdlines[i] = cmdlineContent
		exes[i] = "/usr/bin/watchdog"
	}

	mockReader := &mockProcfsReader{
		pids:      pids,
		stats:     stats,
		statuses:  statuses,
		statms:    statms,
		cmdlines:  cmdlines,
		exes:      exes,
		bootTime:  uint64(1700000000),
		totalMem:  16 * 1024 * 1024 * 1024,
		usernames: map[string]string{"1000": "muhammad"},
	}

	c := NewProcessCollectorWithReader(mockReader)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := c.GetProcesses(ctx, 0, "cpu", "")
		if err != nil {
			b.Fatal(err)
		}
	}
}
