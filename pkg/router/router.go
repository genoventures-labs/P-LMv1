package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

// Router handles model selection based on prompt complexity.
type Router struct {
	Models []string
}

// NewRouter initializes a Router with models from recommended_models.json.
func NewRouter(recommendedModelsFile string) (*Router, error) {
	data, err := os.ReadFile(recommendedModelsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read recommended models: %w", err)
	}

	var models []string
	if err := json.Unmarshal(data, &models); err != nil {
		return nil, fmt.Errorf("failed to unmarshal recommended models: %w", err)
	}

	return &Router{Models: models}, nil
}

// SelectModel chooses the best model for the given prompt.
func (r *Router) SelectModel(prompt string) string {
	if len(r.Models) == 0 {
		return ""
	}

	promptLower := strings.ToLower(prompt)

	// Hard tasks keywords
	hardKeywords := []string{"code", "python", "script", "function", "json", "format", "analyze", "calculate", "solve", "complex"}

	// Tool usage keywords
	toolKeywords := []string{"search", "weather", "latest", "news", "fetch", "url", "http"}

	isHard := false
	for _, kw := range hardKeywords {
		if strings.Contains(promptLower, kw) {
			isHard = true
			break
		}
	}

	needsTools := false
	for _, kw := range toolKeywords {
		if strings.Contains(promptLower, kw) {
			needsTools = true
			break
		}
	}

	// Heuristics
	if isHard || needsTools || len(prompt) > 250 {
		// Use the 2nd best model (usually more capable than the absolute fastest 1b model)
		// Or 3rd if available. Let's say index 1 or 2.
		if len(r.Models) > 1 {
			return r.Models[1] // qwen2.5:3b-instruct in our case
		}
	}

	// Default to the fastest model
	return r.Models[0] // llama3.2:1b in our case
}

const (
	defaultOrchestrationBaseURL = "http://85.31.233.157:8002/api/v1"
	defaultResolveTimeout       = 6 * time.Second
)

// ResolveRequest carries remote orchestration routing hints.
type ResolveRequest struct {
	Query        string   `json:"query,omitempty"`
	Stage        string   `json:"stage,omitempty"`
	MaxLatencyMS int      `json:"max_latency_ms,omitempty"`
	Models       []string `json:"models,omitempty"`
}

// ResolveModel attempts orchestration routing and falls back to local selection.
func (r *Router) ResolveModel(req ResolveRequest) string {
	if model, err := ResolveRemote(req); err == nil && strings.TrimSpace(model) != "" {
		return model
	}
	return r.SelectModel(req.Query)
}

// ResolveRemote calls the orchestration router endpoint.
func ResolveRemote(req ResolveRequest) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("ORCHESTRATION_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = defaultOrchestrationBaseURL
	}

	fullURL := baseURL + path.Clean("/orchestration/router/resolve")
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal resolve request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, fullURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create resolve request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey := strings.TrimSpace(os.Getenv("REGISTRY_READ_KEY")); apiKey != "" {
		httpReq.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{Timeout: defaultResolveTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request resolve endpoint: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("resolve endpoint status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode resolve response: %w", err)
	}
	for _, key := range []string{"model", "model_name", "model_identifier", "resolved_model"} {
		if v, ok := payload[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), nil
		}
	}
	if route, ok := payload["route"].(map[string]interface{}); ok {
		for _, key := range []string{"model", "model_name", "model_identifier", "resolved_model"} {
			if v, ok := route[key].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v), nil
			}
		}
	}
	return "", fmt.Errorf("resolve response did not include model field")
}
