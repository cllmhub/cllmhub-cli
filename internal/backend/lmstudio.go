package backend

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultLMStudioURL = "http://localhost:1234"

// LMStudio implements the Backend interface for LM Studio (OpenAI-compatible API)
type LMStudio struct {
	url    string
	model  string
	apiKey string
	client *http.Client
}

// NewLMStudio creates a new LM Studio backend
func NewLMStudio(cfg Config) (*LMStudio, error) {
	url := cfg.URL
	if url == "" {
		url = defaultLMStudioURL
	}

	if err := CheckInsecureAPIKey(url, cfg.APIKey); err != nil {
		return nil, err
	}

	return &LMStudio{
		url:    url,
		model:  cfg.Model,
		apiKey: cfg.APIKey,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}, nil
}

// Name returns the backend type
func (l *LMStudio) Name() string {
	return "lmstudio"
}

// URL returns the backend endpoint URL
func (l *LMStudio) URL() string {
	return l.url
}

// Complete sends a prompt and returns the full completion
func (l *LMStudio) Complete(ctx context.Context, req *Request) (*Response, error) {
	if len(req.Messages) > 0 {
		return l.completeChat(ctx, req)
	}

	oaiReq := openAIRequest{
		Model:       l.model,
		Prompt:      req.Prompt,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      false,
	}

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", l.url+"/v1/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if l.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+l.apiKey)
	}

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("lmstudio error (status %d): %s", resp.StatusCode, string(body))
	}

	var oaiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&oaiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	text := ""
	if len(oaiResp.Choices) > 0 {
		text = oaiResp.Choices[0].Text
	}

	return &Response{
		Text:             text,
		PromptTokens:     oaiResp.Usage.PromptTokens,
		CompletionTokens: oaiResp.Usage.CompletionTokens,
	}, nil
}

func (l *LMStudio) completeChat(ctx context.Context, req *Request) (*Response, error) {
	chatReq := openAIChatRequest{
		Model:       l.model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      false,
	}

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", l.url+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if l.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+l.apiKey)
	}

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("lmstudio error (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	text := ""
	if len(chatResp.Choices) > 0 {
		text = chatResp.Choices[0].Message.Content
	}

	return &Response{
		Text:             text,
		PromptTokens:     chatResp.Usage.PromptTokens,
		CompletionTokens: chatResp.Usage.CompletionTokens,
	}, nil
}

// Stream sends a prompt and streams tokens via the callback
func (l *LMStudio) Stream(ctx context.Context, req *Request, callback func(token string, done bool) error) (*Response, error) {
	if len(req.Messages) > 0 {
		return l.streamChat(ctx, req, callback)
	}

	oaiReq := openAIRequest{
		Model:       l.model,
		Prompt:      req.Prompt,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      true,
	}

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", l.url+"/v1/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if l.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+l.apiKey)
	}

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("lmstudio error (status %d): %s", resp.StatusCode, string(body))
	}

	var fullText string
	var promptTokens, completionTokens int

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			if err := callback("", true); err != nil {
				return nil, err
			}
			break
		}

		var oaiResp openAIResponse
		if err := json.Unmarshal([]byte(data), &oaiResp); err != nil {
			continue
		}

		if len(oaiResp.Choices) > 0 {
			token := oaiResp.Choices[0].Text
			fullText += token

			done := oaiResp.Choices[0].FinishReason != ""
			if err := callback(token, done); err != nil {
				return nil, err
			}

			if done {
				promptTokens = oaiResp.Usage.PromptTokens
				completionTokens = oaiResp.Usage.CompletionTokens
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading stream: %w", err)
	}

	return &Response{
		Text:             fullText,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	}, nil
}

func (l *LMStudio) streamChat(ctx context.Context, req *Request, callback func(token string, done bool) error) (*Response, error) {
	chatReq := openAIChatRequest{
		Model:       l.model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      true,
	}

	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", l.url+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if l.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+l.apiKey)
	}

	resp, err := l.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("lmstudio error (status %d): %s", resp.StatusCode, string(body))
	}

	var fullText string
	var promptTokens, completionTokens int

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			if err := callback("", true); err != nil {
				return nil, err
			}
			break
		}

		var chatResp openAIChatResponse
		if err := json.Unmarshal([]byte(data), &chatResp); err != nil {
			continue
		}

		if len(chatResp.Choices) > 0 {
			token := chatResp.Choices[0].Delta.Content
			fullText += token

			done := chatResp.Choices[0].FinishReason != ""
			if err := callback(token, done); err != nil {
				return nil, err
			}

			if done {
				promptTokens = chatResp.Usage.PromptTokens
				completionTokens = chatResp.Usage.CompletionTokens
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading stream: %w", err)
	}

	return &Response{
		Text:             fullText,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	}, nil
}

// ListModels returns all models available in LM Studio.
func (l *LMStudio) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", l.url+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	if l.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+l.apiKey)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lmstudio not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lmstudio returned status %d", resp.StatusCode)
	}

	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse lmstudio models: %w", err)
	}

	var models []string
	for _, m := range modelsResp.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

// lmstudioModelInfo is an entry from LM Studio's extended REST API
// (/api/v0/models). Unlike the OpenAI-compatible /v1/models, this endpoint
// exposes quantization, architecture, and context-length fields.
type lmstudioModelInfo struct {
	ID                  string `json:"id"`
	Type                string `json:"type"`
	Publisher           string `json:"publisher"`
	Arch                string `json:"arch"`
	CompatibilityType   string `json:"compatibility_type"`
	Quantization        string `json:"quantization"`
	State               string `json:"state"`
	MaxContextLength    int    `json:"max_context_length"`
	LoadedContextLength int    `json:"loaded_context_length"`
}

// lmstudioExtendedModels queries LM Studio's /api/v0/models endpoint. It
// returns nil when the endpoint is unavailable (older LM Studio builds).
func (l *LMStudio) lmstudioExtendedModels(ctx context.Context) ([]lmstudioModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", l.url+"/api/v0/models", nil)
	if err != nil {
		return nil, err
	}
	if l.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+l.apiKey)
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lmstudio /api/v0/models returned status %d", resp.StatusCode)
	}
	var body struct {
		Data []lmstudioModelInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.Data, nil
}

// ModelInfo returns provenance metadata from LM Studio. Uses the extended
// /api/v0/models endpoint when available to report quantization, architecture,
// and the runtime context length; falls back to /v1/models for the ID only.
func (l *LMStudio) ModelInfo(ctx context.Context) (*ModelIdentity, error) {
	identity := &ModelIdentity{Engine: "lmstudio"}

	if extended, err := l.lmstudioExtendedModels(ctx); err == nil && len(extended) > 0 {
		var match *lmstudioModelInfo
		for i := range extended {
			if extended[i].ID == l.model {
				match = &extended[i]
				break
			}
		}
		if match == nil {
			match = &extended[0]
		}
		identity.Source = match.ID
		identity.Family = match.Arch
		identity.Quantization = match.Quantization
		if match.CompatibilityType != "" {
			identity.Format = match.CompatibilityType
		} else {
			identity.Format = "gguf"
		}
		// Prefer the loaded context length (reflects the runtime ceiling the
		// user configured in LM Studio); fall back to the architectural max.
		if match.LoadedContextLength > 0 {
			identity.ContextLength = match.LoadedContextLength
		} else if match.MaxContextLength > 0 {
			identity.ContextLength = match.MaxContextLength
		}
		identity.Digest = lmstudioDigest(match.ID)
		return identity, nil
	}

	identity.Format = "gguf"
	if models, err := l.ListModels(ctx); err == nil && len(models) > 0 {
		source := models[0]
		for _, m := range models {
			if m == l.model {
				source = m
				break
			}
		}
		identity.Source = source
		identity.Digest = lmstudioDigest(source)
	} else {
		identity.Source = l.model
	}

	return identity, nil
}

// lmstudioDigest attempts to produce an integrity digest for an LM Studio
// model ID. LM Studio stores models under ~/.cache/lm-studio/models/<id>;
// when that path is a directory (publisher/repo layout) we hash the first
// .gguf file inside it.
func lmstudioDigest(modelID string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(home, ".cache", "lm-studio", "models", modelID)
	if digest := hashFile(path); digest != "" {
		return digest
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return ""
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".gguf") {
			continue
		}
		if digest := hashFile(filepath.Join(path, e.Name())); digest != "" {
			return digest
		}
	}
	return ""
}

// Health checks if LM Studio is available
func (l *LMStudio) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", l.url+"/v1/models", nil)
	if err != nil {
		return err
	}
	if l.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+l.apiKey)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return fmt.Errorf("lmstudio not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("lmstudio returned status %d", resp.StatusCode)
	}

	return nil
}
