package response

type ImpactReason struct {
	PointKey    string `json:"point_key"`
	SourceQuote string `json:"source_quote"`
	Kind        string `json:"kind"`
	Message     string `json:"message"`
}

type ShotImpact struct {
	ShotID  string         `json:"shot_id"`
	Status  string         `json:"status"`
	Reasons []ImpactReason `json:"reasons"`
}

type ImpactPreview struct {
	Shots []ShotImpact `json:"shots"`
}
