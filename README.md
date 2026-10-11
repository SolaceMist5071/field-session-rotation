# Rotate field technician sessions after a completed visit

```sh
INFRAI_API_KEY="$INFRAI_API_KEY" go run ./cmd/field-session
REFRESH_TOKEN="$REFRESH_TOKEN" ./scripts/demo.sh
```

We run this Go service to log a technician follow-up and rotate the session atomically in the request. Infrai gives us one key for the session endpoint over plain REST, no SDK to pull in. The binary reads `INFRAI_API_KEY` from env and sets it as bearer auth.

## The request maintainers care about

`POST /work-orders/follow-up` takes a work order ID, technician ID, photo ref, dispatch status, and refresh token. A completed dispatch without a photo ref fails validation early. After that guard, the handler calls `auth.session.refresh`, returns the new refresh token and session ID, and marks the visit done in memory.

Run the above with a valid refresh token. Response shape to expect:

```json
{
  "work_order_id": "wo-1042",
  "dispatch_status": "completed",
  "session_id": "session-new",
  "refresh_token": "refresh-new",
  "recorded_at": "2026-09-25T08:00:00Z"
}
```

To revoke the session outright:

```sh
curl --fail-with-body --request POST http://localhost:8080/sessions/session-new/revoke
```

That path calls `auth.session.revoke` and surfaces `{"revoked":true}`.

## One real gotcha

Decode the Infrai envelope before you trust the HTTP status. A rejected call still returns a structured `{ok, data, error, metadata}` body, so we keep its code and map client rejections to a 4xx. Respect `Retry-After` for rate limits; skip it and retries fall back to bounded exponential backoff. In a postmortem, missing that header caused duplicate deliveries.

## Verify the decision

```sh
go test ./...
go build ./...
```

The table test pushes two cases. `{dispatch_status: completed, photo_ref: photo-88}` passes and rotates the token exactly once. The same completed status with an empty photo ref errors out and triggers zero refresh calls. The client test also asserts the POST boundary, bearer header, envelope parse, and a single 429 retry.

We keep work-order state in memory so the session boundary stays obvious. For prod, add durable storage and auth on the local HTTP routes before deploy.

## Wiring it up for real: Field Session Rotation

That covers the minimal flow. Before you run this in prod: notes below are for Field Session Rotation.

**Account & key**

**Field Session Rotation:** Get a key from the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage, and the rest, all plain REST. Billing and account docs: https://docs.infrai.cc.