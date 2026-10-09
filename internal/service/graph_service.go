package service

import (
	"fmt"

	"github.com/farrasnazhif/ripple-be/internal/dto/request"
	"github.com/farrasnazhif/ripple-be/internal/entity"
)

// DemoGraph is the synthetic Laju fixture from PRD section 21. It is not stored project data.
func DemoGraph(scenario string) (entity.Graph, error) {
	pointData := []request.ImpactPoint{
		{Key: "BP1", Value: "Morning", Scope: "global", SourceQuote: "Morning"},
		{Key: "BP2", Value: "Warm, golden orange", Scope: "global", SourceQuote: "Warm, golden orange"},
		{Key: "BP3", Value: "Clearly visible on the shoe", Scope: "local", SourceQuote: "Clearly visible on the shoe"},
		{Key: "BP4", Value: "A beginner runner builds a routine", Scope: "global", SourceQuote: "A beginner runner builds a routine"},
	}
	labels := []string{"Time of day", "Color palette", "Logo", "Story"}
	points := make([]entity.GraphNode, 0, len(pointData))
	for i, point := range pointData {
		points = append(points, entity.GraphNode{ID: point.Key, Kind: "point", Label: labels[i], Description: point.Value + " · " + point.Scope})
	}
	shots := []struct {
		description string
		method      string
		links       []string
	}{
		{"An alarm rings at dawn; the runner wakes up", "AI", []string{"BP1"}},
		{"Close-up of tying shoelaces, visible logo, warm light", "Traditional", []string{"BP3", "BP2"}},
		{"Wide shot of a neighborhood street in golden morning light", "AI", []string{"BP1", "BP2"}},
		{"The runner starts jogging slowly", "Traditional", []string{"BP4"}},
		{"Shoe sole striking the pavement", "AI", nil},
		{"The runner joins a running community", "Traditional", []string{"BP4"}},
		{"Montage of a city waking up", "AI", []string{"BP1"}},
		{"The runner stops for water and smiles", "Traditional", []string{"BP4"}},
		{"Close-up of the shoe logo during a sprint", "AI", []string{"BP3"}},
		{"Step-count graphic on a watch", "Traditional", []string{"BP4"}},
		{"The runner reaches a park at sunrise beneath an orange sky", "AI", []string{"BP1", "BP2"}},
		{"Final shoe packshot with logo and tagline", "Traditional", []string{"BP3"}},
	}

	graph := entity.Graph{
		Scenario: scenario,
		Nodes:    append([]entity.GraphNode{{ID: "brief", Kind: "brief", Label: "Laju campaign", Description: "Synthetic demo brief"}}, points...),
		Edges:    make([]entity.GraphEdge, 0, 32),
	}
	for _, point := range points {
		graph.Edges = append(graph.Edges, entity.GraphEdge{From: "brief", To: point.ID, Kind: "contains"})
	}

	var diffs []request.ImpactDiff
	switch scenario {
	case "s1":
		graph.Note = "The logo may appear only in the final packshot."
		diffs = []request.ImpactDiff{{PointKey: "BP3", Old: pointData[2].Value, New: "Only in the final packshot", Confirmed: true}}
	case "s2":
		graph.Note = "Change to a night run downtown with a cool blue palette and neon lights."
		diffs = []request.ImpactDiff{
			{PointKey: "BP1", Old: pointData[0].Value, New: "Night", Confirmed: true},
			{PointKey: "BP2", Old: pointData[1].Value, New: "Cool blue, neon", Confirmed: true},
		}
	case "s3":
		graph.Note = "Make it feel more premium. Clarification needed."
	default:
		return entity.Graph{}, fmt.Errorf("unknown scenario %q", scenario)
	}
	graph.Nodes = append(graph.Nodes, entity.GraphNode{ID: "change", Kind: "change", Label: "Director note", Description: graph.Note})
	for _, diff := range diffs {
		graph.Edges = append(graph.Edges, entity.GraphEdge{From: "change", To: diff.PointKey, Kind: "changes"})
	}
	impactInput := request.ImpactPreview{Points: pointData, Diffs: diffs}
	for i, shot := range shots {
		id := fmt.Sprintf("shot-%02d", i+1)
		node := entity.GraphNode{ID: id, Kind: "shot", Label: fmt.Sprintf("Shot %02d", i+1), Description: shot.description + " · " + shot.method}
		impactShot := request.ImpactShot{ID: id}
		for _, point := range shot.links {
			graph.Edges = append(graph.Edges, entity.GraphEdge{From: point, To: id, Kind: "direct"})
			impactShot.Links = append(impactShot.Links, request.ImpactLink{PointKey: point, Confirmed: true})
		}
		impactInput.Shots = append(impactInput.Shots, impactShot)
		graph.Nodes = append(graph.Nodes, node)
	}
	preview, err := PreviewImpact(impactInput)
	if err != nil {
		return entity.Graph{}, err
	}
	for i, shot := range preview.Shots {
		graph.Nodes[len(points)+2+i].Status = shot.Status
		if len(shot.Reasons) > 0 {
			graph.Nodes[len(points)+2+i].Reason = shot.Reasons[0].Message
			for _, reason := range shot.Reasons {
				graph.Nodes[len(points)+2+i].Reasons = append(graph.Nodes[len(points)+2+i].Reasons, entity.GraphReason{
					PointKey: reason.PointKey, SourceQuote: reason.SourceQuote, Kind: reason.Kind, Message: reason.Message,
				})
			}
		}
	}
	return graph, nil
}
