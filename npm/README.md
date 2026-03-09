# cllmhub

Turn your local LLM into a production API.

## Install

```bash
npm install -g cllmhub
```

Or run directly without installing:

```bash
npx cllmhub --help
```

## Usage

```bash
# Authenticate with your cLLMHub account
cllmhub login

# Publish a model (interactive selection from local backends)
cllmhub publish

# Publish a specific model
cllmhub publish -m llama3 -b ollama

# Check who you're logged in as
cllmhub whoami

# Update to the latest version
cllmhub update
```

## Supported backends

- **Ollama** (default) — http://localhost:11434
- **vLLM** — http://localhost:8000
- **llama.cpp** — http://localhost:8080
- **Custom** — any OpenAI-compatible HTTP server

## Features

- OAuth 2.0 device flow authentication
- Interactive model selection with arrow keys
- Auto-reconnect on WebSocket disconnect
- Model server health monitoring
- Rate limiting and concurrency control
- Request audit logging

For more information, visit [github.com/cllmhub/cllmhub-cli](https://github.com/cllmhub/cllmhub-cli).
