package notification

import (
	"errors"
	"fmt"
	"time"
)

type PaymentEvent struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"account_id"`
	AmountCents int64     `json:"amount_cents"`
	Currency    string    `json:"currency"`
	RiskScore   int       `json:"risk_score"`
	OccurredAt  time.Time `json:"occurred_at"`
}

type AuditNotification struct {
	PaymentID   string    `json:"payment_id"`
	AccountID   string    `json:"account_id"`
	AmountCents int64     `json:"amount_cents"`
	Currency    string    `json:"currency"`
	Action      string    `json:"action"`
	Severity    string    `json:"severity"`
	Reason      string    `json:"reason"`
	OccurredAt  time.Time `json:"occurred_at"`
	RecordedAt  time.Time `json:"recorded_at"`
}

type Delivery struct {
	Channel   string            `json:"channel"`
	Event     string            `json:"event"`
	AccountID string            `json:"account_id"`
	Data      AuditNotification `json:"data"`
}

func Decide(event PaymentEvent, recordedAt time.Time) (Delivery, error) {
	if event.ID == "" || event.AccountID == "" || event.Currency == "" || event.OccurredAt.IsZero() {
		return Delivery{}, errors.New("id, account_id, currency, and occurred_at are required")
	}
	if event.AmountCents <= 0 || event.RiskScore < 0 || event.RiskScore > 100 {
		return Delivery{}, errors.New("amount_cents and risk_score are out of range")
	}

	action, severity, reason := "PAYMENT_RECORDED", "info", "payment accepted by policy"
	if event.RiskScore >= 70 {
		action, severity, reason = "REVIEW_REQUIRED", "critical", "risk score requires manual review"
	}
	n := AuditNotification{
		PaymentID: event.ID, AccountID: event.AccountID, AmountCents: event.AmountCents,
		Currency: event.Currency, Action: action, Severity: severity, Reason: reason,
		OccurredAt: event.OccurredAt.UTC(), RecordedAt: recordedAt.UTC(),
	}
	return Delivery{
		Channel: fmt.Sprintf("account-%s", event.AccountID),
		Event:   "payment.notification", AccountID: event.AccountID, Data: n,
	}, nil
}
