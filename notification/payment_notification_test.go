package notification

import (
	"testing"
	"time"
)

func TestDecidePaymentAction(t *testing.T) {
	occurred := time.Date(2026, 9, 12, 8, 30, 0, 0, time.UTC)
	recorded := occurred.Add(2 * time.Second)
	tests := []struct {
		name       string
		risk       int
		wantAction string
		wantLevel  string
	}{
		{name: "routine payment", risk: 18, wantAction: "PAYMENT_RECORDED", wantLevel: "info"},
		{name: "risk review", risk: 70, wantAction: "REVIEW_REQUIRED", wantLevel: "critical"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delivery, err := Decide(PaymentEvent{
				ID: "pay_1042", AccountID: "acct_82", AmountCents: 2599,
				Currency: "USD", RiskScore: tt.risk, OccurredAt: occurred,
			}, recorded)
			if err != nil {
				t.Fatal(err)
			}
			if delivery.Data.Action != tt.wantAction || delivery.Data.Severity != tt.wantLevel {
				t.Fatalf("got action=%s severity=%s", delivery.Data.Action, delivery.Data.Severity)
			}
			if delivery.Data.PaymentID != "pay_1042" || !delivery.Data.RecordedAt.Equal(recorded) {
				t.Fatal("audit identity or timestamp was not preserved")
			}
		})
	}
}
