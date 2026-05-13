package detail

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"iec104-sim/internal/model"
	"iec104-sim/pkg/config"
	"iec104-sim/pkg/library"
)

type DetailHandler struct {
	store  *library.Store
	engine *Engine
	cfgDir string
	instID string
}

func NewDetailHandler(instID string, store *library.Store, engine *Engine, cfgDir string) *DetailHandler {
	return &DetailHandler{
		store:  store,
		engine: engine,
		cfgDir: cfgDir,
		instID: instID,
	}
}

func (h *DetailHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/instances/"+h.instID+"/points", h.handlePoints)
	mux.HandleFunc("/api/v1/instances/"+h.instID+"/points/", h.handlePointsByIOA)
	mux.HandleFunc("/api/v1/instances/"+h.instID+"/upload-csv", h.handleUploadCSV)
}

func (h *DetailHandler) handlePoints(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listSnapshots(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *DetailHandler) handlePointsByIOA(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/instances/"+h.instID+"/points/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing IOA"})
		return
	}

	// Handle sub-routes
	if parts[0] == "export" {
		h.exportCSV(w, r)
		return
	}
	if parts[0] == "auto-change" {
		h.handleAutoChangeConfig(w, r, parts)
		return
	}
	if parts[0] == "batch" && r.Method == http.MethodPost {
		h.batchSetValue(w, r)
		return
	}

	ioa, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid IOA: " + parts[0]})
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getSnapshot(w, uint32(ioa))
	case http.MethodPut:
		h.setValue(w, r, uint32(ioa))
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *DetailHandler) handleAutoChangeConfig(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) >= 2 && parts[1] == "batch" {
		h.batchAutoChange(w, r)
		return
	}
	if len(parts) >= 2 && parts[1] == "export" {
		h.exportAutoConfig(w, r)
		return
	}
	if len(parts) >= 2 && parts[1] == "import" {
		h.importAutoConfig(w, r)
		return
	}

	if len(parts) < 2 || parts[1] == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing IOA for auto-change"})
		return
	}

	ioa, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid IOA"})
		return
	}

	p, ok := h.store.Get(uint32(ioa))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "point not found"})
		return
	}
	if IsAODO(p.PointType) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "AO/DO does not support auto-change"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getAutoChange(w, uint32(ioa))
	case http.MethodPut:
		h.putAutoChange(w, r, uint32(ioa))
	case http.MethodDelete:
		h.deleteAutoChange(w, uint32(ioa))
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *DetailHandler) listSnapshots(w http.ResponseWriter, r *http.Request) {
	points := h.store.GetAll()
	snapshots := make([]model.PointSnapshot, 0, len(points))
	for _, p := range points {
		snapshots = append(snapshots, pointToSnapshot(p))
	}
	// Sort: AI first, then all by IOA ascending
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].PointType != snapshots[j].PointType {
			if snapshots[i].PointType == "AI" {
				return true
			}
			if snapshots[j].PointType == "AI" {
				return false
			}
		}
		return snapshots[i].IOA < snapshots[j].IOA
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"points":       snapshots,
		"refreshed_at": time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
	})
}

func (h *DetailHandler) getSnapshot(w http.ResponseWriter, ioa uint32) {
	p, ok := h.store.Get(ioa)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "point not found"})
		return
	}
	writeJSON(w, http.StatusOK, pointToSnapshot(p))
}

func (h *DetailHandler) setValue(w http.ResponseWriter, r *http.Request, ioa uint32) {
	p, ok := h.store.Get(ioa)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "point not found"})
		return
	}

	// Check API update policy
	if !h.engine.CheckAPIWriteAllowed(ioa) {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "该点已配置自动变化策略，不允许通过接口写入",
		})
		return
	}

	if !IsSetValueAllowed(p.PointType) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "AO/DO does not support set-value"})
		return
	}

	var body struct {
		Value     *float64 `json:"value"`
		BoolValue *bool    `json:"bool_value"`
		IntValue  *int32   `json:"int_value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	var changed bool
	switch p.PointType {
	case config.TypeAI:
		if body.Value != nil {
			if _, err := h.store.SetValue(ioa, *body.Value); err == nil {
				changed = true
			}
		}
	case config.TypeDI:
		if body.BoolValue != nil {
			if _, err := h.store.SetBoolValue(ioa, *body.BoolValue); err == nil {
				changed = true
			}
		} else if body.Value != nil {
			bv := int64(*body.Value) != 0
			if _, err := h.store.SetBoolValue(ioa, bv); err == nil {
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
		h.engine.HandleAOFollow(ioa)
		pub, ok := h.store.Get(ioa)
		if ok {
			// We need the publisher from the engine. Since Handler owns engine reference,
			// we call publish via the same publisher interface stored in engine.
			h.engine.pub.Publish(pub)
		}
		slog.Info("置数成功", "ioa", ioa, "instance", h.instID)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"ioa":     ioa,
		"changed": changed,
	})
}

// batchSetValue handles POST /points/batch — write multiple point values in one request.
// This is the core interface for automated testing: it ensures all points are written
// within a single request, so EGC polls see consistent data.
func (h *DetailHandler) batchSetValue(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Points []struct {
			IOA       uint32   `json:"ioa"`
			Value     *float64 `json:"value"`
			BoolValue *bool    `json:"bool_value"`
			IntValue  *int32   `json:"int_value"`
		} `json:"points"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if len(body.Points) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty points array"})
		return
	}

	type result struct {
		IOA     uint32 `json:"ioa"`
		Success bool   `json:"success"`
		Changed bool   `json:"changed,omitempty"`
		Error   string `json:"error,omitempty"`
	}
	results := make([]result, 0, len(body.Points))
	succeeded := 0

	for _, pt := range body.Points {
		p, ok := h.store.Get(pt.IOA)
		if !ok {
			results = append(results, result{IOA: pt.IOA, Success: false, Error: "point not found"})
			continue
		}
		if !IsSetValueAllowed(p.PointType) {
			results = append(results, result{IOA: pt.IOA, Success: false, Error: "AO/DO does not support set-value"})
			continue
		}
		if !h.engine.CheckAPIWriteAllowed(pt.IOA) {
			results = append(results, result{IOA: pt.IOA, Success: false, Error: "point has auto-change strategy that blocks API writes"})
			continue
		}

		var changed bool
		switch p.PointType {
		case config.TypeAI:
			if pt.Value != nil {
				if _, err := h.store.SetValue(pt.IOA, *pt.Value); err == nil {
					changed = true
				}
			}
		case config.TypeDI:
			if pt.BoolValue != nil {
				if _, err := h.store.SetBoolValue(pt.IOA, *pt.BoolValue); err == nil {
					changed = true
				}
			} else if pt.Value != nil {
				bv := int64(*pt.Value) != 0
				if _, err := h.store.SetBoolValue(pt.IOA, bv); err == nil {
					changed = true
				}
			}
		case config.TypePI:
			if pt.IntValue != nil {
				if _, err := h.store.SetIntValue(pt.IOA, *pt.IntValue); err == nil {
					changed = true
				}
			} else if pt.Value != nil {
				if _, err := h.store.SetIntValue(pt.IOA, int32(*pt.Value)); err == nil {
					changed = true
				}
			}
		}

		if changed {
			h.engine.HandleAOFollow(pt.IOA)
			pub, ok := h.store.Get(pt.IOA)
			if ok {
				h.engine.pub.Publish(pub)
			}
			succeeded++
		}
		results = append(results, result{
			IOA:     pt.IOA,
			Success: true,
			Changed: changed,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"results":   results,
		"total":     len(body.Points),
		"succeeded": succeeded,
		"failed":    len(body.Points) - succeeded,
	})
}

func (h *DetailHandler) getAutoChange(w http.ResponseWriter, ioa uint32) {
	cfg, ok := h.engine.GetConfig(ioa)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no auto-change config"})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *DetailHandler) putAutoChange(w http.ResponseWriter, r *http.Request, ioa uint32) {
	var req struct {
		Strategy string           `json:"strategy"`
		Enabled  bool             `json:"enabled"`
		Params   model.StrategyParams `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Params.PeriodMs > 0 && req.Params.PeriodMs < 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "period_ms must be >= 100"})
		return
	}

	cfg := &model.AutoChangeConfig{
		PointIOA:  ioa,
		Strategy:  model.StrategyType(req.Strategy),
		Enabled:   req.Enabled,
		Params:    req.Params,
		UpdatedAt: time.Now(),
	}

	if err := h.engine.StartOrUpdate(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "ioa": ioa})
}

func (h *DetailHandler) deleteAutoChange(w http.ResponseWriter, ioa uint32) {
	if err := h.engine.Remove(ioa); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (h *DetailHandler) batchAutoChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		IOAs   []uint32            `json:"ioas"`
		Config json.RawMessage     `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	var autoCfg struct {
		Strategy string           `json:"strategy"`
		Enabled  bool             `json:"enabled"`
		Params   model.StrategyParams `json:"params"`
	}
	if err := json.Unmarshal(req.Config, &autoCfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid config: " + err.Error()})
		return
	}

	if autoCfg.Params.PeriodMs > 0 && autoCfg.Params.PeriodMs < 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "period_ms must be >= 100"})
		return
	}

	success := 0
	failed := 0
	var errors []string

	for _, ioa := range req.IOAs {
		p, ok := h.store.Get(ioa)
		if !ok {
			failed++
			errors = append(errors, fmt.Sprintf("IOA %d: point not found", ioa))
			continue
		}
		if IsAODO(p.PointType) {
			failed++
			errors = append(errors, fmt.Sprintf("IOA %d: AO/DO not supported", ioa))
			continue
		}

		cfg := &model.AutoChangeConfig{
			PointIOA: ioa,
			Strategy: model.StrategyType(autoCfg.Strategy),
			Enabled:  autoCfg.Enabled,
			Params:   autoCfg.Params,
			UpdatedAt: time.Now(),
		}
		if err := h.engine.StartOrUpdate(cfg); err != nil {
			failed++
			errors = append(errors, fmt.Sprintf("IOA %d: %s", ioa, err.Error()))
			continue
		}
		success++
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"total":   len(req.IOAs),
		"ok":      success,
		"failed":  failed,
		"errors":  errors,
	})
}

func (h *DetailHandler) exportAutoConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	points := h.store.GetAll()
	configs := h.engine.AllConfigs()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="auto_changes_%s.csv"`, h.instID))
	// BOM for Excel UTF-8
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header: IOA, name, mode code, then A~G param columns
	writer.Write([]string{"信息体地址", "测点名称", "自动变化模式", "A", "B", "C", "D", "E", "F", "G"})

	// Legend rows: explain codes and A~G mapping per strategy
	writer.Write([]string{"", "代码说明", "1=递增 2=随机 3=CSV 4=取大 5=取小 6=SOC 7=电量 8=AO关联 9=接口更新", "", "", "", "", "", "", ""})
	writer.Write([]string{"", "递增(A~D)", "A=起始值 B=步长 C=周期(ms) D=最大值", "", "", "", "", "", "", ""})
	writer.Write([]string{"", "随机(A~D)", "A=最小值 B=最大值 C=周期(ms) D=小数位数", "", "", "", "", "", "", ""})
	writer.Write([]string{"", "MAX/MIN(A~B)", "A=IOA列表(分号分隔) B=关联IOA", "", "", "", "", "", "", ""})
	writer.Write([]string{"", "SOC(A~D)", "A=初始SOC(%) B=额定容量(kWh) C=功率AI点号 D=积分周期(ms)", "", "", "", "", "", "", ""})
	writer.Write([]string{"", "电量(A~D)", "A=初始电量(kWh) B=统计类别(0充电/1放电) C=功率AI点号 D=积分周期(ms)", "", "", "", "", "", "", ""})
	writer.Write([]string{"", "AO关联(A)", "A=关联AO点号", "", "", "", "", "", "", ""})
	writer.Write([]string{"", "接口更新(A)", "A=初始值", "", "", "", "", "", "", ""})
	writer.Write([]string{"", "CSV", "不支持表格导入，请在界面中单独配置", "", "", "", "", "", "", ""})

	// Sort points by IOA ascending for consistent output
	sort.Slice(points, func(i, j int) bool {
		return points[i].IOA < points[j].IOA
	})

	for _, p := range points {
		code := ""
		var a, b, c, d, e, f, g string
		if cfg, ok := configs[p.IOA]; ok && cfg.Enabled {
			code = strategyToCode(cfg.Strategy)
			a, b, c, d, e, f, g = paramsToABCDEFG(cfg.Strategy, &cfg.Params)
		}
		writer.Write([]string{
			strconv.FormatUint(uint64(p.IOA), 10),
			p.Name,
			code,
			a, b, c, d, e, f, g,
		})
	}
}

func (h *DetailHandler) importAutoConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	r.ParseMultipartForm(32 << 20)
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no CSV file provided"})
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to parse CSV: " + err.Error()})
		return
	}

	if len(records) < 3 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "CSV file is empty or missing data rows"})
		return
	}

	// Skip header (row 0), legend rows (rows 1~9), process data rows
	legendCount := 0
	for i, row := range records {
		if i == 0 {
			continue
		}
		code := ""
		if len(row) > 2 {
			code = strings.TrimSpace(row[2])
		}
		if code == "" || strings.ContainsAny(code, "=") {
			legendCount++
			continue
		}
		break
	}

	configs := make(map[uint32]*model.AutoChangeConfig)
	success := 0
	skipped := 0
	dataRows := records[legendCount+1:]
	for _, row := range dataRows {
		if len(row) < 3 {
			skipped++
			continue
		}
		ioa, err := strconv.ParseUint(strings.TrimSpace(row[0]), 10, 32)
		if err != nil {
			skipped++
			continue
		}
		code := strings.TrimSpace(row[2])
		if code == "" {
			skipped++
			continue
		}
		strategy := codeToStrategy(code)
		if strategy == "" {
			skipped++
			continue
		}
		p, ok := h.store.Get(uint32(ioa))
		if !ok {
			skipped++
			continue
		}
		if IsAODO(p.PointType) {
			skipped++
			continue
		}

		// Parse A~G columns
		cols := make([]string, 7)
		for j := 0; j < 7 && j+3 < len(row); j++ {
			cols[j] = strings.TrimSpace(row[j+3])
		}
		params := abcdefgToParams(strategy, cols[0], cols[1], cols[2], cols[3], cols[4], cols[5], cols[6])

		configs[uint32(ioa)] = &model.AutoChangeConfig{
			PointIOA:  uint32(ioa),
			Strategy:  strategy,
			Enabled:   true,
			Params:    params,
			UpdatedAt: time.Now(),
		}
		success++
	}

	if err := h.engine.SaveAll(configs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"total":   len(records) - 2,
		"applied": success,
		"skipped": skipped,
	})
}

func (h *DetailHandler) exportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	points := h.store.GetAll()

	// Sort: AI first, then by IOA ascending (match UI display order)
	sort.Slice(points, func(i, j int) bool {
		if (points[i].PointType == config.TypeAI) != (points[j].PointType == config.TypeAI) {
			return points[i].PointType == config.TypeAI
		}
		return points[i].IOA < points[j].IOA
	})

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="points_%s.csv"`, h.instID))
	// BOM for Excel UTF-8
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	defer writer.Flush()

	writer.Write([]string{"信息体地址", "测点名称", "测点类型", "实时值", "测点值更新时间"})
	for _, p := range points {
		val := formatPointValueStr(p)
		ts := p.Timestamp.Format("2006-01-02 15:04:05.000")
		writer.Write([]string{
			strconv.FormatUint(uint64(p.IOA), 10),
			p.Name,
			string(p.PointType),
			val,
			ts,
		})
	}
}

func (h *DetailHandler) handleUploadCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	r.ParseMultipartForm(32 << 20)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no file provided"})
		return
	}
	defer file.Close()

	csvDir := filepath.Join(h.cfgDir, "csv", h.instID)
	os.MkdirAll(csvDir, 0755)

	filename := filepath.Base(header.Filename)
	dst := filepath.Join(csvDir, filename)
	dstFile, err := os.Create(dst)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save file"})
		return
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, file); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to write file"})
		return
	}

	slog.Info("CSV上传成功", "instance", h.instID, "filename", filename)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"filename": filename,
		"path":     "csv/" + h.instID + "/" + filename,
	})
}

func pointToSnapshot(p *config.Point) model.PointSnapshot {
	return model.PointSnapshot{
		IOA:       p.IOA,
		Name:      p.Name,
		PointType: string(p.PointType),
		Value:     p.Value,
		BoolValue: p.BoolValue,
		IntValue:  p.IntValue,
		UpdatedAt: p.Timestamp,
		Unit:      "",
	}
}

func formatPointValueStr(p *config.Point) string {
	switch p.PointType {
	case config.TypeAI, config.TypeAO:
		return strconv.FormatFloat(p.Value, 'f', 2, 64)
	case config.TypeDI, config.TypeDO:
		if p.BoolValue {
			return "1"
		}
		return "0"
	case config.TypePI:
		return strconv.Itoa(int(p.IntValue))
	default:
		return ""
	}
}

func strategyToCode(s model.StrategyType) string {
	m := map[model.StrategyType]string{
		model.StrategyIncrement: "1",
		model.StrategyRandom:    "2",
		model.StrategyCSV:       "3",
		model.StrategyMax:       "4",
		model.StrategyMin:       "5",
		model.StrategySOC:       "6",
		model.StrategyEnergy:    "7",
		model.StrategyAOFollow:  "8",
		model.StrategyAPIUpdate: "9",
	}
	return m[s]
}

func floatToStr(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func paramsToABCDEFG(strategy model.StrategyType, p *model.StrategyParams) (a, b, c, d, e, f, g string) {
	switch strategy {
	case model.StrategyIncrement:
		return floatToStr(p.StartValue), floatToStr(p.Step),
			strconv.Itoa(p.PeriodMs), floatToStr(p.MaxValue), "", "", ""
	case model.StrategyRandom:
		return floatToStr(p.MinValue), floatToStr(p.MaxValueR),
			strconv.Itoa(p.PeriodMs), strconv.Itoa(p.DecimalPlaces), "", "", ""
	case model.StrategyMax, model.StrategyMin:
		return p.ParaA, p.ParaB, "", "", "", "", ""
	case model.StrategySOC:
		return floatToStr(p.InitSOC), floatToStr(p.RatedCap),
			strconv.FormatUint(uint64(p.PowerIOA), 10), strconv.Itoa(p.IntegralMs), "", "", ""
	case model.StrategyEnergy:
		return floatToStr(p.InitEnergy), strconv.Itoa(p.StatType),
			strconv.FormatUint(uint64(p.EnergyPowerIOA), 10), strconv.Itoa(p.EnergyPeriodMs), "", "", ""
	case model.StrategyAOFollow:
		return strconv.FormatUint(uint64(p.FollowAOIOA), 10), "", "", "", "", "", ""
	case model.StrategyAPIUpdate:
		return floatToStr(p.APIInitValue), "", "", "", "", "", ""
	}
	return "", "", "", "", "", "", ""
}

func parseFloatCol(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func parseIntCol(s string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}

func parseUint32Col(s string) uint32 {
	v, _ := strconv.ParseUint(strings.TrimSpace(s), 10, 32)
	return uint32(v)
}

func abcdefgToParams(strategy model.StrategyType, a, b, c, d, e, f, g string) model.StrategyParams {
	p := model.StrategyParams{}
	switch strategy {
	case model.StrategyIncrement:
		p.StartValue = parseFloatCol(a)
		p.Step = parseFloatCol(b)
		p.PeriodMs = parseIntCol(c)
		p.MaxValue = parseFloatCol(d)
	case model.StrategyRandom:
		p.MinValue = parseFloatCol(a)
		p.MaxValueR = parseFloatCol(b)
		p.PeriodMs = parseIntCol(c)
		p.DecimalPlaces = parseIntCol(d)
	case model.StrategyMax, model.StrategyMin:
		p.ParaA = strings.TrimSpace(a)
		p.ParaB = strings.TrimSpace(b)
	case model.StrategySOC:
		p.InitSOC = parseFloatCol(a)
		p.RatedCap = parseFloatCol(b)
		p.PowerIOA = parseUint32Col(c)
		p.IntegralMs = parseIntCol(d)
	case model.StrategyEnergy:
		p.InitEnergy = parseFloatCol(a)
		p.StatType = parseIntCol(b)
		p.EnergyPowerIOA = parseUint32Col(c)
		p.EnergyPeriodMs = parseIntCol(d)
	case model.StrategyAOFollow:
		p.FollowAOIOA = parseUint32Col(a)
	case model.StrategyAPIUpdate:
		p.APIInitValue = parseFloatCol(a)
	}
	return p
}

func codeToStrategy(code string) model.StrategyType {
	switch strings.TrimSpace(code) {
	case "1":
		return model.StrategyIncrement
	case "2":
		return model.StrategyRandom
	case "3":
		return model.StrategyCSV
	case "4":
		return model.StrategyMax
	case "5":
		return model.StrategyMin
	case "6":
		return model.StrategySOC
	case "7":
		return model.StrategyEnergy
	case "8":
		return model.StrategyAOFollow
	case "9":
		return model.StrategyAPIUpdate
	}
	// Also match by name (case-insensitive)
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "increment":
		return model.StrategyIncrement
	case "random":
		return model.StrategyRandom
	case "csv":
		return model.StrategyCSV
	case "max":
		return model.StrategyMax
	case "min":
		return model.StrategyMin
	case "soc":
		return model.StrategySOC
	case "energy":
		return model.StrategyEnergy
	case "aofollow":
		return model.StrategyAOFollow
	case "apiupdate":
		return model.StrategyAPIUpdate
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// Public wrappers for external routing (called from main.go management server)
func (h *DetailHandler) HandlePoints(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listSnapshots(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *DetailHandler) HandleGetSnapshot(w http.ResponseWriter, ioa uint32) {
	h.getSnapshot(w, ioa)
}

func (h *DetailHandler) HandleSetValue(w http.ResponseWriter, r *http.Request, ioa uint32) {
	h.setValue(w, r, ioa)
}

func (h *DetailHandler) HandleExportCSV(w http.ResponseWriter, r *http.Request) {
	h.exportCSV(w, r)
}

func (h *DetailHandler) HandleUploadCSV(w http.ResponseWriter, r *http.Request) {
	h.handleUploadCSV(w, r)
}

func (h *DetailHandler) HandleAutoChangeConfig(w http.ResponseWriter, r *http.Request, parts []string) {
	defer h.recoverPanic(w)
	h.handleAutoChangeConfig(w, r, parts)
}

func (h *DetailHandler) HandleBatchSetValue(w http.ResponseWriter, r *http.Request) {
	defer h.recoverPanic(w)
	h.batchSetValue(w, r)
}

func (h *DetailHandler) recoverPanic(w http.ResponseWriter) {
	if rec := recover(); rec != nil {
		slog.Error("panic recovered in detail handler", "instance", h.instID, "recover", rec)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}
