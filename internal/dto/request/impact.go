package request

type ImpactPoint struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Scope       string `json:"scope"`
	SourceQuote string `json:"source_quote"`
}

type ImpactLink struct {
	PointKey  string `json:"point_key"`
	Confirmed bool   `json:"confirmed"`
}

type ImpactShot struct {
	ID         string       `json:"id"`
	Links      []ImpactLink `json:"links"`
	Exclusions []string     `json:"exclusions,omitempty"`
}

type ImpactDiff struct {
	PointKey  string `json:"point_key"`
	Old       string `json:"old"`
	New       string `json:"new"`
	Confirmed bool   `json:"confirmed"`
}

type ImpactPreview struct {
	Points []ImpactPoint `json:"points"`
	Shots  []ImpactShot  `json:"shots"`
	Diffs  []ImpactDiff  `json:"diffs"`
}
