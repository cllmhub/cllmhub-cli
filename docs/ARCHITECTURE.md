# cLLMHub CLI Architecture

## Overview

cLLMHub CLI is a Go command-line tool that downloads, runs, and publishes local LLM models to the cLLMHub hub network. It supports both a persistent daemon mode (running its own llama-server) and a foreground mode (connecting to external backends like Ollama, vLLM, or MLX). Built with Go 1.24 and the Cobra CLI framework.

## Project Structure

```
cllmhub-cli/
├── cmd/cllmhub/           # CLI commands (Cobra)
│   ├── main.go            # Root command, version check setup
│   ├── login.go           # OAuth device flow authentication
│   ├── publish.go         # Publish models (daemon or foreground mode)
│   ├── unpublish.go       # Stop serving published models
│   ├── start.go           # Start the daemon
│   ├── stop.go            # Stop the daemon
│   ├── status.go          # Show daemon status
│   ├── logs.go            # Show daemon logs
│   ├── daemon_cmd.go      # Internal daemon process entry point (hidden)
│   ├── download.go        # Download GGUF models from Hugging Face
│   ├── delete.go          # Delete downloaded models
│   ├── models_cmd.go      # List/search models
│   ├── hf_token.go        # Manage Hugging Face API token
│   ├── whoami.go          # Display current user
│   ├── logout.go          # Revoke credentials
│   └── update.go          # Self-update binary
│
├── internal/              # Core business logic
│   ├── auth/              # Credential storage & OAuth 2.0 device flow
│   ├── backend/           # LLM backend abstraction layer
│   ├── daemon/            # Daemon process management & HTTP API
│   ├── engine/            # llama-server config & hardware detection
│   ├── models/            # Model registry, HF API, download management
│   ├── paths/             # Centralized file path management
│   ├── provider/          # Provider lifecycle & request handling
│   ├── hub/               # WebSocket client for hub communication
│   ├── audit/             # JSON lines request audit logging
│   ├── tui/               # Interactive terminal UI (selection menus)
│   └── versioncheck/      # Background GitHub release polling
│
├── npm/                   # npm package wrapper & binary installer
├── Formula/               # Homebrew formula
├── .github/workflows/     # CI/CD (release automation)
├── Makefile               # Build & cross-compilation
└── install.sh             # Shell-based installer
```

## Core Modules

### Authentication (`internal/auth/`)

Implements OAuth 2.0 Device Authorization Grant (RFC 8628) for CLI-friendly login.

- **Credential storage**: `~/.cllmhub/credentials` (JSON, 0600 permissions)
- **TokenManager**: Background goroutine that auto-refreshes tokens 5 minutes before expiry. Exposes a `Dead()` channel to signal expired or revoked sessions.
- **Token revocation**: RFC 7009 compliant via `RevokeToken()`

### Backend Abstraction (`internal/backend/`)

All LLM backends implement a common `Backend` interface:

```go
type Backend interface {
    Complete(ctx, req) (*Response, error)
    Stream(ctx, req) (<-chan StreamEvent, error)
    Health(ctx) error
    ListModels(ctx) ([]Model, error)
}
```

Supported backends:

| Backend    | Default URL              | Protocol           |
|------------|--------------------------|---------------------|
| Ollama     | `localhost:11434`        | Ollama native API   |
| vLLM       | `localhost:8000`         | OpenAI-compatible   |
| LM Studio  | `localhost:1234`         | OpenAI-compatible   |
| Llama.cpp  | `localhost:8080`         | Llama.cpp HTTP      |
| MLX        | `localhost:8080`         | OpenAI-compatible   |
| Custom     | User-specified           | Simple JSON         |

A factory function `New()` instantiates the correct backend from a config type string.

### Daemon (`internal/daemon/`)

Manages a persistent background process that runs llama-server and handles model publishing.

- **Unix socket communication**: CLI commands talk to the daemon via `~/.cllmhub/cllmhub.sock`
- **PID file management**: `~/.cllmhub/daemon.pid` tracks the running process
- **Bridge manager**: Manages multiple simultaneous model publishing sessions
- **HTTP API endpoints**:
  - `GET /api/status` — daemon status and running models
  - `POST /api/publish` — publish a model
  - `POST /api/unpublish` — unpublish a model

The `__daemon` hidden command is the daemon's entry point, spawned by `cllmhub start`.

### Engine (`internal/engine/`)

Configures and manages the llama-server inference engine.

- **Hardware detection**: Identifies Apple Silicon (with flash attention), NVIDIA GPU, and CPU-only environments
- **Auto-sizing**: Calculates appropriate context size, slots, batch size, and GPU layers based on detected hardware
- **Configuration profiles**: Sensible defaults per hardware platform
- **CLI argument generation**: Converts engine config to llama-server command-line arguments

### Model Registry (`internal/models/`)

Manages downloaded GGUF models and Hugging Face integration.

- **Registry**: `~/.cllmhub/models/registry.json` — tracks all downloaded models with metadata (name, file, repo ID, size, SHA256, state, download time)
- **Alias generation**: Auto-assigns short aliases (e.g., `m1`, `m2`) for convenience
- **Hugging Face API**:
  - Search models (filters by GGUF + text-generation)
  - List repository files
  - Download with progress tracking and SHA256 verification
- **Legacy migration**: Automatically upgrades older registry formats

### Path Management (`internal/paths/`)

Centralized management of all cLLMHub file system paths:

| Path | Purpose |
|------|---------|
| `~/.cllmhub/` | Main state directory |
| `~/.cllmhub/models/` | Downloaded GGUF models |
| `~/.cllmhub/models/registry.json` | Model registry |
| `~/.cllmhub/logs/` | Daemon logs |
| `~/.cllmhub/bin/` | Binary storage |
| `~/.cllmhub/daemon.pid` | Daemon PID file |
| `~/.cllmhub/cllmhub.sock` | Unix socket for daemon communication |
| `~/.cllmhub/hf-token` | Hugging Face API token |
| `~/.cllmhub/credentials` | OAuth credentials |

### Provider Management (`internal/provider/`)

Manages the full lifecycle of a published model on the hub:

1. **Registration** — Connects via WebSocket, sends provider metadata
2. **Request handling** — Concurrent processing with configurable max concurrency and rate limiting (requests/minute)
3. **Health monitoring** — Periodic checks on the local model server (5 attempts, 60s intervals) with alert system for down/recovered events
4. **Reconnection** — Auto-reconnect loop (up to 5 attempts, 60s intervals) on connection loss
5. **Token refresh** — Includes fresh tokens in heartbeats to keep the session alive

Supports both daemon mode (publishing through the daemon's bridge manager) and foreground mode (directly connecting to an external backend).

### Hub Gateway Client (`internal/hub/`)

WebSocket-based communication with the cLLMHub gateway. Message types:

| Message         | Direction     | Purpose                          |
|-----------------|---------------|----------------------------------|
| `register`      | Client → Hub  | Provider registration            |
| `registered`    | Hub → Client  | Registration confirmation        |
| `heartbeat`     | Client → Hub  | Keep-alive with queue/GPU stats  |
| `request`       | Hub → Client  | Incoming inference request       |
| `response`      | Client → Hub  | Non-streaming completion         |
| `stream_token`  | Client → Hub  | Streaming token chunk            |
| `error`         | Client → Hub  | Error response                   |
| `ping`/`pong`   | Bidirectional | Connection health                |

### Audit Logging (`internal/audit/`)

Thread-safe JSON lines logger that records metadata for every request (timestamp, request ID, model, latency, token count, errors). Nil-safe — a nil logger is a valid no-op.

### Terminal UI (`internal/tui/`)

Interactive selection menus with vim/arrow key navigation and integer input prompts. Uses raw terminal mode with ANSI escape codes.

### Version Checking (`internal/versioncheck/`)

Non-blocking background check against the GitHub releases API with 24-hour caching (`~/.cllmhub/version-check.json`). Results display after command execution via Cobra's `PersistentPostRun` hook.

## Command Flow

```
cllmhub
  ├── login        OAuth device flow → discover local models → optionally publish
  ├── publish      Daemon mode: publish GGUF via daemon bridge
  │                Foreground mode: connect backend → register → handle requests
  ├── unpublish    Tell daemon to stop serving models
  ├── start        Spawn daemon process (llama-server + HTTP API)
  ├── stop         Send shutdown signal to daemon
  ├── status       Query daemon HTTP API for status
  ├── logs         Read/tail daemon log file
  ├── download     Fetch GGUF from Hugging Face → register in model registry
  ├── delete       Remove model files and registry entries
  ├── models       List local models or search Hugging Face
  ├── hf-token     set | remove | status — manage HF API token
  ├── whoami       Load credentials → display user info
  ├── logout       Revoke token → delete credentials file
  └── update       Check GitHub releases → download & replace binary
```

## Configuration

| Item               | Location                          | Format |
|--------------------|-----------------------------------|--------|
| Credentials        | `~/.cllmhub/credentials`         | JSON   |
| HF API token       | `~/.cllmhub/hf-token`            | Plain text |
| Model registry     | `~/.cllmhub/models/registry.json`| JSON   |
| Daemon PID         | `~/.cllmhub/daemon.pid`          | Plain text |
| Daemon socket      | `~/.cllmhub/cllmhub.sock`        | Unix socket |
| Daemon logs        | `~/.cllmhub/logs/daemon.log`     | Plain text |
| Version check cache| `~/.cllmhub/version-check.json`  | JSON   |
| Engine settings    | CLI flags on `start` command      | —      |
| Provider settings  | CLI flags on `publish` command    | —      |

## Distribution

- **GitHub Releases**: Cross-compiled binaries for darwin/linux (amd64/arm64) and windows (amd64)
- **npm**: `cllmhub` package with postinstall hook that downloads the platform binary
- **Homebrew**: `cllmhub/tap` formula with SHA256 verification
- **Shell script**: `install.sh` with platform detection and PATH configuration

## Key Dependencies

| Package                    | Purpose                |
|----------------------------|------------------------|
| `github.com/spf13/cobra`  | CLI framework          |
| `github.com/gorilla/websocket` | WebSocket client  |
| `github.com/google/uuid`  | Provider ID generation |
| `golang.org/x/time`       | Rate limiting          |

## Design Patterns

- **Interface-based backends**: Extensible backend system via the `Backend` interface and factory pattern
- **Daemon architecture**: Background process with Unix socket IPC, enabling multi-model publishing and persistent inference
- **Hardware auto-detection**: Engine config adapts to Apple Silicon, NVIDIA GPU, or CPU-only environments
- **Background token management**: Automatic refresh with dead-channel signaling for session invalidation
- **Context-based cancellation**: `context.Context` propagated throughout for clean shutdown on SIGINT/SIGTERM
- **Resilient reconnection**: Exponential backoff with health checks before re-registering
- **Concurrency control**: Semaphore-based max concurrency, mutex-protected shared state, channel-based synchronization
- **Model registry**: JSON-based local registry with auto-aliasing and SHA256 integrity verification
