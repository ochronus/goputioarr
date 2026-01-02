# Changelog

## v0.5.37
- Share a single put.io client across components to reduce duplicate transports and memory footprint.
- Prebuild and reuse Arr clients in the download manager instead of constructing them per check.
- Adjust HTTP server/handler and download manager constructors (and tests) to accept injected shared clients.
- Bump version constant to 0.5.37.