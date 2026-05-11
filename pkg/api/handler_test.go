package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"iec104-sim/pkg/config"
	"iec104-sim/pkg/library"
)

type mockPublisher struct{}
func (m *mockPublisher) Publish(point *config.Point) {}

type mockStatus struct{}
func (m *mockStatus) ClientConnected() bool                    { return false }
func (m *mockStatus) ClientAddr() string                       { return "" }
func (m *mockStatus) Stats() (interrog, control, spont int64)  { return 0, 0, 0 }
func (m *mockStatus) Uptime() int64                            { return 0 }

func newTestHandler() *Handler {
	points := []*config.Point{
		{IOA: 1001, Name: "AI_01", PointType: config.TypeAI, ValueType: config.VTFloat, Value: 220.0, Efficient: 1.0, BaseValue: 220.0},
		{IOA: 2001, Name: "DI_01", PointType: config.TypeDI, ValueType: config.VTBit, BoolValue: false},
		{IOA: 3001, Name: "PI_01", PointType: config.TypePI, ValueType: config.VTInt, IntValue: 1000},
	}
	store := library.NewStore(points)
	return &Handler{store: store, publisher: &mockPublisher{}, status: &mockStatus{}}
}

func TestListPoints(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/points", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	pts, ok := resp["points"].([]interface{})
	if !ok {
		t.Fatal("expected points array")
	}
	if len(pts) != 3 {
		t.Errorf("expected 3 points, got %d", len(pts))
	}
}

func TestGetPoint(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/points/1001", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var pt config.Point
	if err := json.NewDecoder(rec.Body).Decode(&pt); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if pt.IOA != 1001 || pt.Name != "AI_01" {
		t.Errorf("point mismatch: %+v", pt)
	}
}

func TestGetPoint_NotFound(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/points/9999", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestMethodNotAllowedPostIndex(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodDelete, "/api/points", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestBatchUpdatePoints(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	body := `{"points":[{"ioa":1001,"value":230.5},{"ioa":2001,"bool_value":true}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/points", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if resp["updated"].(float64) != 2 {
		t.Errorf("expected 2 updated, got %v", resp["updated"])
	}
}

func TestUpdatePoint(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	body := `{"value":230.5}`
	req := httptest.NewRequest(http.MethodPut, "/api/points/1001", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if resp["changed"].(bool) != true {
		t.Errorf("expected changed=true, got %v", resp["changed"])
	}

	// Verify the point value was actually updated
	req2 := httptest.NewRequest(http.MethodGet, "/api/points/1001", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)

	var pt config.Point
	json.NewDecoder(rec2.Body).Decode(&pt)
	if pt.Value != 230.5 {
		t.Errorf("expected value 230.5, got %f", pt.Value)
	}
}

func TestUpdatePoint_NotFound(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	body := `{"value":100.0}`
	req := httptest.NewRequest(http.MethodPut, "/api/points/9999", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateQDS(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	body := `{"invalid":true,"blocked":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/points/1001/qds", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Verify QDS was updated
	req2 := httptest.NewRequest(http.MethodGet, "/api/points/1001", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)

	var pt config.Point
	json.NewDecoder(rec2.Body).Decode(&pt)
	if !pt.QDS.Invalid || !pt.QDS.Blocked {
		t.Errorf("expected QDS invalid=true, blocked=true, got %+v", pt.QDS)
	}
}

func TestMethodNotAllowedOnQDS(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/points/1001/qds", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestStatus(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if resp["connected"].(bool) != false {
		t.Errorf("expected connected=false")
	}
}

func TestInvalidIOA(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/points/abc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
