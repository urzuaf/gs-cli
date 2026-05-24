package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type PathReturnContent struct {
	Content []string `json:"content"`
}

type QueryMetadata struct {
	Time       string `json:"time"`
	TotalPaths int    `json:"totalPaths"`
	PeakMemory string `json:"peakMemory"`
	MaxMemory  string `json:"maxMemory"`
}

type RequestReturnContent struct {
	Success  bool                `json:"success"`
	Message  string              `json:"message"`
	Data     []PathReturnContent `json:"data"`
	Metadata *QueryMetadata      `json:"metadata"`
}

type QueryRequest struct {
	Query string `json:"query"`
}

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(port int) *Client {
	return &Client{
		BaseURL: fmt.Sprintf("http://localhost:%d", port),
		HTTPClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

func (c *Client) CheckStatus() (bool, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/api/v1/healthcheck")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

func (c *Client) ListDatabases() ([]string, error) {
	if ok, err := c.CheckStatus(); !ok || err != nil {
		return nil, fmt.Errorf("server is not reachable")
	}

	resp, err := c.HTTPClient.Get(c.BaseURL + "/api/v1/database/list")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result RequestReturnContent
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("%s", result.Message)
	}

	if len(result.Data) > 0 {
		return result.Data[0].Content, nil
	}
	return []string{}, nil
}

func (c *Client) UseDatabase(dbName string) (string, error) {
	if ok, err := c.CheckStatus(); !ok || err != nil {
		return "", fmt.Errorf("server is not reachable")
	}

	resp, err := c.HTTPClient.Post(c.BaseURL+"/api/v1/database/use/"+dbName, "application/json", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result RequestReturnContent
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}
	return result.Message, nil
}

func (c *Client) Query(dbName, query string) (*RequestReturnContent, error) {
	if ok, err := c.CheckStatus(); !ok || err != nil {
		return nil, fmt.Errorf("server is not reachable")
	}

	reqBody, _ := json.Marshal(QueryRequest{Query: query})
	req, err := http.NewRequest("POST", c.BaseURL+"/api/v1/query/execute", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PathDB-Graph-DB", dbName)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result RequestReturnContent
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// Try to read raw body if decoding fails (for debugging)
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to decode response: %v, body: %s", err, string(body))
	}

	if !result.Success {
		return &result, fmt.Errorf("%s", result.Message)
	}

	return &result, nil
}
