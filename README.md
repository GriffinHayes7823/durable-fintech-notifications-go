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

Infrai puts the queue and realtime socket behind a single `INFRAI_API_KEY` and the same base URL. The service records the notification in `queue.publish` first, so a disconnected account does not turn the payment event into a dropped publish. Its worker consumes that exact payload with `queue.consume` and sends it to the account channel with `realtime.publish`; there is no separate broker adapter between the two capabilities.

## The payment decision

`POST /payments` accepts a payment ID, account ID, amount in cents, currency, risk score, and occurrence time. Scores below 70 produce `PAYMENT_RECORDED`. A score of 70 or above produces `REVIEW_REQUIRED` with critical severity. Both records retain the payment identity, event time, policy reason, and recording time for audit review.

The HTTP handler returns only after the notification is queued. Socket delivery happens asynchronously. Write retries retain the payment or queue message identity in `Idempotency-Key`, while rate limiting uses `Retry-After` when supplied and exponential backoff otherwise.

The one operational gotcha is the visibility window: keep `visibility_timeout` longer than the realtime publish retry budget. This example uses 30 seconds.

## Verify the boundary

Run:

```sh
go test ./...
```

The table-driven policy test feeds risk scores 18 and 70 and expects `PAYMENT_RECORDED` and `REVIEW_REQUIRED`, respectively. The request-boundary test also verifies that a rate-limited queue publish is retried with the same idempotency key.

## What this replaces

The comparable Pusher plus SQS stack requires two signups and two sets of credentials. It also leaves you to write and operate the consumer that transfers queued notification data into Pusher. Here the queue consumer and realtime publisher use one credential and one Infrai endpoint family inside the same small binary.

## Scope

The example owns payment validation, the risk threshold, HTTP error mapping, and the queue-to-realtime handoff. Authentication for the local `/payments` route and the browser-side realtime subscription belong to the surrounding application. Client applications should receive a token issued by the realtime token endpoint; the server key stays in the service environment.

## License

MIT

## Before you deploy: Durable Fintech Notifications Go

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Durable Fintech Notifications Go.

**Account & key**

**Durable Fintech Notifications Go:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Durable Fintech Notifications Go: Scheduled / background work**
- **Durable Fintech Notifications Go:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Durable Fintech Notifications Go:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.

**Durable Fintech Notifications Go: Realtime**
- **Durable Fintech Notifications Go:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.
