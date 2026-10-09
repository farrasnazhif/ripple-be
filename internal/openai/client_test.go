package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSummarizeBrief(t *testing.T) {
	c := New("test-key", "test-model")
	c.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("wrong OpenAI request")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["input"] != "Morning, running shoes" || body["store"] != false {
			t.Fatalf("bad body: %v, %v", body, err)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"Morning running shoe campaign."}]}],"usage":{"input_tokens":12,"output_tokens":8}}`))}, nil
	})}
	text, usage, err := c.SummarizeBrief(context.Background(), "Morning, running shoes")
	if err != nil || text != "Morning running shoe campaign." || usage.InputTokens != 12 {
		t.Fatalf("summary: %q, %+v, %v", text, usage, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
