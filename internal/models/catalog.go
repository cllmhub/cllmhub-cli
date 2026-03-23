package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const hfAPIBase = "https://huggingface.co/api"

// HFModel represents a model from the Hugging Face API.
type HFModel struct {
	ID        string   `json:"id"`        // e.g. "TheBloke/Mistral-7B-v0.1-GGUF"
	Tags      []string `json:"tags"`
	Downloads int      `json:"downloads"`
	Likes     int      `json:"likes"`
}

// HFFile represents a file in a HF model repo.
type HFFile struct {
	Filename string  `json:"rfilename"`
	Size     int64   `json:"size"`
	LFS      *HFLFS  `json:"lfs,omitempty"`
}

// HFLFS holds LFS metadata including the SHA256 hash.
type HFLFS struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// SearchHFModels searches Hugging Face for GGUF text-generation models matching the query.
func SearchHFModels(ctx context.Context, hfToken, query string) ([]HFModel, error) {
	params := url.Values{
		"search": {query},
		"filter": {"gguf,text-generation"},
		"sort":   {"downloads"},
		"limit":  {"20"},
	}

	reqURL := hfAPIBase + "/models?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+hfToken)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search Hugging Face: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid HF token — run 'cllmhub hf-token set <token>' with a valid token")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HF API returned status %d", resp.StatusCode)
	}

	var models []HFModel
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("invalid HF response: %w", err)
	}

	return models, nil
}

// ListRepoGGUFs lists all GGUF files in a Hugging Face repo.
func ListRepoGGUFs(ctx context.Context, hfToken, repoID string) ([]HFFile, error) {
	files, err := listRepoFiles(ctx, hfToken, repoID)
	if err != nil {
		return nil, err
	}

	var ggufFiles []HFFile
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f.Filename), ".gguf") {
			ggufFiles = append(ggufFiles, f)
		}
	}

	return ggufFiles, nil
}

func listRepoFiles(ctx context.Context, hfToken, repoID string) ([]HFFile, error) {
	reqURL := fmt.Sprintf("%s/models/%s", hfAPIBase, repoID)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+hfToken)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repo info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("repo %q not found on Hugging Face", repoID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HF API returned status %d", resp.StatusCode)
	}

	var repoInfo struct {
		Siblings []HFFile `json:"siblings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return nil, fmt.Errorf("invalid HF response: %w", err)
	}

	return repoInfo.Siblings, nil
}
