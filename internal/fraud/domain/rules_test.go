package domain_test

import (
	"testing"

	"github.com/cassianobraz/payment-processor/internal/fraud/domain"
)

func TestApplyFastRules(t *testing.T) {
	passed := domain.RuleResult{Verdict: domain.VerdictApprove, Reason: "fast_rules_passed", Final: false}
	blocked := domain.RuleResult{Verdict: domain.VerdictDeny, Reason: "card_blocklisted", Final: true}
	overAmount := domain.RuleResult{Verdict: domain.VerdictManualReview, Reason: "amount_above_hard_limit", Final: true}
	overVelocity := domain.RuleResult{Verdict: domain.VerdictDeny, Reason: "velocity_limit_exceeded", Final: true}

	tests := []struct {
		name    string
		attempt domain.Attempt
		want    domain.RuleResult
	}{
		{"common attempt", domain.Attempt{CardFingerPrint: "fp_ok", AmountCents: 15_000, VelocityLastMin: 1}, passed},

		{"blocklisted card", domain.Attempt{CardFingerPrint: "fp_stolen_card_001", AmountCents: 15_000}, blocked},
		{"unknown card", domain.Attempt{CardFingerPrint: "fp_stolen_card_003", AmountCents: 15_000}, passed},

		{"amount just below limit", domain.Attempt{AmountCents: domain.MaxAmountCents - 1}, passed},
		{"amount at limit", domain.Attempt{AmountCents: domain.MaxAmountCents}, passed},
		{"amount just above limit", domain.Attempt{AmountCents: domain.MaxAmountCents + 1}, overAmount},

		{"velocity just below limit", domain.Attempt{VelocityLastMin: domain.VelocityPerMinLimit - 1}, passed},
		{"velocity at limit", domain.Attempt{VelocityLastMin: domain.VelocityPerMinLimit}, passed},
		{"velocity just above limit", domain.Attempt{VelocityLastMin: domain.VelocityPerMinLimit + 1}, overVelocity},

		{"blocklist wins over amount", domain.Attempt{CardFingerPrint: "fp_stolen_card_002", AmountCents: domain.MaxAmountCents + 1}, blocked},
		{"amount wins over velocity", domain.Attempt{AmountCents: domain.MaxAmountCents + 1, VelocityLastMin: domain.VelocityPerMinLimit + 1}, overAmount},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domain.ApplyFastRules(tt.attempt); got != tt.want {
				t.Errorf("ApplyFastRules() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
