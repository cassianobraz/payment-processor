// Package domain defines the  fraud scoring model: input features,
// rules outcomes, and the final decision contract.
package domain

import "time"

// Attempt is the payment attempt as seen by the frad pipeline
type Attempt struct {
	PaymentID       string
	AccountID       string
	CardFingerPrint string
	AmountCents     int64
	Currency        string
	ClientIP        string
	MerchantID      string
	OccurredAt      time.Time

	// Enriched during the pipeline run
	CountryCode     string
	VelocityLastMin int
	RiskScore       float64
	Reasons         []string
	Degraded        bool
}
