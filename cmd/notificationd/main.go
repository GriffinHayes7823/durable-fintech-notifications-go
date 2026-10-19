package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/durable-fintech-notifications/notification"
)

type service struct {
	infrai *notification.Client
	now    func() time.Time
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	baseURL := os.Getenv("INFRAI_BASE_URL")
	if baseURL == "" {
		baseURL = notification.DefaultBaseURL
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	svc := &service{infrai: notification.NewClient(baseURL, key), now: time.Now}
	go svc.deliverLoop(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /payments", svc.acceptPayment)
	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("notification service listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func (s *service) acceptPayment(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var event notification.PaymentEvent
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payment event"})
		return
	}
	delivery, err := notification.Decide(event, s.now())
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	if err := s.infrai.Enqueue(r.Context(), delivery, "payment-"+event.ID); err != nil {
		writeClientError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{
		"payment_id": event.ID, "action": delivery.Data.Action, "delivery": "queued",
	})
}

func (s *service) deliverLoop(ctx context.Context) {
	for ctx.Err() == nil {
		messages, err := s.infrai.Consume(ctx, 10, 30)
		if err != nil {
			log.Printf("consume notification: %v", err)
			wait(ctx, time.Second)
			continue
		}
		if len(messages) == 0 {
			wait(ctx, 500*time.Millisecond)
			continue
		}
		for _, message := range messages {
			var delivery notification.Delivery
			if err := json.Unmarshal(message.Payload, &delivery); err != nil {
				log.Printf("decode queued notification %s: %v", message.MessageID, err)
				continue
			}
			if err := s.infrai.Publish(ctx, delivery, "delivery-"+message.MessageID); err != nil {
				log.Printf("publish queued notification %s: %v", message.MessageID, err)
			}
		}
	}
}

func writeClientError(w http.ResponseWriter, err error) {
	var apiErr *notification.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Code, "message": apiErr.Message})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "notification delivery unavailable"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func wait(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
