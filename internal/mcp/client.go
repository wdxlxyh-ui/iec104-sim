package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// SimulatorClient wraps HTTP calls to the IEC104 simulator.
type SimulatorClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewSimulatorClient creates a new client pointing at the simulator HTTP API.
func NewSimulatorClient(baseURL string) *SimulatorClient {
	return &SimulatorClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{},
	}
}

func (c *SimulatorClient) url(path string) string {
	return c.baseURL + path
}

// ---- Instance Management ----

func (c *SimulatorClient) ListInstances() (json.RawMessage, error) {
	return c.get("/api/v1/instances")
}

func (c *SimulatorClient) GetInstance(id string) (json.RawMessage, error) {
	return c.get("/api/v1/instances/" + id)
}

func (c *SimulatorClient) CreateInstance(body json.RawMessage) (json.RawMessage, error) {
	return c.post("/api/v1/instances", body)
}

func (c *SimulatorClient) UpdateInstance(id string, body json.RawMessage) (json.RawMessage, error) {
	return c.put("/api/v1/instances/"+id, body)
}

func (c *SimulatorClient) DeleteInstance(id string) (json.RawMessage, error) {
	return c.delete("/api/v1/instances/" + id)
}

func (c *SimulatorClient) StartInstance(id string) (json.RawMessage, error) {
	return c.post("/api/v1/instances/"+id+"/start", nil)
}

func (c *SimulatorClient) StopInstance(id string) (json.RawMessage, error) {
	return c.post("/api/v1/instances/"+id+"/stop", nil)
}

func (c *SimulatorClient) RestartInstance(id string) (json.RawMessage, error) {
	return c.post("/api/v1/instances/"+id+"/restart", nil)
}

func (c *SimulatorClient) GetServerStatus() (json.RawMessage, error) {
	return c.get("/api/v1/status")
}

// ---- Data Operations ----

func (c *SimulatorClient) ListPoints(instID string) (json.RawMessage, error) {
	return c.get(fmt.Sprintf("/api/v1/instances/%s/points", instID))
}

func (c *SimulatorClient) ReadPoint(instID string, ioa uint32) (json.RawMessage, error) {
	return c.get(fmt.Sprintf("/api/v1/instances/%s/points/%d", instID, ioa))
}

func (c *SimulatorClient) WritePoint(instID string, ioa uint32, value json.RawMessage) (json.RawMessage, error) {
	return c.put(fmt.Sprintf("/api/v1/instances/%s/points/%d", instID, ioa), value)
}

func (c *SimulatorClient) WritePoints(instID string, points json.RawMessage) (json.RawMessage, error) {
	return c.post(fmt.Sprintf("/api/v1/instances/%s/points/batch", instID), points)
}

func (c *SimulatorClient) GetAutoChange(instID string, ioa uint32) (json.RawMessage, error) {
	return c.get(fmt.Sprintf("/api/v1/instances/%s/points/auto-change/%d", instID, ioa))
}

func (c *SimulatorClient) ConfigAutoChange(instID string, ioa uint32, body json.RawMessage) (json.RawMessage, error) {
	return c.put(fmt.Sprintf("/api/v1/instances/%s/points/auto-change/%d", instID, ioa), body)
}

func (c *SimulatorClient) BatchAutoChange(instID string, body json.RawMessage) (json.RawMessage, error) {
	return c.put(fmt.Sprintf("/api/v1/instances/%s/points/auto-change/batch", instID), body)
}

func (c *SimulatorClient) DeleteAutoChange(instID string, ioa uint32) (json.RawMessage, error) {
	return c.delete(fmt.Sprintf("/api/v1/instances/%s/points/auto-change/%d", instID, ioa))
}

func (c *SimulatorClient) ExportAutoChanges(instID string) (json.RawMessage, error) {
	return c.get(fmt.Sprintf("/api/v1/instances/%s/points/auto-change/export", instID))
}

func (c *SimulatorClient) ImportAutoChanges(instID string, csvContent string) (json.RawMessage, error) {
	// CSV import uses multipart form
	body := &bytes.Buffer{}
	body.WriteString(csvContent)
	return c.postRaw(fmt.Sprintf("/api/v1/instances/%s/points/auto-change/import", instID),
		"text/csv", body.Bytes())
}

func (c *SimulatorClient) UploadCSV(instID string, csvContent string) (json.RawMessage, error) {
	body := &bytes.Buffer{}
	body.WriteString(csvContent)
	return c.postRaw(fmt.Sprintf("/api/v1/instances/%s/upload-csv", instID),
		"text/csv", body.Bytes())
}

func (c *SimulatorClient) ExportPointsCSV(instID string) (json.RawMessage, error) {
	return c.get(fmt.Sprintf("/api/v1/instances/%s/points/export", instID))
}

// ---- HTTP helpers ----

func (c *SimulatorClient) get(path string) (json.RawMessage, error) {
	resp, err := c.httpClient.Get(c.url(path))
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	return c.readResponse(resp)
}

func (c *SimulatorClient) post(path string, body json.RawMessage) (json.RawMessage, error) {
	var buf io.Reader
	if body != nil {
		buf = bytes.NewReader(body)
	}
	req, err := http.NewRequest(http.MethodPost, c.url(path), buf)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	return c.readResponse(resp)
}

func (c *SimulatorClient) put(path string, body json.RawMessage) (json.RawMessage, error) {
	var buf io.Reader
	if body != nil {
		buf = bytes.NewReader(body)
	}
	req, err := http.NewRequest(http.MethodPut, c.url(path), buf)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PUT %s: %w", path, err)
	}
	defer resp.Body.Close()
	return c.readResponse(resp)
}

func (c *SimulatorClient) delete(path string) (json.RawMessage, error) {
	req, err := http.NewRequest(http.MethodDelete, c.url(path), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DELETE %s: %w", path, err)
	}
	defer resp.Body.Close()
	return c.readResponse(resp)
}

func (c *SimulatorClient) postRaw(path, contentType string, body []byte) (json.RawMessage, error) {
	req, err := http.NewRequest(http.MethodPost, c.url(path), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	return c.readResponse(resp)
}

func (c *SimulatorClient) readResponse(resp *http.Response) (json.RawMessage, error) {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw))
	}
	return raw, nil
}

func getStringArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func getFloatArg(args map[string]any, key string) float64 {
	v, _ := args[key].(float64)
	return v
}

func getBoolArg(args map[string]any, key string) bool {
	v, _ := args[key].(bool)
	return v
}

func getUint32Arg(args map[string]any, key string) uint32 {
	switch v := args[key].(type) {
	case float64:
		return uint32(v)
	case string:
		n, _ := strconv.ParseUint(v, 10, 32)
		return uint32(n)
	}
	return 0
}

func getFloat64Array(args map[string]any, key string) []float64 {
	v, _ := args[key].([]any)
	if v == nil {
		return nil
	}
	res := make([]float64, 0, len(v))
	for _, item := range v {
		if f, ok := item.(float64); ok {
			res = append(res, f)
		}
	}
	return res
}

func getStringArray(args map[string]any, key string) []string {
	v, _ := args[key].([]any)
	if v == nil {
		return nil
	}
	res := make([]string, len(v))
	for i, item := range v {
		res[i], _ = item.(string)
	}
	return res
}
