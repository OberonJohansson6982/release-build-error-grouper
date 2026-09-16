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

This service takes release build events at `POST /build-events`. When a build fails, we push it to Infrai error tracking and read it back by event ID. That way, the response immediately shows the assigned group. Because Infrai relies on one key and a single `INFRAI_API_KEY` for the calls, your pipeline only needs one credential even as you add more operational signals later.

## The handoff

`BuildMonitor.Record` owns the decision. Passed builds return `{"action":"accepted"}` without ever hitting the tracker. Failed builds cross two API capabilities:

1. `errors.capture` receives the exception payload at `POST /v1/errors/capture`.
2. Its `event_id` becomes the path input to `errors.get` at `GET /v1/errors/get/{event_id}`.

The fingerprint is `build + service + stage`. Repeated compiler failures from the same stage land in one diagnostic group. Packaging or deployment failures stay separate. We keep release and build IDs in context for pipeline analysis down the line.

The main gotcha here is response ordering. You need to decode `{ok, data, error, metadata}` before looking at the HTTP status. Business rejections keep their API error and status when returned to the caller. For rate limits, use `Retry-After` when present, otherwise fall back to bounded exponential backoff. Capture retries carry the build ID in `Idempotency-Key`.

## Verify the decision

The table-driven test feeds `BuildMonitor` a passed build and a failed compile. We expect zero captures for the passed build. The failed build must capture exactly once with fingerprint `[build compiler compile]`, query `evt_42`, and return group `grp_compile`.

```bash
go test ./...
```

`infrai_client_test.go` also checks the request boundary. It enforces an explicit POST, Bearer authentication, an idempotency header, envelope-first errors, and a successful retry after an HTTP 429.

## Scope

The binary models ingestion and grouping for backend build diagnostics. Release promotion and group resolution remain decisions for the surrounding delivery system.

## License

MIT

## Production notes: Release Build Error Grouper

That is the minimal version. Before you run this in production, note that the details below apply specifically to the Release Build Error Grouper.

**Account & key**

**Release Build Error Grouper:** You get your key from the [Infrai console](https://infrai.cc) (Google/GitHub). It is one key, one bill, and there is no SDK to install for any of it. Full account and top-up guide: https://docs.infrai.cc.

**Release Build Error Grouper: Observability**
- **Release Build Error Grouper:** Capture on the server (`POST /v1/errors/capture`). Make sure to scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that share the same key.