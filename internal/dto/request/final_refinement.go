package request

// FinalRefinement identifies a saved source; reference URLs are resolved server-side.
type FinalRefinement struct {
	RequestKey string `json:"request_key"`
	ImageIndex int    `json:"image_index"`
	Prompt     string `json:"prompt"`
	Reason     string `json:"reason"`
}
