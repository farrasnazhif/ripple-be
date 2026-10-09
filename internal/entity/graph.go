package entity

type GraphNode struct {
	ID          string        `json:"id"`
	Kind        string        `json:"kind"`
	Label       string        `json:"label"`
	Description string        `json:"description,omitempty"`
	Status      string        `json:"status,omitempty"`
	Reason      string        `json:"reason,omitempty"`
	Reasons     []GraphReason `json:"reasons,omitempty"`
}

type GraphReason struct {
	PointKey    string `json:"point_key"`
	SourceQuote string `json:"source_quote,omitempty"`
	Kind        string `json:"kind"`
	Message     string `json:"message"`
}

type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type Graph struct {
	Scenario string      `json:"scenario"`
	Note     string      `json:"note"`
	Nodes    []GraphNode `json:"nodes"`
	Edges    []GraphEdge `json:"edges"`
}
