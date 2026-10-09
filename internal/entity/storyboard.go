package entity

type StoryboardShot struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Prompt          string   `json:"prompt"`
	Position        int      `json:"position"`
	RequirementKeys []string `json:"requirement_keys"`
}
