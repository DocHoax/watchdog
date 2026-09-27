package fleet

import (
	"sync"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestTelemetryBuffer_PushPop(t *testing.T) {
	buf := NewTelemetryBuffer(5, 1024*1024)

	if buf.Len() != 0 {
		t.Errorf("Expected length 0, got %d", buf.Len())
	}
	if buf.Pop() != nil {
		t.Errorf("Expected pop from empty buffer to return nil")
	}

	sub1 := &model.TelemetrySubmission{NodeID: "node-1", Timestamp: time.Now()}
	sub2 := &model.TelemetrySubmission{NodeID: "node-2", Timestamp: time.Now()}

	dropped := buf.Push(sub1)
	if dropped {
		t.Errorf("Unexpected drop on first push")
	}
	dropped = buf.Push(sub2)
	if dropped {
		t.Errorf("Unexpected drop on second push")
	}

	if buf.Len() != 2 {
		t.Errorf("Expected length 2, got %d", buf.Len())
	}

	peeked := buf.Peek()
	if peeked == nil || peeked.NodeID != "node-1" {
		t.Errorf("Expected peek node-1, got %v", peeked)
	}
	if buf.Len() != 2 {
		t.Errorf("Peek should not remove item, length is %d", buf.Len())
	}

	popped := buf.Pop()
	if popped == nil || popped.NodeID != "node-1" {
		t.Errorf("Expected popped node-1, got %v", popped)
	}

	popped = buf.Pop()
	if popped == nil || popped.NodeID != "node-2" {
		t.Errorf("Expected popped node-2, got %v", popped)
	}

	if buf.Len() != 0 {
		t.Errorf("Expected empty buffer, got %d", buf.Len())
	}
}

func TestTelemetryBuffer_CapacityEviction(t *testing.T) {
	capacity := 3
	buf := NewTelemetryBuffer(capacity, 1024*1024)

	for i := 1; i <= 5; i++ {
		sub := &model.TelemetrySubmission{
			NodeID:    "node-1",
			Timestamp: time.Now(),
		}
		dropped := buf.Push(sub)
		if i <= capacity && dropped {
			t.Errorf("Item %d should not have caused drop", i)
		}
		if i > capacity && !dropped {
			t.Errorf("Item %d should have caused drop", i)
		}
	}

	if buf.Len() != capacity {
		t.Errorf("Expected buffer length %d, got %d", capacity, buf.Len())
	}
	if buf.DroppedCount() != 2 {
		t.Errorf("Expected 2 dropped items, got %d", buf.DroppedCount())
	}
}

func TestTelemetryBuffer_MaxBytesEviction(t *testing.T) {
	// Small maxBytes limit
	buf := NewTelemetryBuffer(10, 300)

	sub := &model.TelemetrySubmission{
		NodeID:    "node-1",
		Timestamp: time.Now(),
		Metadata: map[string]string{
			"large_data": "some-value-string-with-enough-bytes-to-take-space",
		},
	}

	buf.Push(sub)
	buf.Push(sub)
	buf.Push(sub)

	if buf.Bytes() > 300 {
		t.Errorf("Buffer bytes %d exceeded limit 300", buf.Bytes())
	}
}

func TestTelemetryBuffer_PopBatch(t *testing.T) {
	buf := NewTelemetryBuffer(10, 1024*1024)

	for i := 1; i <= 5; i++ {
		buf.Push(&model.TelemetrySubmission{NodeID: "node-batch"})
	}

	batch := buf.PopBatch(3)
	if len(batch) != 3 {
		t.Fatalf("Expected batch of 3, got %d", len(batch))
	}
	if buf.Len() != 2 {
		t.Errorf("Expected remaining 2 items, got %d", buf.Len())
	}

	batch2 := buf.PopBatch(10)
	if len(batch2) != 2 {
		t.Fatalf("Expected batch of 2, got %d", len(batch2))
	}
	if buf.Len() != 0 {
		t.Errorf("Expected empty buffer, got %d", buf.Len())
	}

	emptyBatch := buf.PopBatch(5)
	if len(emptyBatch) != 0 {
		t.Errorf("Expected empty batch, got %d", len(emptyBatch))
	}
}

func TestTelemetryBuffer_Clear(t *testing.T) {
	buf := NewTelemetryBuffer(10, 1024*1024)
	buf.Push(&model.TelemetrySubmission{NodeID: "node-1"})
	buf.Push(&model.TelemetrySubmission{NodeID: "node-2"})

	buf.Clear()
	if buf.Len() != 0 {
		t.Errorf("Expected 0 items after clear, got %d", buf.Len())
	}
	if buf.Bytes() != 0 {
		t.Errorf("Expected 0 bytes after clear, got %d", buf.Bytes())
	}
}

func TestTelemetryBuffer_Concurrent(t *testing.T) {
	buf := NewTelemetryBuffer(50, 1024*1024)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				buf.Push(&model.TelemetrySubmission{NodeID: "node-concurrent"})
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = buf.Pop()
				_ = buf.Len()
				_ = buf.Bytes()
			}
		}()
	}

	wg.Wait()
}
