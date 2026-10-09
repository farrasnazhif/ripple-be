package service

import "testing"

func TestDemoGraphImpact(t *testing.T) {
	for _, tc := range []struct {
		scenario string
		want     int
	}{
		{"s1", 3},
		{"s2", 12},
		{"s3", 0},
	} {
		graph, err := DemoGraph(tc.scenario)
		if err != nil {
			t.Fatal(err)
		}
		got := 0
		for _, node := range graph.Nodes {
			if node.Status == "Needs update" {
				got++
				if node.Reason == "" {
					t.Fatalf("%s has no reason", node.ID)
				}
			}
		}
		if got != tc.want {
			t.Errorf("%s affected %d shots, want %d", tc.scenario, got, tc.want)
		}
		if tc.scenario == "s2" {
			for _, node := range graph.Nodes {
				if node.ID == "shot-03" && len(node.Reasons) != 2 {
					t.Fatalf("shot 3 should retain both changed points: %+v", node.Reasons)
				}
			}
		}
	}
}
