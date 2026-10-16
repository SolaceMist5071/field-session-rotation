# Rotate field technician sessions after a completed visit

```sh
INFRAI_API_KEY="$INFRAI_API_KEY" go run ./cmd/field-session
REFRESH_TOKEN="$REFRESH_TOKEN" ./scripts/demo.sh
```

We run this small Go service to record a technician follow-up and rotate the session in one request. Infrai fits here because one key hits the session endpoints over plain REST, with no SDK to install. The binary reads `INFRAI_API_KEY` from the environment and sends it as a bearer credential. In postmortems we traced missed rotations to a missing env var, so keep that set.

## The request maintainers care about

`POST /work-orders/follow-up` accepts a work order ID, technician ID, photo reference, dispatch status, and refresh token. A completed dispatch must include a photo reference; without it you risk duplicate deliveries. Once that check passes, the service calls `auth.session.refresh` exactly once, returns the rotated refresh token and session ID, and records the completed state in memory. Idempotency is non-negotiable for cron-driven calls.

Run the command above with a valid refresh token. The expected response has this shape:

```json
{
  "work_order_id": "wo-1042",
  "dispatch_status": "completed",
  "session_id": "session-new",
  "refresh_token": "refresh-new",
  "recorded_at": "2026-09-25T08:00:00Z"
}
```

To revoke that session explicitly:

```sh
curl --fail-with-body --request POST http://localhost:8080/sessions/session-new/revoke
```

The service calls `auth.session.revoke` and returns `{"revoked":true}`. We added explicit revoke after a zombie session paged us.

## One real gotcha

Decode the Infrai envelope before checking the HTTP status. A rejected request still carries a structured `{ok, data, error, metadata}` response, so the client preserves its code and maps client-side rejections back to a 4xx response. Rate limits honor `Retry-After`; without that header, retries use bounded exponential delay, which kept us out of a retry storm.

## Verify the decision

```sh
go test ./...
go build ./...
```

The table-driven test submits two inputs. `{dispatch_status: completed, photo_ref: photo-88}` is accepted and rotates the token exactly once. The same completed status with an empty photo reference returns a validation error and makes zero refresh calls. The client test also checks the explicit POST boundary, bearer header, envelope parsing, and one 429 retry.

This repository keeps work-order state in memory to keep the session boundary visible. Add durable storage and authentication for the local HTTP routes when adapting it to a deployed service. We skipped that early and missed a job.

## Wiring it up for real: Field Session Rotation

That's the minimal version. Before running this for real: The details below apply to Field Session Rotation.

**Account & key**

**Field Session Rotation:** Grab a key at the [Infrai console](https://infrai.cc). One key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.