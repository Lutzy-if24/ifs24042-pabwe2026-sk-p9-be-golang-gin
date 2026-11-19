package ai

import (
	"context"
	"net/http"
	"net/url"
)

// maxSearchResults membatasi jumlah sumber yang dikirim sebagai konteks.
const maxSearchResults = 5

// SearchClient adalah klien HTTP Search Engine.
//
// Kontrak yang diasumsikan: GET {endpoint}?q=<kata kunci> dengan balasan JSON
// {"results":[{"title","url","snippet"}]} (kunci "data" juga diterima).
// Sesuaikan struct searchResponse bila skema Search Engine Delcom berbeda.
type SearchClient struct {
	endpoint string
	http     *http.Client
}

// NewSearchClient membuat SearchClient.
func NewSearchClient(endpoint string, hc *http.Client) *SearchClient {
	return &SearchClient{endpoint: endpoint, http: hc}
}

type searchResponse struct {
	Results []SearchResult `json:"results"`
	Data    []SearchResult `json:"data"`
}

// Search mencari fakta terkini untuk sebuah topik.
func (c *SearchClient) Search(ctx context.Context, query string) ([]SearchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"?q="+url.QueryEscape(query), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	var out searchResponse
	if err := doJSON(c.http, req, &out); err != nil {
		return nil, err
	}
	results := out.Results
	if len(results) == 0 {
		results = out.Data
	}
	if len(results) > maxSearchResults {
		results = results[:maxSearchResults]
	}
	return results, nil
}
