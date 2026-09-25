package fsm_test

import (
	"context"
	"testing"

	"github.com/cassianobraz/payment-processor/pkg/fsm"
)

type payload struct {
	visited []string
	score   float64
}

func record(p *payload, name string) {
	p.visited = append(p.visited, name)
}

func TestRunHappyPath(t *testing.T) {
	tests := []struct {
		name      string
		machine   *fsm.Machine[payload]
		wantHops  int
		wantScore float64
	}{
		{
			name: "start to score to done",
			machine: fsm.New[payload]("start").
				Handle("start", func(ctx context.Context, p *payload) (fsm.State, error) {
					record(p, "start")
					return "score", nil
				}).
				Handle("score", func(ctx context.Context, p *payload) (fsm.State, error) {
					record(p, "score")
					p.score = 0.9
					return "done", nil
				}).
				Terminal("done"),
			wantHops:  2,
			wantScore: 0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p payload
			trace, err := tt.machine.Run(context.Background(), &p)
			if err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
			if len(trace) != tt.wantHops {
				t.Errorf("len(trace) = %d, want %d", len(trace), tt.wantHops)
			}
			if p.score != tt.wantScore {
				t.Errorf("p.score = %v, want %v", p.score, tt.wantScore)
			}
		})
	}
}
