// Package use case implements the fraud scoring pipeline as an
// explicit finite state machine Every attempt walks the same states:
// Lenrichfast rules ml scoringsemantic tiebreak finalize.The ml
// stage has a deterministic fallback so the pipeline never blocks the
// Iauthorization path when the model is unavailable,
package usecase

import (
	"context"
	"log/slog"

	"github.com/cassianobraz/payment-processor/internal/fraud/domain"
	"github.com/cassianobraz/payment-processor/pkg/batch"
	"github.com/cassianobraz/payment-processor/pkg/fsm"
	"github.com/cassianobraz/payment-processor/pkg/memcache"
)

// FSM states of the scoring pipeline.
const (
	StateEnrich    = "enrich"
	StateFastRules = "fast_rules"
	StateMLScore   = "ml_score"
	StateTiebreak  = "tiebreak"
	StateFinalize  = "finalize"
	StateDone      = "done"
)

// GeoResolver enriches the attempt with a country code
type GeoResolver interface {
	CountryCode(ctx context.Context, ip string) (string, error)
}

// Embedder converts text to a vector for the semantic cache
type Embedder interface {
	Embed(text string) []float64
}

// SemanticCache resolves ambiguous scores by similarity to recently
// decided attempts.
type SemanticCache interface {
	Lookup(vector []float64) (domain.Verdict, float64, bool)
	Store(vector []float64, verdict domain.Verdict)
}

// Decision is the pipeline output.
type Decision struct {
	Verdict   domain.Verdict
	RishScore float64
	Reasons   []string
	Degraded  bool
}

// scoringData is the mutable payload driven through the FSM
type scoringData struct {
	attempt domain.Attempt
	verdict domain.Verdict
	final   bool
	vector  []float64
}

// pipeline score payment attempts
type pipeline struct {
	machine  *fsm.Machine[scoringData]
	model    *domain.TreeModel
	geo      GeoResolver
	embedder Embedder
	semCache SemanticCache
	velocity *memcache.Cache[int]
	batcher  *batch.Batcher[[]float64, float64]
	logger   *slog.Logger
}
