// Package ai menghasilkan draf artikel blog memakai OpenAI Chat Completions,
// diperkaya konteks hasil penelusuran Search Engine terkini.
package ai

// GenerateRequest adalah payload POST /api/posts/ai-generate.
type GenerateRequest struct {
	Topic            string `json:"topic" binding:"required,min=3,max=200"`
	Category         string `json:"category"`
	IncludeWebSearch bool   `json:"include_web_search"`
}

// SearchResult adalah satu sumber rujukan dari Search Engine.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// GenerateResponse adalah draf artikel hasil AI beserta sumber penelusuran.
type GenerateResponse struct {
	Title         string         `json:"title"`
	Category      string         `json:"category"`
	Excerpt       string         `json:"excerpt"`
	Content       string         `json:"content"`
	Sources       []SearchResult `json:"sources"`
	UsedWebSearch bool           `json:"used_web_search"`
	SearchWarning string         `json:"search_warning,omitempty"`
}
