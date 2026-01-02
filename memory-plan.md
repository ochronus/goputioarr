# Memory Optimization Plan for goputioarr

This plan lists pragmatic changes to shrink RSS on memory-constrained hosts, from quickest wins to deeper refactors.

## Configuration Tweaks (no code changes)
1. Reduce worker counts:
   - `orchestration_workers`: set to 1–2.
   - `download_workers`: set to 1–2.
   - Rationale: each worker is a goroutine (~2–4 KB stacks that grow). Fewer workers lower baseline memory.
2. Lower channel buffers:
   - `transferChan` / `downloadChan`: set buffer to match total workers (e.g., 2–4) instead of 100 to avoid queueing many objects.
3. Raise polling interval if acceptable:
   - Increase `polling_interval` to reduce ticker wakeups and transient allocations.
4. Lower log verbosity:
   - Default `loglevel` to `warn` (or `error`) in config to reduce log allocation volume.

## Code Changes (prioritized)
1. Reuse clients (stop per-component allocations):
   - Create one `putio.Client` in `main` and pass it to download manager and HTTP handler.
   - Pre-create one Arr client per configured service and reuse (avoid building per `isImported` call).
2. Avoid whole-tree target accumulation:
   - Stream targets from `recurseDownloadTargets` into the download channel directly, or
   - If keeping slices, nil `Transfer.Targets` once not needed (after import/seeding) to let GC reclaim.
3. Clear large slices after use:
   - After `watchSeeding` completes (or already-imported paths), call `transfer.SetTargets(nil)`.
4. Shrink Arr history fetch size:
   - Reduce page size from 1000 to ~100–250 to trim JSON allocation per poll.
5. Remove double JSON marshal/unmarshal in handlers:
   - Bind `req.Arguments` directly into typed structs for `torrent-add` and `torrent-remove`; avoid marshal-then-unmarshal.
6. Use lighter server/logging stack (optional but impactful):
   - Replace Gin with `net/http` handlers and Logrus with stdlib logger or `zerolog` for lower allocations/background goroutines.

## Download/Worker Path Refinements
- Replace per-target `doneChan` allocations with a `sync.WaitGroup` or a small fixed channel pool to reduce channel churn.
- Keep `fetchFile` streaming (already uses `io.Copy`); avoid any `io.ReadAll` patterns.

## Caching/Sharing Opportunities
- If both HTTP handlers and the manager need transfer lists in the same interval, share the fetched list to avoid duplicate allocations (naturally improves after client reuse).

## Runtime/Operational Settings
- Set `GOMEMLIMIT` to the host budget (e.g., `GOMEMLIMIT=128MiB`) to cap heap growth.
- Lower `GOGC` (e.g., `GOGC=50`) to trigger earlier GC at the cost of some CPU—useful on memory-bound hosts.

## Suggested Implementation Order
1) Share a single `putio.Client`; prebuild Arr clients.  
2) Lower worker counts and channel buffer sizes; reduce default log level.  
3) Nil out `Transfer.Targets` after import/seeding.  
4) Reduce Arr history page size.  
5) Remove double JSON marshal/unmarshal.  
6) (Optional) Swap Gin/Logrus for stdlib/zerolog.  
7) (Optional) Stream target generation instead of building full slices.