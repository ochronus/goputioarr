# goputioarr Improvement Ideas

## Architecture & Code Quality
- [x] Introduce lifecycle management with contexts: graceful shutdown of workers, tickers, and HTTP server; central cancelation point.
- [x] Centralize dependencies (config, logger, put.io client, Arr clients) into a small container struct; inject interfaces to improve testability.
- [x] Add interfaces for put.io and Arr clients; mock them in tests to cover orchestration and HTTP handlers without network calls.
- [x] Validate configuration more strictly: check download directory existence/permissions; enforce sane bounds on polling interval and worker counts.
- [x] Remove double JSON marshal/unmarshal in HTTP handlers by binding `Arguments` into typed structs.

## Stability & Resilience
- Add retry with backoff for put.io and Arr calls; treat 5xx/429 specially and emit structured errors.
- Introduce stuck-transfer detection (e.g., long “DOWNLOADING” with no progress) with a configurable timeout.
- Add jitter to polling to avoid thundering herd when multiple instances run.
- Add a grace period before deleting local files after import to avoid races with Arr indexing.
- Cap recursion depth/target count when generating download targets; log and skip pathological trees.

## Performance & Memory
- Reuse a single put.io client and prebuilt Arr clients across components.
- Use a shared `http.Client` with timeouts and reuse for downloads instead of raw `http.Get`.
- Stream target generation to the download queue, or explicitly `nil` `Transfer.Targets` after use to free memory.
- Make Arr history page size configurable (lower defaults, e.g., 100–250) to trim allocations.
- Consider webhooks from put.io to reduce polling frequency; fall back to polling as backup.

## User Experience & Features
- Add CLI/HTTP status endpoints: `/health`, `/metrics` (Prometheus), and a “manual rescan” command.
- Improve logs with structured fields (transfer ID/hash, target path, service name); add request logging middleware.
- Enhance `generate-config`: non-interactive mode (env/flags), secure default permissions, and clearer output of config path.
- Optional TLS support (or clear reverse-proxy guidance) for the Transmission façade.
- Add a `version` built from ldflags instead of hardcoding in `main`.

## Security
- Write config with mode `0600`; document secret handling (API keys, passwords).
- Recommend TLS or a TLS-terminating proxy; avoid sending Basic Auth over plaintext.
- Sanitize logging of user-supplied names; never log tokens/credentials.

## Testing & CI
- Add integration-style tests for HTTP handlers using mocked put.io to validate Transmission RPC semantics (session headers, errors).
- Add manager state-transition tests with fake put.io/Arr clients covering download, import detection, seeding, and skip rules.
- Run `go vet`, `staticcheck`, and race detector in CI; add coverage thresholds.

## Operations & Packaging
- Provide systemd unit examples and container healthcheck hitting `/health`.
- Allow env overrides for bind address/port and config path for containerized deployments.
- Document resource tuning: worker counts, channel buffer sizes, `GOMEMLIMIT`, and `GOGC` for memory-constrained hosts.