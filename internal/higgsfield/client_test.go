package higgsfield

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSubmitAndStatus(t *testing.T) {
	var paths []string
	c := New("test-key")
	c.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Key test-key" {
			t.Error("missing server-side credential")
		}
		paths = append(paths, r.URL.Path)
		if r.Method == http.MethodPost {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["prompt"] != "running shoes" {
				t.Errorf("bad submission body: %v, %v", body, err)
			}
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(`{"request_id":"req_123"}`))}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":"completed","images":[{"url":"https://example.com/a.png"}],"video":{"url":"https://example.com/a.mp4"}}`))}, nil
	})}
	for _, kind := range []string{"image", "video"} {
		job, err := c.Submit(context.Background(), kind, "running shoes")
		if err != nil || job.RequestID != "req_123" {
			t.Fatalf("submit %s: %+v, %v", kind, job, err)
		}
	}
	job, err := c.Status(context.Background(), "req_123")
	if err != nil || job.Status != "completed" || len(job.Images) != 1 || job.Video == "" {
		t.Fatalf("status: %+v, %v", job, err)
	}
	if paths[0] != "/"+imageModel || paths[1] != "/"+videoModel || paths[2] != "/requests/req_123/status" {
		t.Fatalf("wrong paths: %v", paths)
	}
	if _, err := c.Status(context.Background(), "../other"); err == nil {
		t.Fatal("accepted unsafe request ID")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
