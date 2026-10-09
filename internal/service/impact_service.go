package service

import (
	"fmt"
	"strings"

	"github.com/farrasnazhif/ripple-be/internal/dto/request"
	"github.com/farrasnazhif/ripple-be/internal/dto/response"
)

// PreviewImpact calculates shot impact from confirmed point changes and links.
func PreviewImpact(input request.ImpactPreview) (response.ImpactPreview, error) {
	if len(input.Points) == 0 || len(input.Shots) == 0 {
		return response.ImpactPreview{}, fmt.Errorf("points and shots are required")
	}
	points := make(map[string]request.ImpactPoint, len(input.Points))
	for _, point := range input.Points {
		if strings.TrimSpace(point.Key) == "" || strings.TrimSpace(point.Value) == "" || (point.Scope != "global" && point.Scope != "local") {
			return response.ImpactPreview{}, fmt.Errorf("invalid point %q", point.Key)
		}
		if _, exists := points[point.Key]; exists {
			return response.ImpactPreview{}, fmt.Errorf("duplicate point %q", point.Key)
		}
		points[point.Key] = point
	}
	diffs := make(map[string]request.ImpactDiff, len(input.Diffs))
	for _, diff := range input.Diffs {
		point, exists := points[diff.PointKey]
		if !exists || diff.Old != point.Value || strings.TrimSpace(diff.New) == "" || diff.Old == diff.New {
			return response.ImpactPreview{}, fmt.Errorf("invalid diff for %q", diff.PointKey)
		}
		if _, exists := diffs[diff.PointKey]; exists {
			return response.ImpactPreview{}, fmt.Errorf("duplicate diff for %q", diff.PointKey)
		}
		diffs[diff.PointKey] = diff
	}
	result := response.ImpactPreview{Shots: make([]response.ShotImpact, 0, len(input.Shots))}
	seenShots := make(map[string]bool, len(input.Shots))
	for _, shot := range input.Shots {
		if strings.TrimSpace(shot.ID) == "" || seenShots[shot.ID] {
			return response.ImpactPreview{}, fmt.Errorf("invalid or duplicate shot %q", shot.ID)
		}
		seenShots[shot.ID] = true
		excluded := make(map[string]bool, len(shot.Exclusions))
		for _, key := range shot.Exclusions {
			if _, ok := points[key]; !ok {
				return response.ImpactPreview{}, fmt.Errorf("unknown exclusion %q", key)
			}
			excluded[key] = true
		}
		linked := make(map[string]bool, len(shot.Links))
		for _, link := range shot.Links {
			if _, ok := points[link.PointKey]; !ok {
				return response.ImpactPreview{}, fmt.Errorf("unknown link %q", link.PointKey)
			}
			if link.Confirmed {
				linked[link.PointKey] = true
			}
		}
		impact := response.ShotImpact{ShotID: shot.ID, Status: "Unchanged", Reasons: []response.ImpactReason{}}
		pending := false
		for _, point := range input.Points {
			diff, changed := diffs[point.Key]
			if !changed || excluded[point.Key] {
				continue
			}
			applies := linked[point.Key] || point.Scope == "global"
			if !applies {
				continue
			}
			if !diff.Confirmed {
				pending = true
				continue
			}
			kind, message := "direct", "Directly linked to "+point.Key
			if !linked[point.Key] {
				kind, message = "global", "Applies to every shot; not mentioned directly in the description"
			}
			impact.Reasons = append(impact.Reasons, response.ImpactReason{PointKey: point.Key, SourceQuote: point.SourceQuote, Kind: kind, Message: message})
		}
		if len(impact.Reasons) > 0 {
			impact.Status = "Needs update"
		} else if pending {
			impact.Status = "Ambiguous"
		}
		result.Shots = append(result.Shots, impact)
	}
	return result, nil
}
