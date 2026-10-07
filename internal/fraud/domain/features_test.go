package domain_test

import (
	"math"
	"testing"
	"time"

	"github.com/cassianobraz/payment-processor/internal/fraud/domain"
)

func at(hour, min, sec int) time.Time {
	return time.Date(2026, 10, 7, hour, min, sec, 0, time.UTC)
}

func TestFeatureVectorLength(t *testing.T) {
	v := domain.FeatureVector(domain.Attempt{OccurredAt: at(12, 0, 0)})
	if len(v) != domain.FeatureCount {
		t.Errorf("len(FeatureVector()) = %d, want %d", len(v), domain.FeatureCount)
	}
}

func TestFeatureVector(t *testing.T) {
	tests := []struct {
		name    string
		attempt domain.Attempt
		feature int
		want    float64
	}{
		{"amount copied", domain.Attempt{AmountCents: 12_345, OccurredAt: at(14, 0, 0)}, domain.FeatureAmountCents, 12_345},
		{"velocity copied", domain.Attempt{VelocityLastMin: 3, OccurredAt: at(14, 0, 0)}, domain.FeatureVelocityLastMin, 3},

		{"midnight is night", domain.Attempt{OccurredAt: at(0, 0, 0)}, domain.FeatureNightTime, 1},
		{"05:59:59 is night", domain.Attempt{OccurredAt: at(5, 59, 59)}, domain.FeatureNightTime, 1},
		{"06:00:00 is not night", domain.Attempt{OccurredAt: at(6, 0, 0)}, domain.FeatureNightTime, 0},
		{"afternoon is not night", domain.Attempt{OccurredAt: at(14, 0, 0)}, domain.FeatureNightTime, 0},

		{"empty country is not foreign", domain.Attempt{CountryCode: "", OccurredAt: at(14, 0, 0)}, domain.FeatureForeignCountry, 0},
		{"BR is not foreign", domain.Attempt{CountryCode: "BR", OccurredAt: at(14, 0, 0)}, domain.FeatureForeignCountry, 0},
		{"US is foreign", domain.Attempt{CountryCode: "US", OccurredAt: at(14, 0, 0)}, domain.FeatureForeignCountry, 1},

		{"gambling is high risk", domain.Attempt{MerchantCategory: "gambling", OccurredAt: at(14, 0, 0)}, domain.FeatureHighRickMCC, 1},
		{"crypto is high risk", domain.Attempt{MerchantCategory: "crypto", OccurredAt: at(14, 0, 0)}, domain.FeatureHighRickMCC, 1},
		{"gift_card is high risk", domain.Attempt{MerchantCategory: "gift_card", OccurredAt: at(14, 0, 0)}, domain.FeatureHighRickMCC, 1},
		{"money_transfer is high risk", domain.Attempt{MerchantCategory: "money_transfer", OccurredAt: at(14, 0, 0)}, domain.FeatureHighRickMCC, 1},
		{"grocery is not high risk", domain.Attempt{MerchantCategory: "grocery", OccurredAt: at(14, 0, 0)}, domain.FeatureHighRickMCC, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := domain.FeatureVector(tt.attempt)
			if got := v[tt.feature]; got != tt.want {
				t.Errorf("FeatureVector()[%d] = %v, want %v", tt.feature, got, tt.want)
			}
		})
	}
}

func TestVerdictFromScore(t *testing.T) {
	tests := []struct {
		name  string
		score float64
		want  domain.Verdict
	}{
		{"low score", 0.1, domain.VerdictApprove},
		{"just below ApproveBelow", math.Nextafter(domain.ApproveBelow, 0), domain.VerdictApprove},
		{"exactly ApproveBelow", domain.ApproveBelow, domain.VerdictManualReview},
		{"just above ApproveBelow", math.Nextafter(domain.ApproveBelow, 1), domain.VerdictManualReview},
		{"middle of band", 0.5, domain.VerdictManualReview},
		{"just below DenyAbove", math.Nextafter(domain.DenyAbove, 0), domain.VerdictManualReview},
		{"exactly DenyAbove", domain.DenyAbove, domain.VerdictManualReview},
		{"just above DenyAbove", math.Nextafter(domain.DenyAbove, 1), domain.VerdictDeny},
		{"high score", 0.9, domain.VerdictDeny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domain.VerdictFromScore(tt.score); got != tt.want {
				t.Errorf("VerdictFromScore(%v) = %q, want %q", tt.score, got, tt.want)
			}
		})
	}
}
