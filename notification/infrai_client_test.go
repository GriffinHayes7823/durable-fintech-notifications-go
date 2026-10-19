package notification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestEnqueueRetriesRateLimitWithSameIdempotencyKey(t *testing.T) {
	calls := 0
	client := NewClient("https://example.test", "test-key")
	client.Sleep = func(context.Context, time.Duration) error { return nil }
	client.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != http.MethodPost || req.Header.Get("Idempotency-Key") != "payment-pay_1042" {
			t.Fatal("request boundary changed during retry")
		}
		var requestBody struct {
			Queue   string   `json:"queue"`
			Payload Delivery `json:"payload"`
		}
		if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		if requestBody.Queue != notificationQueue || requestBody.Payload.Channel != "account-acct_82" {
			t.Fatalf("unexpected publish body: %+v", requestBody)
		}
		status, body := http.StatusOK, `{"ok":true,"data":{},"error":null,"metadata":{}}`
		if calls == 1 {
			status, body = http.StatusTooManyRequests, `{"ok":false,"data":null,"error":{"code":"RATE_LIMITED","message":"retry later"},"metadata":{}}`
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})

	err := client.Enqueue(context.Background(), Delivery{Channel: "account-acct_82"}, "payment-pay_1042")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("got %d calls, want 2", calls)
	}
}
