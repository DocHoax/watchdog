package fleet

import (
	"encoding/json"
	"sync"

	"github.com/DocHoax/watchdog/pkg/model"
)

// TelemetryBuffer provides a thread-safe bounded FIFO queue for buffering telemetry
// on agents during transient network partitions or server unavailability.
type TelemetryBuffer struct {
	mu           sync.RWMutex
	items        []*model.TelemetrySubmission
	capacity     int
	maxBytes     int64
	currentBytes int64
	droppedCount int64
}

// NewTelemetryBuffer initializes a new bounded telemetry buffer.
func NewTelemetryBuffer(capacity int, maxBytes int64) *TelemetryBuffer {
	if capacity <= 0 {
		capacity = 1000
	}
	if maxBytes <= 0 {
		maxBytes = 10 * 1024 * 1024 // 10MB default
	}

	return &TelemetryBuffer{
		items:    make([]*model.TelemetrySubmission, 0, capacity),
		capacity: capacity,
		maxBytes: maxBytes,
	}
}

// Push adds a telemetry submission to the back of the queue. If capacity or byte limits
// are exceeded, it evicts the oldest items to make room, returning dropped=true.
func (b *TelemetryBuffer) Push(sub *model.TelemetrySubmission) (dropped bool) {
	if sub == nil {
		return false
	}

	subBytes := estimateSubmissionBytes(sub)

	b.mu.Lock()
	defer b.mu.Unlock()

	// If a single item exceeds maxBytes, we cannot store it
	if subBytes > b.maxBytes {
		b.droppedCount++
		return true
	}

	// Evict oldest items if capacity or memory limit is exceeded
	for (len(b.items) >= b.capacity || (b.currentBytes+subBytes > b.maxBytes)) && len(b.items) > 0 {
		evicted := b.items[0]
		b.items = b.items[1:]
		b.currentBytes -= estimateSubmissionBytes(evicted)
		b.droppedCount++
		dropped = true
	}

	if b.currentBytes < 0 {
		b.currentBytes = 0
	}

	b.items = append(b.items, sub)
	b.currentBytes += subBytes

	return dropped
}

// Pop removes and returns the oldest telemetry submission from the buffer, or nil if empty.
func (b *TelemetryBuffer) Pop() *model.TelemetrySubmission {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.items) == 0 {
		return nil
	}

	sub := b.items[0]
	b.items = b.items[1:]
	b.currentBytes -= estimateSubmissionBytes(sub)
	if b.currentBytes < 0 || len(b.items) == 0 {
		b.currentBytes = 0
	}

	return sub
}

// PopBatch retrieves and removes up to maxItems from the buffer in FIFO order.
func (b *TelemetryBuffer) PopBatch(maxItems int) []*model.TelemetrySubmission {
	if maxItems <= 0 {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.items) == 0 {
		return nil
	}

	n := maxItems
	if n > len(b.items) {
		n = len(b.items)
	}

	batch := make([]*model.TelemetrySubmission, n)
	copy(batch, b.items[:n])

	for _, item := range batch {
		b.currentBytes -= estimateSubmissionBytes(item)
	}
	if b.currentBytes < 0 {
		b.currentBytes = 0
	}

	b.items = b.items[n:]
	if len(b.items) == 0 {
		b.currentBytes = 0
	}

	return batch
}

// Peek returns the oldest submission without removing it.
func (b *TelemetryBuffer) Peek() *model.TelemetrySubmission {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.items) == 0 {
		return nil
	}
	return b.items[0]
}

// Len returns the current count of items in the buffer.
func (b *TelemetryBuffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.items)
}

// Bytes returns the estimated byte size of buffered telemetry submissions.
func (b *TelemetryBuffer) Bytes() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.currentBytes
}

// DroppedCount returns the total number of submissions dropped due to buffer overflow.
func (b *TelemetryBuffer) DroppedCount() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.droppedCount
}

// Clear empties all buffered telemetry and resets byte counters.
func (b *TelemetryBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.items = b.items[:0]
	b.currentBytes = 0
}

func estimateSubmissionBytes(sub *model.TelemetrySubmission) int64 {
	if sub == nil {
		return 0
	}
	data, err := json.Marshal(sub)
	if err != nil {
		return 512 // fallback estimate
	}
	return int64(len(data))
}
