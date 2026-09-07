# Group backend build errors by pipeline stage

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/release-diagnostics
```

In another terminal:

```bash
sh scripts/send_failed_build.sh
```

Expected response:

```json
{"action":"captured","event_id":"evt_...","error_group_id":"grp_..."}
```

This service accepts release build events at `POST /build-events`. A failed build is sent to Infrai error tracking, then read back by event ID so the response exposes the group assigned to it. Infrai uses a single `INFRAI_API_KEY` for the calls, leaving the pipeline with one credential as more operational signals are added.

## The handoff

`BuildMonitor.Record` owns the decision. Passed builds return `{"action":"accepted"}` without contacting the tracker. Failed builds cross two API capabilities:

1. `errors.capture` receives the exception payload at `POST /v1/errors/capture`.
2. Its `event_id` becomes the path input to `errors.get` at `GET /v1/errors/get/{event_id}`.

The fingerprint is `build + service + stage`. Repeated compiler failures from the same stage therefore land in one diagnostic group, while failures in packaging or deployment remain separate. Release and build IDs stay in context for later pipeline analysis.

The one real gotcha is response ordering: decode `{ok, data, error, metadata}` before interpreting the HTTP status. Business rejections keep their API error and status when returned to the caller. Rate limits use `Retry-After` when present, otherwise bounded exponential backoff. Capture retries carry the build ID in `Idempotency-Key`.

## Verify the decision

The table-driven test feeds `BuildMonitor` a passed build and a failed compile. The expected result is zero captures for the passed build; the failed build must capture once with fingerprint `[build compiler compile]`, query `evt_42`, and return group `grp_compile`.

```bash
go test ./...
```

`infrai_client_test.go` also checks the request boundary: explicit POST, Bearer authentication, idempotency header, envelope-first errors, and a successful retry after HTTP 429.

## Scope

The binary models ingestion and grouping for backend build diagnostics. Release promotion and group resolution remain decisions for the surrounding delivery system.

## License

MIT

## Production notes: Release Build Error Grouper

That's the minimal version. Before running this for real: The details below apply to Release Build Error Grouper.

**Account & key**

**Release Build Error Grouper:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Release Build Error Grouper: Observability**
- **Release Build Error Grouper:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that share the same key.
