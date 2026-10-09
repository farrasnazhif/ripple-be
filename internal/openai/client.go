package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

var ErrNotConfigured = errors.New("ChatGPT API is not configured")
var ErrEmptyBrief = errors.New("brief text is empty")

type Client struct {
	key     string
	model   string
	baseURL string
	http    *http.Client
}

func New(key, model string) *Client {
	return &Client{key: key, model: model, baseURL: "https://api.openai.com", http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) SummarizeBrief(ctx context.Context, rawText string) (string, entity.TokenUsage, error) {
	if c.key == "" || c.model == "" {
		return "", entity.TokenUsage{}, ErrNotConfigured
	}
	if strings.TrimSpace(rawText) == "" {
		return "", entity.TokenUsage{}, ErrEmptyBrief
	}
	body, err := json.Marshal(map[string]any{
		"model":        c.model,
		"store":        false,
		"instructions": "Summarize the creative brief concisely in the same language as the brief. Preserve concrete constraints and uncertainties. Do not invent details.",
		"input":        rawText,
	})
	if err != nil {
		return "", entity.TokenUsage{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/responses", bytes.NewReader(body))
	if err != nil {
		return "", entity.TokenUsage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", entity.TokenUsage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", entity.TokenUsage{}, fmt.Errorf("ChatGPT API returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		Status string `json:"status"`
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage entity.TokenUsage `json:"usage"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return "", entity.TokenUsage{}, err
	}
	if result.Status != "completed" {
		return "", result.Usage, fmt.Errorf("ChatGPT response status %q", result.Status)
	}
	for _, output := range result.Output {
		if output.Type != "message" {
			continue
		}
		for _, content := range output.Content {
			if content.Type == "output_text" && strings.TrimSpace(content.Text) != "" {
				return content.Text, result.Usage, nil
			}
		}
	}
	return "", result.Usage, fmt.Errorf("ChatGPT returned no summary text")
}
