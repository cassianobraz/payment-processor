package batch_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cassianobraz/payment-processor/pkg/batch"
)

func TestSubmitAfterClose(t *testing.T) {
	echo := func(_ context.Context, items []int) ([]int, error) { return items, nil }
	b := batch.New(2, time.Millisecond, echo)
	b.Close()

	_, err := b.Submit(context.Background(), 1)
	if !errors.Is(err, batch.ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestBatchBySize(t *testing.T) {
	var calls atomic.Int32
	double := func(_ context.Context, items []int) ([]int, error) {
		calls.Add(1)
		out := make([]int, len(items))
		for i, v := range items {
			out[i] = v * 2
		}
		return out, nil
	}

	b := batch.New(4, time.Second, double)
	defer b.Close()

	results := make([]int, 8)
	var wg sync.WaitGroup

	for i := range 8 {
		wg.Go(func() {
			res, err := b.Submit(t.Context(), i)
			if err != nil {
				t.Errorf("Submit failed for %d: %v", i, err)
				return
			}
			results[i] = res
		})
	}

	wg.Wait()

	for i := range 8 {
		if results[i] != i*2 {
			t.Errorf("results[%d] = %d, expected %d", i, results[i], i*2)
		}
	}

	if count := calls.Load(); count > 4 {
		t.Errorf("expected at most 4 flusher calls, got %d", count)
	}
}

func TestFlusherError(t *testing.T) {
	errBoom := errors.New("flusher boom")
	failFlush := func(_ context.Context, items []int) ([]int, error) {
		return nil, errBoom
	}
	b := batch.New(2, time.Second, failFlush)
	defer b.Close()

	var wg sync.WaitGroup
	for i := range 2 {
		wg.Go(func() {
			_, err := b.Submit(t.Context(), i)
			if err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
	wg.Wait()
}

func TestFlusherErrorWithMatchingOuts(t *testing.T) {
	errBoom := errors.New("custom error")
	failFlush := func(_ context.Context, items []int) ([]int, error) {
		return make([]int, len(items)), errBoom
	}
	b := batch.New(1, time.Second, failFlush)
	defer b.Close()

	_, err := b.Submit(t.Context(), 42)
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected %v, got %v", errBoom, err)
	}
}

func TestCloseIdempotent(t *testing.T) {
	echo := func(_ context.Context, items []int) ([]int, error) { return items, nil }
	b := batch.New(2, time.Millisecond, echo)

	b.Close()
	b.Close()

	_, err := b.Submit(context.Background(), 1)
	if !errors.Is(err, batch.ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestFlushesByTimeWindow(t *testing.T) {
	echo := func(_ context.Context, items []string) ([]string, error) { return items, nil }

	// maxSize far above what we submit, so only the timer can close the batch.
	b := batch.New(100, 20*time.Millisecond, echo)
	defer b.Close()

	start := time.Now()
	res, err := b.Submit(t.Context(), "lonely")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if res != "lonely" {
		t.Errorf("result = %q, expected %q", res, "lonely")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("Submit took %v, expected the 20ms window to flush well under 500ms", elapsed)
	}
}

func TestContextCancelUnblocksCaller(t *testing.T) {
	slow := func(_ context.Context, items []int) ([]int, error) {
		time.Sleep(200 * time.Millisecond)
		return items, nil
	}
	b := batch.New(10, 50*time.Millisecond, slow)
	defer b.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := b.Submit(ctx, 1)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got: %v", err)
	}
}

func TestNewWithInvalidMaxSize(t *testing.T) {
	echo := func(_ context.Context, items []int) ([]int, error) { return items, nil }
	b := batch.New(0, time.Millisecond, echo)
	defer b.Close()

	res, err := b.Submit(t.Context(), 10)
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if res != 10 {
		t.Fatalf("expected 10, got %d", res)
	}
}

func TestTimerResetAfterExpiry(t *testing.T) {
	echo := func(_ context.Context, items []int) ([]int, error) { return items, nil }
	b := batch.New(10, 10*time.Millisecond, echo)
	defer b.Close()

	res1, err := b.Submit(t.Context(), 1)
	if err != nil || res1 != 1 {
		t.Fatalf("first submit failed: %v, %d", err, res1)
	}
	time.Sleep(30 * time.Millisecond)

	res2, err := b.Submit(t.Context(), 2)
	if err != nil || res2 != 2 {
		t.Fatalf("second submit failed: %v, %d", err, res2)
	}
}
