package media

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Search(query string, limit int) ([]map[string]any, error) {
	body := fmt.Sprintf(`{"query":"%s","limit":%d}`, query, limit)
	resp, err := c.request("POST", "/v1/search", body, 35)
	if err != nil {
		return nil, err
	}
	var result []map[string]any
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) Resolve(url string) (map[string]any, error) {
	body := fmt.Sprintf(`{"url":"%s"}`, url)
	resp, err := c.request("POST", "/v1/resolve", body, 75)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	if _, ok := result["media_id"]; !ok {
		return nil, fmt.Errorf("no media_id in response")
	}
	return result, nil
}

func (c *Client) Download(sourceURL, mediaID string) (map[string]any, error) {
	body := fmt.Sprintf(`{"url":"%s","media_id":"%s"}`, sourceURL, mediaID)
	resp, err := c.request("POST", "/v1/media/download", body, 150)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) Ensure(items []map[string]any) ([]string, error) {
	itemsJSON, _ := json.Marshal(items)
	body := fmt.Sprintf(`{"items":%s}`, string(itemsJSON))
	resp, err := c.request("POST", "/v1/media/ensure", body, 150)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	ready, _ := result["ready"].([]any)
	out := make([]string, 0, len(ready))
	for _, r := range ready {
		if s, ok := r.(string); ok {
			out = append(out, s)
		}
	}
	return out, nil
}

func (c *Client) Related(sourceURL string, limit int) ([]string, error) {
	body := fmt.Sprintf(`{"source_url":"%s","limit":%d}`, sourceURL, limit)
	resp, err := c.request("POST", "/v1/radio/related", body, 55)
	if err != nil {
		return nil, err
	}
	var result []string
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) Captions(sourceURL, lang string) (map[string]any, error) {
	url := fmt.Sprintf("%s/v1/captions?source_url=%s", c.baseURL, sourceURL)
	if lang != "" {
		url += "&lang=" + lang
	}
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return map[string]any{"lang": "", "auto": false, "cues": []any{}}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return map[string]any{"lang": "", "auto": false, "cues": []any{}}, nil
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func (c *Client) Content(mediaID, rangeHeader string) (int64, string, []byte, error) {
	url := fmt.Sprintf("%s/v1/media/%s", c.baseURL, mediaID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, "", nil, err
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", nil, err
	}
	return int64(resp.StatusCode), resp.Header.Get("Content-Type"), body, nil
}

func (c *Client) UpdateReferences(mediaIDs []string) error {
	ids, _ := json.Marshal(mediaIDs)
	body := fmt.Sprintf(`{"media_ids":%s}`, string(ids))
	_, err := c.request("POST", "/v1/media/references", body, 10)
	return err
}

func (c *Client) request(method, path, body string, timeoutSec int) ([]byte, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: time.Duration(timeoutSec) * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("media service unavailable: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("media service error: %s", string(data[:min(len(data), 500)]))
	}
	return data, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
