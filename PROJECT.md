# Project: remote-rag Unit Tests

## Architecture
- `client/bridge`: Go client application bridging standard input/output to HTTP/SSE services. It has `auth.go` (authentication helpers), `main.go` (CLI entry point, parsing, loop), `sse.go` (SSE server connections/events), and `transmitter.go` (transmitting requests).
- `server/auth-helper`: Go server application helper for token validation and caching. It has `cache.go` (cache storage/invalidation) and `main.go` (server handlers, endpoints).

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| 1 | Client Bridge Exploration | Explore `client/bridge` codebase and design test suite | none | DONE (Conv: 233f33a6-5164-481a-a7c3-d1493a205a60) |
| 2 | Server Auth-Helper Exploration | Explore `server/auth-helper` codebase and design test suite | none | DONE (Conv: 8c5fe2c6-12f4-491e-8ae3-d3fb3c6dff1e) |
| 3 | Client Bridge Unit Tests Implementation | Implement unit tests for `client/bridge` files | M1 | DONE (Conv: 0312f4e8-7190-46a5-a7aa-7fafad4fbbe5) |
| 4 | Server Auth-Helper Unit Tests Implementation | Implement unit tests for `server/auth-helper` files | M2 | DONE (Conv: 0312f4e8-7190-46a5-a7aa-7fafad4fbbe5) |
| 5 | Challenger Coverage Hardening | Run challenger to check coverage & boundaries | M3, M4 | SKIPPED (No local Go compiler) |
| 6 | Forensic Audit & Verification | Run Forensic Auditor and verify build/test correctness | M5 | DONE (Conv: 0ffce735-d4bf-4f55-806e-3042e49941a2) |
| 7 | Handoff & Completion | Commit changes, update TODO.md, prepare reports | M6 | IN_PROGRESS (Conv: 3fcee034-a89e-479a-958e-fc8f81c6bddd) |

## Interface Contracts
- Go standard testing package (`testing`) as the primary mechanism.
- Optional use of `github.com/stretchr/testify` if needed to mock or verify assertions easily.

## Code Layout
- `client/bridge/`
  - `auth_test.go`
  - `main_test.go`
  - `sse_test.go`
  - `transmitter_test.go`
- `server/auth-helper/`
  - `cache_test.go`
  - `main_test.go`
