package higgsfield

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	imageModel = "higgsfield-ai/soul/v2/standard"
	videoModel = "bytedance/seedance-2.0/text-to-video"
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var ErrSubmissionRejected = errors.New("Higgsfield rejected the submission")

var ErrNotConfigured = errors.New("Higgsfield API is not configured")

type Job struct {
	RequestID string   `json:"request_id"`
	Status    string   `json:"status"`
	Images    []string `json:"images,omitempty"`
	Video     string   `json:"video,omitempty"`
}

type Client struct {
	key     string
	baseURL string
	http    *http.Client
}

func New(key string) *Client {
	return &Client{key: key, baseURL: "https://api.higgsfield.ai", http: &http.Client{Timeout: 30 * time.Second}}
}

// Submit sends exactly one paid generation request. The caller must persist the returned ID.
func (c *Client) Submit(ctx context.Context, kind, prompt string) (Job, error) {
	if c.key == "" {
		return Job{}, ErrNotConfigured
	}
	if strings.TrimSpace(prompt) == "" {
		return Job{}, fmt.Errorf("prompt is empty")
	}
	var model string
	var input map[string]any
	switch kind {
	case "image":
		model = imageModel
		input = map[string]any{"prompt": prompt, "batch_size": 1, "resolution": "720p", "aspect_ratio": "16:9"}
	case "video":
		model = videoModel
		input = map[string]any{"prompt": prompt, "duration": 5, "resolution": "720p", "aspect_ratio": "16:9", "generate_audio": false}
	default:
		return Job{}, fmt.Errorf("unknown media kind %q", kind)
	}
	body, err := json.Marshal(input)
	if err != nil {
		return Job{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+model, bytes.NewReader(body))
	if err != nil {
		return Job{}, err
	}
	req.Header.Set("Authorization", "Key "+c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return Job{}, err // An ambiguous timeout must not be retried automatically.
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusRequestTimeout {
			return Job{}, fmt.Errorf("%w: HTTP %d", ErrSubmissionRejected, resp.StatusCode)
		}
		return Job{}, fmt.Errorf("Higgsfield API returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		RequestID string `json:"request_id"`
		Status    string `json:"status"`
		Images    []struct {
			URL string `json:"url"`
		} `json:"images"`
		Video struct {
			URL string `json:"url"`
		} `json:"video"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return Job{}, err
	}
	job := Job{RequestID: result.RequestID, Status: result.Status, Video: result.Video.URL}
	for _, image := range result.Images {
		job.Images = append(job.Images, image.URL)
	}
	if job.RequestID == "" {
		return Job{}, fmt.Errorf("Higgsfield returned no request ID")
	}
	return job, nil
}

func (c *Client) Status(ctx context.Context, requestID string) (Job, error) {
	if c.key == "" {
		return Job{}, ErrNotConfigured
	}
	if !requestIDPattern.MatchString(requestID) {
		return Job{}, fmt.Errorf("invalid request ID")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/requests/"+requestID+"/status", nil)
	if err != nil {
		return Job{}, err
	}
	req.Header.Set("Authorization", "Key "+c.key)
	resp, err := c.http.Do(req)
	if err != nil {
		return Job{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Job{}, fmt.Errorf("Higgsfield API returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		Status string `json:"status"`
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
		Video struct {
			URL string `json:"url"`
		} `json:"video"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return Job{}, err
	}
	job := Job{RequestID: requestID, Status: result.Status, Video: result.Video.URL}
	for _, image := range result.Images {
		job.Images = append(job.Images, image.URL)
	}
	return job, nil
}
