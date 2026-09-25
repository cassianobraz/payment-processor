package fsm_test

import (
	"context"
	"errors"
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

func TestFallbackKeepsPipelineAvailable(t *testing.T) {
	dependencyDown := errors.New("upstream timeout")

	tests := []struct {
		name            string
		machine         *fsm.Machine[payload]
		wantLastVisited string
		wantErrAtHop    int
	}{
		{
			name: "ml down falls back to rules_only",
			machine: fsm.New[payload]("start").
				Handle("start", func(ctx context.Context, p *payload) (fsm.State, error) {
					record(p, "start")
					return "ml", nil
				}).
				Handle("ml", func(ctx context.Context, p *payload) (fsm.State, error) {
					return "", dependencyDown
				}).
				FallbackTo("ml", "rules_only").
				Handle("rules_only", func(ctx context.Context, p *payload) (fsm.State, error) {
					record(p, "rules_only")
					return "done", nil
				}).
				Terminal("done"),
			wantLastVisited: "rules_only",
			wantErrAtHop:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p payload
			trace, err := tt.machine.Run(context.Background(), &p)
			if err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
			if len(p.visited) == 0 || p.visited[len(p.visited)-1] != tt.wantLastVisited {
				t.Errorf("p.visited = %v, want last item %q", p.visited, tt.wantLastVisited)
			}
			if len(trace) <= tt.wantErrAtHop {
				t.Fatalf("len(trace) = %d, want > %d", len(trace), tt.wantErrAtHop)
			}
			if trace[tt.wantErrAtHop].Err == nil {
				t.Errorf("trace[%d].Err = nil, want original error preserved", tt.wantErrAtHop)
			}
		})
	}
}

func TestErrorWithoutFallbackAborts(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name    string
		machine *fsm.Machine[payload]
		wantErr error
	}{
		{
			name: "start fails with no fallback",
			machine: fsm.New[payload]("start").
				Handle("start", func(ctx context.Context, p *payload) (fsm.State, error) {
					return "", boom
				}).
				Terminal("done"),
			wantErr: boom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p payload
			_, err := tt.machine.Run(context.Background(), &p)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Run() error = %v, want errors.Is(err, %v)", err, tt.wantErr)
			}
		})
	}
}
