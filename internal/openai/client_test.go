package openai

import (
	"context"
	"encoding/json"
	"github.com/farrasnazhif/ripple-be/internal/entity"
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

func TestSummaryImageAndPDFInputs(t *testing.T) {
	c := New("test-key", "vision-model")
	c.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body struct {
			Input []struct {
				Content []map[string]any `json:"content"`
			} `json:"input"`
			Store bool `json:"store"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		parts := body.Input[0].Content
		if len(parts) != 5 || parts[2]["type"] != "input_image" || parts[2]["image_url"] != "https://r2.example/image" || parts[4]["type"] != "input_file" || parts[4]["file_url"] != "https://r2.example/pdf" || body.Store {
			t.Fatalf("bad attachment inputs: %+v", body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"Attachment summary"}]}]}`))}, nil
	})}
	_, _, err := c.SummarizeBrief(context.Background(), "", entity.BriefReference{URL: "https://r2.example/image", ContentType: "image/png", FileName: "image.png"}, entity.BriefReference{URL: "https://r2.example/pdf", ContentType: "application/pdf", FileName: "brief.pdf"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestReviewedPlanAndSummaryRefinement(t *testing.T) {
	c := New("test-key", "test-model")
	output := `[{"title":"Garden","prompt":"Warm green garden"}]`
	c.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["store"] != false || body["input"] == "" || body["instructions"] == "" {
			t.Fatal("missing reviewed text input", body)
		}
		payload, _ := json.Marshal(map[string]any{"status": "completed", "output": []any{map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": output}}}}})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(payload)))}, nil
	})}
	plan, err := c.DraftPlan(context.Background(), "Warm green campaign", "video")
	if err != nil || len(plan) != 1 || plan[0].Prompt != "Warm green garden" {
		t.Fatal(plan, err)
	}
	output = "Revised summary with evening light"
	text, err := c.RefineSummary(context.Background(), "Morning campaign", "Use evening light")
	if err != nil || text != output {
		t.Fatal(text, err)
	}
	if _, err = c.DraftPlan(context.Background(), "Campaign", "image"); err == nil {
		t.Fatal("non-JSON plan accepted")
	}
}
