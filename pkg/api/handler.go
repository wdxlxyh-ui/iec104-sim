package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"iec104-sim/pkg/config"
	"iec104-sim/pkg/library"
)

type Publisher interface {
	Publish(point *config.Point)
}

type StatusProvider interface {
	ClientConnected() bool
	ClientAddr() string
	Stats() (interrog, control, spont int64)
	Uptime() int64
}

type Handler struct {
	store     *library.Store
	publisher Publisher
	status    StatusProvider
}

func NewHandler(store *library.Store, publisher Publisher, status StatusProvider) *Handler {
	return &Handler{store: store, publisher: publisher, status: status}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/points", h.handlePoints)
	mux.HandleFunc("/api/points/", h.handlePointsByIOA)
	mux.HandleFunc("/api/status", h.handleStatus)
}

func (h *Handler) handlePoints(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		h.listPoints(w, r)
	case http.MethodPost:
		h.batchUpdatePoints(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handlePointsByIOA(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse IOA from URL: /api/points/{ioa}, /api/points/{ioa}/qds
	path := strings.TrimPrefix(r.URL.Path, "/api/points/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "missing IOA")
		return
	}

	ioa, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid IOA: "+parts[0])
		return
	}

	// QDS sub-path
	if len(parts) >= 2 && parts[1] == "qds" {
		if r.Method == http.MethodPut {
			h.updateQDS(w, r, uint32(ioa))
		} else {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getPoint(w, r, uint32(ioa))
	case http.MethodPut:
		h.updatePoint(w, r, uint32(ioa))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) listPoints(w http.ResponseWriter, r *http.Request) {
	points := h.store.GetAll()
	resp := map[string]interface{}{"points": points}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) getPoint(w http.ResponseWriter, r *http.Request, ioa uint32) {
	pt, ok := h.store.Get(ioa)
	if !ok {
		writeError(w, http.StatusNotFound, "point not found")
		return
	}
	writeJSON(w, http.StatusOK, pt)
}

func (h *Handler) updatePoint(w http.ResponseWriter, r *http.Request, ioa uint32) {
	var body struct {
		Value     *float64 `json:"value"`
		BoolValue *bool    `json:"bool_value"`
		IntValue  *int32   `json:"int_value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	pt, ok := h.store.Get(ioa)
	if !ok {
		writeError(w, http.StatusNotFound, "point not found")
		return
	}

	var changed bool
	switch pt.PointType {
	case config.TypeAI, config.TypeAO:
		if body.Value != nil {
			if _, err := h.store.SetValue(ioa, *body.Value); err == nil {
				changed = true
			}
		}
	case config.TypeDI, config.TypeDO:
		if body.BoolValue != nil {
			if _, err := h.store.SetBoolValue(ioa, *body.BoolValue); err == nil {
				changed = true
			}
		} else if body.Value != nil {
			if _, err := h.store.SetBoolValue(ioa, int64(*body.Value) != 0); err == nil {
				changed = true
			}
		}
	case config.TypePI:
		if body.IntValue != nil {
			if _, err := h.store.SetIntValue(ioa, *body.IntValue); err == nil {
				changed = true
			}
		} else if body.Value != nil {
			if _, err := h.store.SetIntValue(ioa, int32(*body.Value)); err == nil {
				changed = true
			}
		}
	}

	if changed {
		h.publisher.Publish(pt)
		pt, _ = h.store.Get(ioa)
		slog.Info("HTTP修改点值", "ioa", ioa, "value", formatPointValue(pt))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"ioa":     ioa,
		"changed": changed,
	})
}

func (h *Handler) batchUpdatePoints(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Points []struct {
			IOA       uint32   `json:"ioa"`
			Value     *float64 `json:"value"`
			BoolValue *bool    `json:"bool_value"`
			IntValue  *int32   `json:"int_value"`
		} `json:"points"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	updated := 0
	failed := 0
	var details []map[string]interface{}

	for _, p := range body.Points {
		pt, ok := h.store.Get(p.IOA)
		if !ok {
			failed++
			details = append(details, map[string]interface{}{"ioa": p.IOA, "success": false})
			continue
		}

		var err error
		switch pt.PointType {
		case config.TypeAI, config.TypeAO:
			if p.Value != nil {
				_, err = h.store.SetValue(p.IOA, *p.Value)
			}
		case config.TypeDI, config.TypeDO:
			if p.BoolValue != nil {
				_, err = h.store.SetBoolValue(p.IOA, *p.BoolValue)
			} else if p.Value != nil {
				_, err = h.store.SetBoolValue(p.IOA, int64(*p.Value) != 0)
			}
		case config.TypePI:
			if p.IntValue != nil {
				_, err = h.store.SetIntValue(p.IOA, *p.IntValue)
			} else if p.Value != nil {
				_, err = h.store.SetIntValue(p.IOA, int32(*p.Value))
			}
		}

		if err != nil {
			failed++
			details = append(details, map[string]interface{}{"ioa": p.IOA, "success": false})
			continue
		}

		h.publisher.Publish(pt)
		updated++
		details = append(details, map[string]interface{}{"ioa": p.IOA, "success": true})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"updated": updated,
		"failed":  failed,
		"details": details,
	})
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	interrog, control, spont := h.status.Stats()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"connected":    h.status.ClientConnected(),
		"client_addr":  h.status.ClientAddr(),
		"uptime":       h.status.Uptime(),
		"interrog":     interrog,
		"control":      control,
		"spont":        spont,
	})
}

func (h *Handler) updateQDS(w http.ResponseWriter, r *http.Request, ioa uint32) {
	var qds config.QualityDescriptor
	if err := json.NewDecoder(r.Body).Decode(&qds); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if _, err := h.store.SetQDS(ioa, qds); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func formatPointValue(pt *config.Point) interface{} {
	switch pt.PointType {
	case config.TypeAI, config.TypeAO:
		return pt.Value
	case config.TypeDI, config.TypeDO:
		return pt.BoolValue
	case config.TypePI:
		return pt.IntValue
	default:
		return nil
	}
}
