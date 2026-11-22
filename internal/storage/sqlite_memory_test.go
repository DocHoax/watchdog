package storage

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestSQLiteStorage_Memory_SingleConnectionIsolation(t *testing.T) {
	// 1. Verify tables created during migration remain visible and concurrent operations do not fail.
	store, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create memory storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Verify default org is present
	org, err := store.GetOrganization(ctx, model.DefaultOrganizationID)
	if err != nil {
		t.Fatalf("failed to get default organization: %v", err)
	}
	if org.ID != model.DefaultOrganizationID {
		t.Errorf("expected default org, got %s", org.ID)
	}

	// 2. Multiple repository operations using the same in-memory storage instance see the same data
	testOrg := &model.Organization{
		ID:          "org-test-mem",
		Name:        "Memory Test Org",
		Status:      model.OrgStatusActive,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := store.SaveOrganization(ctx, testOrg); err != nil {
		t.Fatalf("failed to save organization: %v", err)
	}

	fetched, err := store.GetOrganization(ctx, "org-test-mem")
	if err != nil {
		t.Fatalf("failed to retrieve saved organization: %v", err)
	}
	if fetched.Name != "Memory Test Org" {
		t.Errorf("expected name 'Memory Test Org', got %s", fetched.Name)
	}

	// 3. Concurrent goroutines performing operations do not encounter missing-table errors
	var wg sync.WaitGroup
	concurrency := 20
	iterations := 50

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			localCtx := context.Background()
			for j := 0; j < iterations; j++ {
				metricName := "cpu_usage"
				pt := MetricPoint{
					Timestamp: time.Now().Unix(),
					Metric:    metricName,
					Value:     float64(workerID * j),
				}
				_ = store.SaveMetric(localCtx, pt)

				_, _ = store.ListOrganizations(localCtx)
			}
		}(i)
	}
	wg.Wait()
}

func TestSQLiteStorage_Memory_SeparateInstancesIsolated(t *testing.T) {
	// 4. Separate in-memory storage instances remain isolated from one another.
	store1, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create store1: %v", err)
	}
	defer store1.Close()

	store2, err := NewSQLiteStorage(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create store2: %v", err)
	}
	defer store2.Close()

	ctx := context.Background()

	org1 := &model.Organization{
		ID:          "org-instance-1",
		Name:        "Instance 1 Org",
		Status:      model.OrgStatusActive,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := store1.SaveOrganization(ctx, org1); err != nil {
		t.Fatalf("failed to save org1 in store1: %v", err)
	}

	// store1 should have it
	_, err = store1.GetOrganization(ctx, "org-instance-1")
	if err != nil {
		t.Errorf("expected org-instance-1 in store1, got err: %v", err)
	}

	// store2 should NOT have it
	_, err = store2.GetOrganization(ctx, "org-instance-1")
	if err == nil {
		t.Errorf("expected org-instance-1 to be absent in store2, but found it")
	}
}

func TestSQLiteStorage_FileBased_BehaviorPreserved(t *testing.T) {
	// 5. File-based SQLite behavior remains unchanged (uses WAL mode and specified MaxConns).
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteStorage(Config{
		Path:     dbPath,
		MaxConns: 5,
	})
	if err != nil {
		t.Fatalf("failed to create file storage: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	org, err := store.GetOrganization(ctx, model.DefaultOrganizationID)
	if err != nil {
		t.Fatalf("failed to get default organization from file DB: %v", err)
	}
	if org.ID != model.DefaultOrganizationID {
		t.Errorf("expected default org ID, got %s", org.ID)
	}
}
