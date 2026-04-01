package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"syscall"
	"time"
)

// Backend defines the interface for LLM inference backends
type Backend interface {
	// Name returns the backend type name
	Name() string

	// URL returns the backend endpoint URL
	URL() string

	// Complete sends a prompt and returns the full completion
	Complete(ctx context.Context, req *Request) (*Response, error)

	// Stream sends a prompt and streams tokens via the callback
	Stream(ctx context.Context, req *Request, callback func(token string, done bool) error) (*Response, error)

	// Health checks if the backend is available
	Health(ctx context.Context) error

	// ListModels returns the models available on the backend.
	// Returns nil, nil if the backend does not support listing.
	ListModels(ctx context.Context) ([]string, error)

	// ConcurrentSlots queries the backend for the number of concurrent
	// inference slots it can handle. Returns 0 if the backend does not
	// expose this information.
	ConcurrentSlots(ctx context.Context) (int, error)
}

// Request represents an inference request to a backend
type Request struct {
	Prompt      string
	Messages    json.RawMessage // original chat messages with multimodal content parts
	MaxTokens   int
	Temperature float64
	TopP        float64
}

// Response represents an inference response from a backend
type Response struct {
	Text             string
	PromptTokens     int
	CompletionTokens int
}

// Config holds backend configuration
type Config struct {
	Type   string // "ollama", "llamacpp", "vllm", "lmstudio", "mlx"
	URL    string
	Model  string
	APIKey string // for backends that need auth
}

// CheckInsecureAPIKey returns an error if an API key is being sent over
// plain HTTP to a non-localhost host.
func CheckInsecureAPIKey(rawURL, apiKey string) error {
	if apiKey == "" {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	if u.Scheme != "http" {
		return nil
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("refusing to send API key over plain HTTP to remote host %q; use HTTPS or remove the API key", host)
}

// IsConnectionError returns true if the error indicates the model server
// is unreachable (connection refused, timeout, no route, etc.).
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	return false
}

// ProbeConcurrentSlots discovers the number of concurrent slots a backend
// supports by sending parallel 1-token requests and measuring wall-clock time.
//
// It first sends a single request to establish baseline latency, then sends
// batches of 2, 4, 8, … concurrent requests. As long as the batch completes
// within 1.5× the baseline, the requests ran in parallel. Once the batch time
// exceeds that threshold the previous batch size is returned. Caps at 10.
//
// Returns 0 if the baseline request fails (backend may be down or model not loaded).
func ProbeConcurrentSlots(ctx context.Context, b Backend) (int, error) {
	probe := &Request{
		Prompt:    "hi",
		MaxTokens: 1,
	}

	// Baseline: single request latency.
	start := timeNow()
	if _, err := b.Complete(ctx, probe); err != nil {
		return 0, nil
	}
	baseline := timeSince(start)

	// Threshold: if a batch takes longer than this, requests were queued.
	threshold := baseline * 3 / 2 // 1.5× baseline

	lastGood := 1
	for n := 2; n <= 10; n *= 2 {
		batchCtx, cancel := context.WithTimeout(ctx, baseline*time.Duration(n+1))

		errs := make(chan error, n)
		for i := 0; i < n; i++ {
			go func() {
				_, err := b.Complete(batchCtx, probe)
				errs <- err
			}()
		}

		start := timeNow()
		failed := false
		for i := 0; i < n; i++ {
			if err := <-errs; err != nil {
				failed = true
			}
		}
		elapsed := timeSince(start)
		cancel()

		if failed || elapsed > threshold {
			break
		}
		lastGood = n
	}

	return lastGood, nil
}

// timeNow and timeSince are variables so tests can override them.
var (
	timeNow   = time.Now
	timeSince = time.Since
)

// openAIChatRequest is the OpenAI-compatible chat completions request format.
// Used by vLLM, llama.cpp, LM Studio, and MLX when messages are present.
type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    json.RawMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
	Stream      bool            `json:"stream"`
}

// openAIChatResponse is the OpenAI-compatible chat completions response format.
type openAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// New creates a backend based on the config type
func New(cfg Config) (Backend, error) {
	switch cfg.Type {
	case "ollama":
		return NewOllama(cfg)
	case "llamacpp", "llama.cpp":
		return NewLlamaCpp(cfg)
	case "vllm":
		return NewVLLM(cfg)
	case "lmstudio":
		return NewLMStudio(cfg)
	case "mlx":
		return NewMLX(cfg)
	default:
		return nil, fmt.Errorf("unknown backend type: %s", cfg.Type)
	}
}
