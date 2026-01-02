# Changelog

## v0.5.38
- Introduce lifecycle management with contexts for graceful shutdown of workers, tickers, and the HTTP server with a central cancellation point.
- Centralize dependencies into a shared container struct and inject interfaces to improve testability.
- Add interfaces for put.io and Arr clients with mocked variants in tests to cover orchestration and HTTP handlers.
- Validate configuration strictly by checking download directory existence/permissions and enforcing sane bounds on polling intervals and worker counts.
- Remove double JSON marshal/unmarshal in HTTP handlers by binding Transmission `Arguments` into typed structs.

## v0.5.37
- Share a single put.io client across components to reduce duplicate transports and memory footprint.
- Prebuild and reuse Arr clients in the download manager instead of constructing them per check.
- Adjust HTTP server/handler and download manager constructors (and tests) to accept injected shared clients.
- Bump version constant to 0.5.37.