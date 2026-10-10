package app_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/app"
	"github.com/setupforge/setupforge/internal/domain"
)

type mockUpdateChecker struct {
	checkCalls int32
	status     domain.UpdateStatus
	err        error
	delay      time.Duration
}

func (m *mockUpdateChecker) Check(ctx context.Context, opts domain.CheckOptions) (domain.UpdateStatus, error) {
	atomic.AddInt32(&m.checkCalls, 1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return domain.UpdateStatus{State: domain.UpdateStateFailed}, ctx.Err()
		}
	}
	if m.err != nil {
		return m.status, m.err
	}
	return m.status, nil
}

func TestUpdateService_GetUpdateStatus_Cached(t *testing.T) {
	mock := &mockUpdateChecker{
		status: domain.UpdateStatus{
			State:       domain.UpdateStateAvailable,
			UpdateCount: 3,
			LastChecked: time.Now(),
		},
	}

	svc := app.NewUpdateService(mock)

	// Verify no cached status initially
	if _, ok := svc.CachedStatus(); ok {
		t.Fatal("expected no cached status initially")
	}

	// First call executes check
	status1, err := svc.GetUpdateStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status1.UpdateCount != 3 {
		t.Errorf("expected 3 updates, got %d", status1.UpdateCount)
	}
	if atomic.LoadInt32(&mock.checkCalls) != 1 {
		t.Errorf("expected 1 call to checker, got %d", mock.checkCalls)
	}

	// Verify CachedStatus now returns data
	cached, ok := svc.CachedStatus()
	if !ok || cached.UpdateCount != 3 {
		t.Errorf("expected CachedStatus to return 3 updates, got %v (ok=%v)", cached, ok)
	}

	// Second call should return cached status without invoking checker again
	status2, err := svc.GetUpdateStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if status2.UpdateCount != 3 {
		t.Errorf("expected 3 updates on second call, got %d", status2.UpdateCount)
	}
	if atomic.LoadInt32(&mock.checkCalls) != 1 {
		t.Errorf("expected checker not called again, got %d calls", mock.checkCalls)
	}
}

func TestUpdateService_RefreshUpdateStatus(t *testing.T) {
	mock := &mockUpdateChecker{
		status: domain.UpdateStatus{
			State:       domain.UpdateStateAvailable,
			UpdateCount: 5,
		},
	}

	svc := app.NewUpdateService(mock)

	// First get
	_, err := svc.GetUpdateStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&mock.checkCalls) != 1 {
		t.Errorf("expected 1 call, got %d", mock.checkCalls)
	}

	// Update mock status to simulate system updated
	mock.status = domain.UpdateStatus{
		State:       domain.UpdateStateUpToDate,
		UpdateCount: 0,
	}

	// Call Refresh
	status, err := svc.RefreshUpdateStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error on refresh: %v", err)
	}
	if status.UpdateCount != 0 || status.State != domain.UpdateStateUpToDate {
		t.Errorf("expected up-to-date status after refresh, got %+v", status)
	}
	if atomic.LoadInt32(&mock.checkCalls) != 2 {
		t.Errorf("expected 2 calls after refresh, got %d", mock.checkCalls)
	}
}

func TestUpdateService_ErrorHandling(t *testing.T) {
	mock := &mockUpdateChecker{
		err: errors.New("network failure"),
	}

	svc := app.NewUpdateService(mock)
	_, err := svc.GetUpdateStatus(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Cache should not be populated on error
	if _, ok := svc.CachedStatus(); ok {
		t.Errorf("cache should not be populated on failure")
	}
}

func TestUpdateService_ConcurrentRequests(t *testing.T) {
	mock := &mockUpdateChecker{
		status: domain.UpdateStatus{
			State:       domain.UpdateStateAvailable,
			UpdateCount: 2,
		},
		delay: 20 * time.Millisecond,
	}

	svc := app.NewUpdateService(mock)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, err := svc.GetUpdateStatus(context.Background())
			if err != nil {
				t.Errorf("unexpected error in goroutine: %v", err)
			}
			if status.UpdateCount != 2 {
				t.Errorf("expected 2 updates, got %d", status.UpdateCount)
			}
		}()
	}
	wg.Wait()

	// Due to serialization and double-checked caching, checker should only be called once
	if calls := atomic.LoadInt32(&mock.checkCalls); calls != 1 {
		t.Errorf("expected exactly 1 call during concurrent access, got %d", calls)
	}
}
