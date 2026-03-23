# Security Assessment — cllmhub-cli

**Date:** 2026-03-22
**Scope:** Full codebase review of cllmhub-cli (v0.5.0)
**Tool:** Static analysis, code review, dependency audit

---

## Executive Summary

cllmhub-cli is a Go CLI that downloads GGUF models from Hugging Face, runs a local inference daemon (llama-server), and publishes models to the cLLMHub network via persistent WebSocket connections. The v0.5.0 release adds significant new functionality: a daemon architecture with Unix socket IPC, model download/management, and Hugging Face integration. All previously identified security findings have been addressed.

**Risk Rating:** Low

---

## Architecture Overview

```
CLI Commands
    |
    v
Cobra Command Parser
    |
    ├─── Daemon Mode ──────────────────────────┐
    │    cllmhub start/stop/status             │
    │         |                                 │
    │         v                                 │
    │    Unix Socket (~/.cllmhub/cllmhub.sock) │
    │    (0600 perms + bearer token auth)       │
    │         |                                 │
    │         v                                 │
    │    Daemon Process (__daemon)              │
    │      ├── llama-server (engine)           │
    │      └── Bridge Manager (multi-model)    │
    │              |                            │
    │              v                            │
    │         Hub Gateway (WSS + cert pinning) │
    │                                           │
    └─── Foreground Mode ──────────────────────┘
         cllmhub publish -m/-b
              |              |
              v              v
         Local Backend    Hub Gateway
         (HTTP localhost) (WSS cllmhub.com)
```

---

## Findings

No open findings. All previously identified issues have been resolved — see below.

---

## Resolved Findings

#### Provider Token on CLI — RESOLVED

**Previous finding:** `--token` flag exposed credentials in shell history and process listings.
**Resolution:** The `--token` flag has been removed. Authentication now uses OAuth 2.0 device flow with credentials stored in `~/.cllmhub/credentials` (0600 permissions) and automatic background token refresh.

#### Binary Signature Verification on Self-Update — RESOLVED

**Previous finding:** The `update` command downloaded binaries without checksum verification.
**Resolution:** `verifyChecksum()` now downloads `checksums.txt` from the GitHub release and verifies the SHA-256 hash of the downloaded binary before replacing the current executable.

#### TLS Certificate Pinning for Hub Connection — RESOLVED

**Previous finding:** WebSocket connection to the hub used Go's default TLS verification with no certificate pinning.
**Resolution:** `hub/client.go` now supports TLS certificate pinning via `SetPinnedCertFingerprints()`. The WebSocket dialer uses a custom `TLSClientConfig` that verifies the server certificate's SHA-256 fingerprint against a set of pinned values.

#### Daemon Unix Socket Authentication — RESOLVED

**Previous finding:** The daemon HTTP API on the Unix socket had no authentication.
**Resolution:** The daemon generates a random 256-bit auth token on startup, writes it to `~/.cllmhub/daemon.token` (0600), and requires `Authorization: Bearer <token>` on all API requests (except `/api/health`). The client reads the token from disk. Token comparison uses `crypto/subtle.ConstantTimeCompare`.

#### Model Download SHA256 Verification — RESOLVED

**Previous finding:** SHA256 was computed during download but never verified against a trusted source.
**Resolution:** The `Download()` function now accepts an `expectedSHA256` parameter (from the HF API's `lfs.sha256` field). After download, the computed hash is compared against the expected value. On mismatch, the file is deleted and an error is returned.

#### Path Traversal in Model File Names — RESOLVED

**Previous finding:** Model file names were joined to the models directory without verifying the result stayed within bounds.
**Resolution:** A `safePath()` function validates that the resolved absolute path has the models directory as a prefix. Used in `Download()`, `DeleteModel()`, and `ModelFilePath()`.

#### Unsanitized Symlinks in Engine Binary Extraction — RESOLVED

**Previous finding:** Symlinks in tar archives were created without validating that targets stay within the extraction directory.
**Resolution:** `extractFromTarGz()` now resolves the symlink target to an absolute path and verifies it falls within `binDir` before creating the symlink. Escaping targets cause an error.

#### Unsanitized Error Messages in Daemon API Responses — RESOLVED

**Previous finding:** Internal error details (engine startup errors, auth failures) were embedded directly in daemon HTTP API JSON responses.
**Resolution:** Daemon handlers now return generic error messages in API responses and log the full error details to the daemon log file via `d.logger.Error()`.

#### Daemon PID File Race Conditions — RESOLVED

**Previous finding:** PID file management used standard file I/O with no advisory locking, creating TOCTOU gaps.
**Resolution:** The daemon now acquires an exclusive `flock` on the PID file at startup and holds it for the process lifetime. `IsRunning()` checks whether the lock is held rather than relying on signal probing. This also prevents multiple daemon instances from running simultaneously.

#### Backend API Key Over Plaintext HTTP — RESOLVED

**Previous finding:** API keys could be sent over plaintext HTTP to remote backends.
**Resolution:** `CheckInsecureAPIKey()` in `internal/backend/backend.go` refuses to initialize a backend if an API key is configured with a non-localhost HTTP URL.

#### Installation Script Modifies RC Files — RESOLVED

**Previous finding:** Install script modified shell RC files without confirmation.
**Resolution:** The script now prompts the user before each modification.

#### Temporary File Race During Update — RESOLVED

**Previous finding:** Temp file created with potential race condition.
**Resolution:** Uses `os.CreateTemp()` (0600 by default), verifies checksum before replacement, and uses atomic `os.Rename()`.

#### Audit Logging — RESOLVED

**Previous finding:** No logging of incoming requests or provider lifecycle events.
**Resolution:** Audit logging implemented via `--log-file` flag, outputs JSON lines with request metadata.

#### Rate Limiting — RESOLVED

**Previous finding:** No per-timewindow rate limiting on the provider side.
**Resolution:** Rate limiting implemented via `--rate-limit` flag (requests per minute).

#### Input Validation on Model Name — RESOLVED

**Previous finding:** No input sanitization on model names.
**Resolution:** Model names validated against `^[a-zA-Z0-9._:/-]+$` regex.

#### Verbose Backend Error Messages — RESOLVED

**Previous finding:** Backend HTTP error responses were forwarded to the hub, potentially exposing infrastructure details.
**Resolution:** `sanitizeError()` in `provider.go` logs the full error locally and returns `"internal backend error"` to the hub.

---

## Dependency Audit

| Dependency | Version | Known CVEs | Status |
|---|---|---|---|
| `github.com/gorilla/websocket` | v1.5.3 | None | OK |
| `github.com/spf13/cobra` | v1.8.0 | None | OK |
| `github.com/google/uuid` | v1.6.0 | None | OK |
| `github.com/spf13/pflag` | v1.0.5 | None | OK |
| `github.com/inconshreveable/mousetrap` | v1.1.0 | None | OK |

All dependencies are at current stable versions with no known vulnerabilities.

---

## Positive Security Observations

- **HTTPS/WSS by default** — Hub connection uses TLS out of the box
- **TLS certificate pinning** — Hub WebSocket supports pinned certificate fingerprints
- **Minimal attack surface** — Small dependency tree, no CGO, no database
- **Proper goroutine management** — Context cancellation and signal handling (SIGINT/SIGTERM)
- **Thread safety** — Mutex-protected concurrent request counting
- **Unique provider IDs** — UUID-based session identifiers prevent collisions
- **Automatic scheme upgrade** — HTTP hub URLs are converted to WSS
- **Audit logging** — Configurable JSON lines logging for request tracing
- **Rate limiting** — Per-minute rate limiting protects local backends from abuse
- **Download integrity** — SHA256 verification against HF API checksums
- **Path traversal protection** — Model file paths validated to stay within target directory
- **Symlink validation** — Tar extraction validates symlink targets stay within binDir
- **Restrictive file permissions** — Credentials, HF token, PID file, socket, and daemon token all use 0600/0700 permissions
- **Daemon authentication** — Auth token required for all daemon API operations
- **PID file locking** — Advisory `flock` prevents duplicate daemons and eliminates TOCTOU races
- **Graceful daemon shutdown** — Clean process lifecycle with PID file management
- **Secure self-update** — SHA-256 checksum verification before binary replacement
- **API key protection** — Refuses to send API keys over plaintext HTTP to remote hosts
- **Model name validation** — Regex allowlist prevents injection via model names
- **Error sanitization** — Generic errors to hub/API, detailed errors logged locally

---

## Methodology

- Manual source code review of all `.go` files
- Dependency version audit via `go.mod` / `go.sum`
- Data flow analysis from CLI input through WebSocket, HTTP backends, and daemon socket
- Review of build and release pipeline (GitHub Actions workflow)
- Review of installation script (`install.sh`)
- Review of daemon process management and IPC
- Review of model download and Hugging Face API integration
- Review of engine binary extraction and symlink handling
