package service

import (
	"github.com/farrasnazhif/ripple-be/internal/entity"
	"testing"
)

func TestChangeImpactSelectiveGlobalDeletedAndAmbiguous(t *testing.T) {
	points := []entity.BriefPoint{{Key: "light", Value: "Morning", Scope: "local"}, {Key: "palette", Value: "Green", Scope: "global"}}
	shots := []entity.StoryboardShot{{ID: "one", RequirementKeys: []string{"light"}}, {ID: "two", RequirementKeys: []string{}}}
	changed := append([]entity.BriefPoint{}, points...)
	changed[0].Value = "Evening"
	result := CalculateChange(points, changed, shots, true)
	if result.Impacts[0].Status != "needs_review" || result.Impacts[1].Status != "unchanged" || len(result.Diffs) != 1 {
		t.Fatal(result)
	}
	changed[1].Value = "Blue"
	result = CalculateChange(points, changed, shots, false)
	if result.Impacts[1].Status != "needs_review" {
		t.Fatal("global ignored", result)
	}
	result = CalculateChange(points, points[1:], shots, false)
	if result.Impacts[0].Status != "needs_review" || result.Impacts[1].Status != "unchanged" {
		t.Fatal("deleted requirement lost its links", result)
	}
	result = CalculateChange(points, points, shots, true)
	if result.Impacts[0].Status != "needs_review" || result.Impacts[1].Status != "needs_review" {
		t.Fatal("unmapped narrative changes incorrectly ignored", result)
	}
	state := &entity.ProjectReview{Changes: []entity.ProjectChange{{BriefVersion: 2, Impacts: []entity.ShotReview{{ShotID: "one", Status: "needs_review"}}}}}
	if !unresolvedImpact(state, "one", 1, "old") || unresolvedImpact(state, "two", 1, "old") {
		t.Fatal("review scope")
	}
	id := "old"
	state.Decisions = []entity.ShotDecision{{ShotID: "one", JobID: &id, BriefVersion: 2, Action: "keep", Reason: "Approved exception"}}
	if unresolvedImpact(state, "one", 1, "old") {
		t.Fatal("keep did not resolve review")
	}
}
