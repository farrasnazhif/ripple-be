package main

import (
	"bytes"
	"image/png"
	"regexp"
	"testing"
)

func TestDemoIDsAreStableAndOwnerScoped(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range demos {
		id := demoID("owner-1", d.key)
		if id != demoID("owner-1", d.key) || id == demoID("owner-2", d.key) || seen[id] || !regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`).MatchString(id) {
			t.Fatal("invalid demo id", id)
		}
		seen[id] = true
	}
}
func TestDemoFlowsAndImages(t *testing.T) {
	counts := []int{0, 2, 4, 0, 0}
	for i, d := range demos {
		if len(d.name) > 100 || len(d.description) > 500 || d.brief == "" {
			t.Fatal("invalid project fixture")
		}
		count := 0
		for _, s := range d.shots {
			count += len(s.kinds)
			if s.title == "" || s.prompt == "" {
				t.Fatal("empty shot")
			}
		}
		if count != counts[i] || (len(d.shots) > 0 && d.summary == "") {
			t.Fatal("demo flows not distinct")
		}
		data, err := sampleImage(i)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil || img.Bounds().Dx() != 960 || img.Bounds().Dy() != 540 {
			t.Fatal("broken demo image")
		}
	}
	if len(demos) != 5 || demos[3].summary == "" || len(demos[3].shots) != 0 {
		t.Fatal("fourth demo must contain a brief and summary only")
	}
	if demos[4].brief == "" || demos[4].summary == "" || len(demos[4].shots) == 0 || demos[4].failed {
		t.Fatal("fifth demo must stop at the storyboard")
	}
	for _, shot := range demos[4].shots {
		if len(shot.kinds) != 0 {
			t.Fatal("fifth demo must not generate media")
		}
	}
	if demos[0].summary != "" || !demos[2].failed {
		t.Fatal("missing draft or failure scenario")
	}
}
