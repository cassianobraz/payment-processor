// Package batch implements a dynamic batcher . Callers submit item
// individually, and the batcher groups then by size or time window,
// whichever happens first. This trades a small bounded latency for
// a large gain in throughput on the downstream call.
package batch

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrClosed = errors.New("batch: batcher is closed")

// Flusher processes a full batch. The implementation must be safe for
// concurrent calls because two windows can overlap under load.
type Flusher[In any, Out any] func(ctx context.Context, items []In) ([]Out, error)

type Batcher[In any, Out any] struct {
	maxSize int
	maxWait time.Duration
	flush   Flusher[In, Out]

	mu     sync.Mutex
	queue  chan request[In, Out]
	closed bool
	wg     sync.WaitGroup
}

type request[In any, Out any] struct {
	items In
	resp  chan result[Out]
}
type result[Out any] struct {
	err   error
	value Out
}

func New[In any, Out any](maxSize int, maxWait time.Duration, flush Flusher[In, Out]) *Batcher[In, Out] {
	if maxSize < 1 {
		maxSize = 1
	}
	b := &Batcher[In, Out]{
		maxSize: maxSize,
		maxWait: maxWait,
		flush:   flush,
		queue:   make(chan request[In, Out], maxSize),
	}
	b.wg.Add(1)
	go b.Loop()
	return b
}

func (b *Batcher[In, Out]) Submit(ctx context.Context, item In) (Out, error) {
	var zero Out

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return zero, ErrClosed

	}

	req := request[In, Out]{items: item, resp: make(chan result[Out])}
	b.queue <- req

	select {
	case res := <-req.resp:
		return res.value, res.err
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}
