package collector

import (
	"context"
	"os"
	"testing"

	"github.com/DocHoax/watchdog/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockProcfsReader struct {
	pids        []int32
	stats       map[int32][]byte
	statuses    map[int32][]byte
	statms      map[int32][]byte
	cmdlines    map[int32][]byte
	exes        map[int32]string
	bootTime    uint64
	totalMem    uint64
	usernames   map[string]string
	listErr     error
	statErrs    map[int32]error
}

func (m *mockProcfsReader) ListPIDs(ctx context.Context) ([]int32, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.pids, nil
}

func (m *mockProcfsReader) ReadStat(ctx context.Context, pid int32) ([]byte, error) {
	if err, ok := m.statErrs[pid]; ok && err != nil {
		return nil, err
	}
	if data, ok := m.stats[pid]; ok {
		return data, nil
	}
	return nil, os.ErrNotExist
}

func (m *mockProcfsReader) ReadStatus(ctx context.Context, pid int32) ([]byte, error) {
	if data, ok := m.statuses[pid]; ok {
		return data, nil
	}
	return nil, os.ErrNotExist
}

func (m *mockProcfsReader) ReadStatm(ctx context.Context, pid int32) ([]byte, error) {
	if data, ok := m.statms[pid]; ok {
		return data, nil
	}
	return []byte("1000 200 150 50 0 10 0"), nil
}

func (m *mockProcfsReader) ReadCmdline(ctx context.Context, pid int32) ([]byte, error) {
	if data, ok := m.cmdlines[pid]; ok {
		return data, nil
	}
	return []byte("watchdog-test\x00arg1"), nil
}

func (m *mockProcfsReader) ReadExe(ctx context.Context, pid int32) (string, error) {
	if exe, ok := m.exes[pid]; ok {
		return exe, nil
	}
	return "/usr/bin/watchdog-test", nil
}

func (m *mockProcfsReader) GetBootTime(ctx context.Context) (uint64, error) {
	if m.bootTime > 0 {
		return m.bootTime, nil
	}
	return uint64(1700000000), nil
}

func (m *mockProcfsReader) GetTotalMemory(ctx context.Context) (uint64, error) {
	if m.totalMem > 0 {
		return m.totalMem, nil
	}
	return uint64(16 * 1024 * 1024 * 1024), nil // 16 GB
}

func (m *mockProcfsReader) LookupUsername(uid string) (string, error) {
	if name, ok := m.usernames[uid]; ok {
		return name, nil
	}
	return "root", nil
}

func TestProcessSnapshotCollector_DeterministicFixture(t *testing.T) {
	// Stat format: pid (comm) state ppid ...
	// S is sleeping state
	statContent := []byte("1234 (foo (bar) baz) S 1 1234 0 0 -1 4194304 0 0 0 0 100 200 0 0 0 0 1 0 1700001000 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 50")
	statusContent := []byte("Name:\tfoo-bar\nUid:\t1000\t1000\t1000\t1000\nThreads:\t4\n")
	statmContent := []byte("25600 10240 5120 100 0 50 0")
	cmdlineContent := []byte("/usr/bin/foo-bar\x00--config\x00test.yaml")

	mockReader := &mockProcfsReader{
		pids:      []int32{1234},
		stats:     map[int32][]byte{1234: statContent},
		statuses:  map[int32][]byte{1234: statusContent},
		statms:    map[int32][]byte{1234: statmContent},
		cmdlines:  map[int32][]byte{1234: cmdlineContent},
		exes:      map[int32]string{1234: "/usr/bin/foo-bar"},
		bootTime:  uint64(1700000000),
		totalMem:  16 * 1024 * 1024 * 1024,
		usernames: map[string]string{"1000": "muhammad"},
	}

	c := NewProcessCollectorWithReader(mockReader)
	ctx := context.Background()

	summary, err := c.GetProcesses(ctx, 10, "pid", "")
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, 1, summary.TotalCount)
	require.Len(t, summary.Processes, 1)

	p := summary.Processes[0]
	assert.Equal(t, int32(1234), p.PID)
	assert.Equal(t, int32(1), p.PPID)
	assert.Equal(t, "foo-bar", p.Name)
	assert.Equal(t, "muhammad", p.Username)
	assert.Equal(t, int32(4), p.NumThreads)
	assert.Equal(t, "Sleeping", p.Status)
	assert.Equal(t, uint64(10240*4096), p.MemoryRSS)
	assert.Equal(t, uint64(25600*4096), p.MemoryVMS)
	assert.Equal(t, "/usr/bin/foo-bar --config test.yaml", p.CommandLine)
	assert.Equal(t, "/usr/bin/foo-bar", p.ExePath)
}

func TestProcessSnapshotCollector_ErrorPolicies(t *testing.T) {
	mockReader := &mockProcfsReader{
		pids: []int32{101, 102, 103},
		stats: map[int32][]byte{
			101: []byte("101 (proc1) S 1 0 0 0 0 0 0 0 0 0 10 10 0 0 0 0 1 0 1700001000 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0"),
		},
		statErrs: map[int32]error{
			102: os.ErrNotExist,   // ENOENT
			103: os.ErrPermission, // EACCES
		},
	}

	c := NewProcessCollectorWithReader(mockReader)
	ctx := context.Background()

	summary, err := c.GetProcesses(ctx, 0, "pid", "")
	require.NoError(t, err)
	require.NotNil(t, summary)

	assert.Equal(t, 2, summary.TotalCount)

	pids := []int32{summary.Processes[0].PID, summary.Processes[1].PID}
	assert.Contains(t, pids, int32(101))
	assert.Contains(t, pids, int32(103))

	var partial model.ProcessInfo
	for _, p := range summary.Processes {
		if p.PID == 103 {
			partial = p
		}
	}
	assert.Equal(t, int32(103), partial.PID)
	assert.Equal(t, "PID-103", partial.Name)
	assert.Equal(t, "Unknown", partial.Status)
}

func TestProcessSnapshotCollector_ContextCancellation(t *testing.T) {
	mockReader := &mockProcfsReader{
		pids: []int32{1, 2, 3, 4, 5},
	}
	c := NewProcessCollectorWithReader(mockReader)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GetProcesses(ctx, 0, "pid", "")
	assert.Error(t, err)
}
