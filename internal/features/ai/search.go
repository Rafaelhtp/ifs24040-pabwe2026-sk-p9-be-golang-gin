package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SearchClient struct {
	endpoint string
	http     *http.Client
}

func NewSearchClient(endpoint string) *SearchClient {
	return &SearchClient{endpoint: endpoint, http: &http.Client{Timeout: 30 * time.Second}}
}

type searchResult struct {
	Results []SearchSource `json:"results"`
}

func (c *SearchClient) Search(ctx context.Context, query string) ([]SearchSource, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("q", query)
	req.URL.RawQuery = q.Encode()

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search engine mengembalikan status %d", resp.StatusCode)
	}

	var out searchResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Results, nil
}
