package service

import (
	"testing"

	"github.com/farrasnazhif/ripple-be/internal/dto/request"
)

func TestPreviewImpact(t *testing.T) {
	base := request.ImpactPreview{
		Points: []request.ImpactPoint{
			{Key: "logo", Value: "visible", Scope: "local", SourceQuote: "logo visible"},
			{Key: "time", Value: "day", Scope: "global", SourceQuote: "day run"},
		},
		Shots: []request.ImpactShot{
			{ID: "one", Links: []request.ImpactLink{{PointKey: "logo", Confirmed: true}}},
			{ID: "two", Links: []request.ImpactLink{{PointKey: "logo", Confirmed: false}}},
			{ID: "three", Exclusions: []string{"time"}},
		},
		Diffs: []request.ImpactDiff{{PointKey: "logo", Old: "visible", New: "final only", Confirmed: true}},
	}
	got, err := PreviewImpact(base)
	if err != nil {
		t.Fatal(err)
	}
	if got.Shots[0].Status != "Needs update" || got.Shots[0].Reasons[0].Kind != "direct" || got.Shots[1].Status != "Unchanged" {
		t.Fatalf("local impact or unconfirmed link: %+v", got.Shots)
	}
	base.Diffs = append(base.Diffs, request.ImpactDiff{PointKey: "time", Old: "day", New: "night", Confirmed: true})
	got, err = PreviewImpact(base)
	if err != nil {
		t.Fatal(err)
	}
	if got.Shots[1].Status != "Needs update" || got.Shots[1].Reasons[0].Kind != "global" || got.Shots[2].Status != "Unchanged" {
		t.Fatalf("global impact or exclusion: %+v", got.Shots)
	}
	base.Diffs[1].Confirmed = false
	got, err = PreviewImpact(base)
	if err != nil {
		t.Fatal(err)
	}
	if got.Shots[1].Status != "Ambiguous" || got.Shots[0].Status != "Needs update" {
		t.Fatalf("pending impact: %+v", got.Shots)
	}
	base.Diffs[0].Old = "wrong"
	if _, err := PreviewImpact(base); err == nil {
		t.Fatal("stale old value accepted")
	}
}
