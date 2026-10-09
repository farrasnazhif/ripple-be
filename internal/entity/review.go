package entity

import "time"

type BriefPoint struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Scope string `json:"scope"`
}
type BriefVersion struct {
	Version     int          `json:"version"`
	RawText     string       `json:"raw_text"`
	Summary     string       `json:"summary"`
	Points      []BriefPoint `json:"points"`
	Attachments []string     `json:"attachments"`
	CreatedAt   time.Time    `json:"created_at"`
}
type PointDiff struct {
	Key string `json:"key"`
	Old string `json:"old"`
	New string `json:"new"`
}
type ShotReview struct {
	ShotID  string   `json:"shot_id"`
	Status  string   `json:"status"`
	Reasons []string `json:"reasons"`
}
type ProjectChange struct {
	ID           string       `json:"id"`
	BriefVersion int          `json:"brief_version"`
	Note         string       `json:"note"`
	Diffs        []PointDiff  `json:"diffs"`
	Impacts      []ShotReview `json:"impacts"`
	CreatedAt    time.Time    `json:"created_at"`
}
type ShotDecision struct {
	ID           string    `json:"id"`
	ShotID       string    `json:"shot_id"`
	BriefVersion int       `json:"brief_version"`
	Action       string    `json:"action"`
	Reason       string    `json:"reason"`
	JobID        *string   `json:"job_id"`
	CreatedAt    time.Time `json:"created_at"`
}
type FinalOutput struct {
	ID           string   `json:"id"`
	BriefVersion int      `json:"brief_version"`
	Kind         string   `json:"kind"`
	JobIDs       []string `json:"job_ids"`
	Outputs      []string `json:"outputs"`
	ObjectKey    string   `json:"-"`
	DownloadURL  string   `json:"download_url,omitempty"`
}
type ProjectReview struct {
	OutputType string          `json:"output_type"`
	Points     []BriefPoint    `json:"points"`
	Versions   []BriefVersion  `json:"versions"`
	Changes    []ProjectChange `json:"changes"`
	Decisions  []ShotDecision  `json:"decisions"`
	Finals     []FinalOutput   `json:"finals"`
}
type BriefEdit struct {
	ExpectedVersion   int          `json:"expected_version"`
	RawText           string       `json:"raw_text"`
	Summary           string       `json:"summary"`
	Points            []BriefPoint `json:"points"`
	RemoveAttachments []string     `json:"remove_attachments"`
	Note              string       `json:"note"`
}
