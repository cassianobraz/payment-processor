package domain_test

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/cassianobraz/payment-processor/internal/fraud/domain"
)

func leaf(v float64) domain.Node {
	return domain.Node{Left: -1, Right: -1, Value: v}
}

func split(feature int, threshold float64, left, right int) domain.Node {
	return domain.Node{Feature: feature, Threshold: threshold, Left: left, Right: right}
}

// handModel has a depth-2 tree (amount, then velocity) plus a single-leaf tree.
func handModel() domain.TreeModel {
	return domain.TreeModel{
		Version:  "test",
		Features: 2,
		Trees: []domain.Tree{
			{Nodes: []domain.Node{
				split(0, 1000, 1, 2),
				leaf(-1),
				split(1, 5, 3, 4),
				leaf(0.5),
				leaf(2),
			}},
			{Nodes: []domain.Node{leaf(0.25)}},
		},
	}
}

func encode(m domain.TreeModel) []byte {
	b, _ := json.Marshal(m)
	return b
}

func want(total float64) float64 {
	return 1 / (1 + math.Exp(-total))
}

func TestParseTreeModelValid(t *testing.T) {
	data := []byte(`{
		"version": "v1",
		"features": 2,
		"baseline": -0.5,
		"trees": [{"nodes": [
			{"feature": 1, "threshold": 3.5, "left": 1, "right": 2, "value": 0},
			{"feature": 0, "threshold": 0, "left": -1, "right": -1, "value": -0.2},
			{"feature": 0, "threshold": 0, "left": -1, "right": -1, "value": 0.8}
		]}]
	}`)

	m, err := domain.ParseTreeModel(data)
	if err != nil {
		t.Fatalf("ParseTreeModel() error = %v, want nil", err)
	}
	if m.Version != "v1" || m.Features != 2 || m.Baseline != -0.5 || len(m.Trees) != 1 {
		t.Fatalf("ParseTreeModel() = %+v, header not decoded", m)
	}
	if got := m.Trees[0].Nodes[0]; got != split(1, 3.5, 1, 2) {
		t.Errorf("root node = %+v, want %+v", got, split(1, 3.5, 1, 2))
	}
}

func TestParseTreeModelBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(m *domain.TreeModel)
		wantErr bool
	}{
		{"hand model", func(m *domain.TreeModel) {}, false},

		{"features -1", func(m *domain.TreeModel) { m.Features = -1 }, true},
		{"features 0", func(m *domain.TreeModel) { m.Features = 0 }, true},
		{"features 1 with feature 1 split", func(m *domain.TreeModel) { m.Features = 1 }, true},
		{"features 1 with only feature 0", func(m *domain.TreeModel) {
			m.Features = 1
			m.Trees = []domain.Tree{{Nodes: []domain.Node{split(0, 1, 1, 2), leaf(0), leaf(1)}}}
		}, false},

		{"no trees", func(m *domain.TreeModel) { m.Trees = nil }, true},
		{"tree without nodes", func(m *domain.TreeModel) { m.Trees[1].Nodes = nil }, true},

		{"feature -1", func(m *domain.TreeModel) { m.Trees[0].Nodes[0].Feature = -1 }, true},
		{"feature 0", func(m *domain.TreeModel) { m.Trees[0].Nodes[0].Feature = 0 }, false},
		{"feature Features-1", func(m *domain.TreeModel) { m.Trees[0].Nodes[0].Feature = 1 }, false},
		{"feature Features", func(m *domain.TreeModel) { m.Trees[0].Nodes[0].Feature = 2 }, true},

		{"left -2 is not a leaf", func(m *domain.TreeModel) { m.Trees[0].Nodes[0].Left = -2 }, true},
		{"left -1 skips split checks", func(m *domain.TreeModel) { m.Trees[0].Nodes[1].Feature = 99 }, false},
		{"left 0 is a split and gets checked", func(m *domain.TreeModel) {
			m.Trees[0].Nodes[1] = split(99, 0, 0, 0)
		}, true},
		{"left len-1", func(m *domain.TreeModel) { m.Trees[0].Nodes[2].Left = 4 }, false},
		{"left len", func(m *domain.TreeModel) { m.Trees[0].Nodes[2].Left = 5 }, true},

		{"right -1 on split", func(m *domain.TreeModel) { m.Trees[0].Nodes[0].Right = -1 }, true},
		{"right 0", func(m *domain.TreeModel) { m.Trees[0].Nodes[2].Right = 0 }, false},
		{"right len-1", func(m *domain.TreeModel) { m.Trees[0].Nodes[2].Right = 4 }, false},
		{"right len", func(m *domain.TreeModel) { m.Trees[0].Nodes[2].Right = 5 }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := handModel()
			tt.mutate(&m)

			_, err := domain.ParseTreeModel(encode(m))
			if tt.wantErr && !errors.Is(err, domain.ErrModelInvalid) {
				t.Fatalf("ParseTreeModel() error = %v, want ErrModelInvalid", err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ParseTreeModel() error = %v, want nil", err)
			}
		})
	}
}

func TestParseTreeModelMalformedJSON(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"truncated", `{"features": 2, "trees": [`},
		{"not json", `model`},
		{"wrong type", `{"features": "two", "trees": []}`},
		{"empty input", ``},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := domain.ParseTreeModel([]byte(tt.data)); !errors.Is(err, domain.ErrModelInvalid) {
				t.Fatalf("ParseTreeModel() error = %v, want ErrModelInvalid", err)
			}
		})
	}
}

func TestScoreWalksTrees(t *testing.T) {
	tests := []struct {
		name      string
		features  []float64
		wantTotal float64
	}{
		{"amount below threshold goes left", []float64{999, 0}, -1 + 0.25},
		{"amount at threshold goes left", []float64{1000, 99}, -1 + 0.25},
		{"amount above threshold, velocity below", []float64{1001, 4}, 0.5 + 0.25},
		{"amount above threshold, velocity at threshold", []float64{1001, 5}, 0.5 + 0.25},
		{"amount above threshold, velocity above", []float64{1001, 6}, 2 + 0.25},
	}

	m := handModel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m.Score(tt.features)
			if err != nil {
				t.Fatalf("Score() error = %v, want nil", err)
			}
			if math.Abs(got-want(tt.wantTotal)) > 1e-12 {
				t.Errorf("Score(%v) = %v, want %v", tt.features, got, want(tt.wantTotal))
			}
		})
	}
}

func TestScoreBaseline(t *testing.T) {
	m := handModel()
	m.Baseline = -0.5

	got, err := m.Score([]float64{999, 0})
	if err != nil {
		t.Fatalf("Score() error = %v, want nil", err)
	}
	if w := want(-0.5 - 1 + 0.25); math.Abs(got-w) > 1e-12 {
		t.Errorf("Score() = %v, want %v", got, w)
	}
}

func TestScoreFeatureCountMismatch(t *testing.T) {
	tests := []struct {
		name     string
		features []float64
	}{
		{"one short", []float64{1}},
		{"one extra", []float64{1, 2, 3}},
		{"nil", nil},
	}

	m := handModel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := m.Score(tt.features); !errors.Is(err, domain.ErrModelInvalid) {
				t.Fatalf("Score() error = %v, want ErrModelInvalid", err)
			}
		})
	}
}

// TestScoreSigmoid drives sigmoid and expNeg through the baseline of a
// model whose only tree is a zero leaf, so the total equals the baseline.
func TestScoreSigmoid(t *testing.T) {
	tests := []struct {
		name  string
		total float64
		check func(t *testing.T, got float64)
	}{
		{"zero", 0, exactly(0.5)},
		{"just above zero", 1e-3, near(want(1e-3))},
		{"just below zero", -1e-3, near(want(-1e-3))},
		{"positive", 2, near(want(2))},
		{"negative", -2, near(want(-2))},
		{"needs halving", 12.7, near(want(12.7))},

		{"just below positive cutoff", 29.9, func(t *testing.T, got float64) {
			if got >= 1 || got < 1-1e-12 {
				t.Errorf("Score() = %v, want in [1-1e-12, 1)", got)
			}
		}},
		{"at positive cutoff", 30, func(t *testing.T, got float64) {
			if got >= 1 {
				t.Errorf("Score() = %v, want < 1 (cutoff is x > 30)", got)
			}
		}},
		{"just above positive cutoff", 30.1, exactly(1)},

		{"just above negative cutoff", -29.9, func(t *testing.T, got float64) {
			if got <= 0 || got > 1e-12 {
				t.Errorf("Score() = %v, want in (0, 1e-12]", got)
			}
		}},
		{"at negative cutoff", -30, func(t *testing.T, got float64) {
			if got <= 0 {
				t.Errorf("Score() = %v, want > 0 (cutoff is x > 30)", got)
			}
		}},
		{"just below negative cutoff", -30.1, exactly(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := domain.TreeModel{
				Features: 1,
				Baseline: tt.total,
				Trees:    []domain.Tree{{Nodes: []domain.Node{leaf(0)}}},
			}
			got, err := m.Score([]float64{0})
			if err != nil {
				t.Fatalf("Score() error = %v, want nil", err)
			}
			tt.check(t, got)
		})
	}
}

func exactly(w float64) func(*testing.T, float64) {
	return func(t *testing.T, got float64) {
		if got != w {
			t.Errorf("Score() = %v, want exactly %v", got, w)
		}
	}
}

func near(w float64) func(*testing.T, float64) {
	return func(t *testing.T, got float64) {
		if math.Abs(got-w) > 1e-9*math.Max(1, math.Abs(w)) {
			t.Errorf("Score() = %v, want %v", got, w)
		}
	}
}
