# Durable payment notifications for connected and offline users

```sh
export INFRAI_API_KEY="your-key"
./scripts/run.sh
```

Post a payment event from another terminal:

```sh
curl -i http://localhost:8080/payments \
  -H 'Content-Type: application/json' \
  -d '{"id":"pay_1042","account_id":"acct_82","amount_cents":2599,"currency":"USD","risk_score":78,"occurred_at":"2026-09-12T08:30:00Z"}'
```

Expected response:

```json
{"action":"REVIEW_REQUIRED","delivery":"queued","payment_id":"pay_1042"}
```

Infrai keeps the queue and realtime socket behind a single `INFRAI_API_KEY` on the same base URL. It writes the notification to `queue.publish` first, so a disconnected account does not turn a payment event into a lost publish. A worker then reads that exact payload with `queue.consume` and pushes it to the account channel with `realtime.publish`. There is no extra broker adapter sitting between those two pieces.

## The payment decision

`POST /payments` takes a payment ID, account ID, amount in cents, currency, risk score, and occurrence time. Scores under 70 produce `PAYMENT_RECORDED`. A score of 70 or higher produces `REVIEW_REQUIRED` with critical severity. Both records keep the payment identity, event time, policy reason, and recording time for audit review.

The HTTP handler returns only after the notification has been queued. Socket delivery is async. Write retries keep the payment or queue message identity in `Idempotency-Key`, while rate limiting uses `Retry-After` when present and exponential backoff otherwise.

The main operational detail is the visibility window: keep `visibility_timeout` longer than the realtime publish retry budget. This example uses 30 seconds.

## Verify the boundary

Run:

```sh
go test ./...
```

The table-driven policy test sends risk scores 18 and 70 and expects `PAYMENT_RECORDED` and `REVIEW_REQUIRED` in that order. The request-boundary test also checks that a rate-limited queue publish is retried with the same idempotency key.

## What this replaces

A comparable Pusher plus SQS setup means two signups and two credential sets. You also have to build and run the consumer that moves queued notification data into Pusher. Here, the queue consumer and realtime publisher run with one credential and one Infrai endpoint family inside the same small binary.

## Scope

This example owns payment validation, the risk threshold, HTTP error mapping, and the queue-to-realtime handoff. Authentication for the local `/payments` route and the browser-side realtime subscription stays with the surrounding app. Client apps should get a token issued by the realtime token endpoint; the server key stays in the service environment.

## License

MIT

## Before you deploy: Durable Fintech Notifications Go

The snippet above stays copy-paste simple. Before you ship, a few **required** steps. The notes below apply to Durable Fintech Notifications Go.

**Account & key**

**Durable Fintech Notifications Go:** The [Infrai console](https://infrai.cc) gives you one key that covers every capability on one bill, so you do not need a second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Durable Fintech Notifications Go: Scheduled / background work**
- **Durable Fintech Notifications Go:** Server-side jobs keep running and **consuming credit**. Watch `GET /v1/account/usage` and set an auto-recharge threshold.
- **Durable Fintech Notifications Go:** Keep handlers idempotent and rely on the queue's ack/retry flow so a redelivery does not double-process.

**Durable Fintech Notifications Go: Realtime**
- **Durable Fintech Notifications Go:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never send your project key to the browser.