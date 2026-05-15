package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"strconv"

	"iec104-sim/internal/auth"
	"iec104-sim/internal/detail"
	"iec104-sim/internal/manager"
	"iec104-sim/internal/model"
	"iec104-sim/internal/storage"
	"iec104-sim/pkg/api"
	"iec104-sim/pkg/config"
	"iec104-sim/pkg/firewall"
	"iec104-sim/pkg/iec104"
	"iec104-sim/pkg/library"

	"github.com/spf13/pflag"
)

var version = "2.1.3"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		runServerMode()
	} else {
		runLegacyMode()
	}
}

// ─── Legacy Mode (backward compatible) ─────────────────────────────────────

func runLegacyMode() {
	var (
		port     int
		cfgPath  string
		httpAddr string
		logLvl   string
	)

	pflag.IntVarP(&port, "port", "p", 2404, "IEC104 TCP 端口号")
	pflag.StringVarP(&cfgPath, "config", "c", "", "配置文件路径 (.xlsx)")
	pflag.StringVarP(&httpAddr, "http", "H", ":8989", "HTTP API 监听地址")
	pflag.StringVarP(&logLvl, "log", "l", "info", "日志级别: debug/info/warn/error")
	pflag.Parse()

	if cfgPath == "" {
		slog.Error("必须指定配置文件路径 (-c)")
		os.Exit(1)
	}

	setupLogLevel(logLvl)
	slog.Info("启动模拟器 (传统模式)", "port", port, "config", cfgPath, "http", httpAddr)

	points, err := config.LoadFromXLSX(cfgPath)
	if err != nil {
		slog.Error("加载配置文件失败", "error", err)
		os.Exit(1)
	}

	counts := countByType(points)
	slog.Info("点表加载完成",
		"totalPoints", len(points),
		"AI", counts["AI"], "DI", counts["DI"],
		"PI", counts["PI"], "DO", counts["DO"], "AO", counts["AO"],
	)

	store := library.NewStore(points)
	server := iec104.NewServer(port, store)
	if err := server.Start(); err != nil {
		slog.Error("启动IEC104服务端失败", "error", err)
		os.Exit(1)
	}

	apiHandler := api.NewHandler(store, server, server)
	mux := http.NewServeMux()
	apiHandler.Register(mux)

	if p := parsePort(httpAddr); p > 0 {
		firewall.EnsurePort(p, "iec104-sim-http")
	}
	firewall.EnsurePort(port, "iec104-sim-data")

	httpSrv := &http.Server{Addr: httpAddr, Handler: mux}
	go func() {
		slog.Info("HTTP API 已启动", "addr", httpAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP 服务失败", "error", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	slog.Info("收到信号，正在关闭", "signal", sig)

	server.Stop()
	httpSrv.Close()
	slog.Info("模拟器已关闭")
}

// ─── Server Mode (web management) ──────────────────────────────────────────

type webServer struct {
	mgr     *manager.Manager
	httpSrv *http.Server
	cfgDir  string
}

func runServerMode() {
	var (
		httpAddr  string
		configDir string
		logDir    string
		logLvl    string
	)

	pflag.StringVarP(&httpAddr, "http", "H", ":8989", "管理API监听地址")
	pflag.StringVarP(&configDir, "config-dir", "c", "./config", "配置文件目录")
	pflag.StringVarP(&logDir, "log-dir", "L", "./logs", "日志文件目录")
	pflag.StringVarP(&logLvl, "log", "l", "info", "日志级别: debug/info/warn/error")
	pflag.Parse()

	setupLogLevel(logLvl)

	os.MkdirAll(configDir, 0755)
	os.MkdirAll(logDir, 0755)

	// Init storage & manager
	cfgStore := storage.NewConfigStore(filepath.Join(configDir, "instances.json"))
	if err := cfgStore.Load(); err != nil {
		slog.Warn("加载实例配置失败，使用空配置", "error", err)
	}

	mgr := manager.New(cfgStore, configDir)

	// Init auth
	authStore := auth.NewUserStore(filepath.Join(configDir, "users.json"))
	authHandler := auth.NewAuthHandler(authStore)

	// Build HTTP mux
	mux := http.NewServeMux()
	ws := &webServer{mgr: mgr, cfgDir: configDir}
	ws.registerRoutes(mux, configDir)
	authHandler.Register(mux)

	if p := parsePort(httpAddr); p > 0 {
		firewall.EnsurePort(p, "iec104-sim-http")
	}

	// Start HTTP server
	httpSrv := &http.Server{Addr: httpAddr, Handler: mux}
	go func() {
		slog.Info("管理服务已启动", "http", httpAddr, "configDir", configDir)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("管理服务启动失败", "error", err)
		}
	}()

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	slog.Info("收到信号，正在关闭所有实例", "signal", sig)

	mgr.StopAll()
	httpSrv.Close()
	slog.Info("管理服务已关闭")
}

func (ws *webServer) registerRoutes(mux *http.ServeMux, configDir string) {
	// Resolve web/dist relative to executable path.
	// IMPORTANT: 二进制位于 bin/ 目录，前端位于 ../web/dist（包根目录/web/dist）。
	// 禁止使用 ./web/dist（依赖 CWD）或 bin/web/dist（路径错误）。
	exePath, _ := os.Executable()
	webDir := filepath.Join(filepath.Dir(exePath), "..", "web", "dist")

	mux.HandleFunc("/api/v1/instances", ws.handleInstances)
	mux.HandleFunc("/api/v1/instances/", ws.handleInstanceByID)
	mux.HandleFunc("/api/v1/status", ws.handleStatus)
	mux.HandleFunc("/api/v1/upload", ws.handleUpload)
	mux.HandleFunc("/api/v1/files", ws.handleFiles)
	// Serve static frontend if built
	if _, err := os.Stat(webDir); err == nil {
		mux.Handle("/", http.FileServer(http.Dir(webDir)))
	} else {
		slog.Warn("前端构建目录不存在，Web UI 不可用", "path", webDir)
	}
}

// ─── Management API Handlers ───────────────────────────────────────────────

func (ws *webServer) handleInstances(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		states := ws.mgr.ListStates()
		result := make([]map[string]interface{}, len(states))
		for i, s := range states {
			result[i] = instanceStateToMap(s)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"instances": result})

	case http.MethodPost:
		var req model.InstanceConfig
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if err := validateConfig(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		created, err := ws.mgr.CreateConfig(req)
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, created)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (ws *webServer) handleInstanceByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/instances/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "missing instance ID")
		return
	}

	id := parts[0]

	if len(parts) >= 2 {
		switch parts[1] {
		case "start":
			ws.execAction(w, id, ws.mgr.StartInstance)
			return
		case "stop":
			ws.execAction(w, id, ws.mgr.StopInstance)
			return
		case "restart":
			ws.execAction(w, id, ws.mgr.RestartInstance)
			return
		case "points":
			ws.handleInstancePoints(w, r, id)
			return
		case "upload-csv":
			store := ws.mgr.GetStore(id)
			engine := ws.mgr.GetEngine(id)
			if store == nil || engine == nil {
				writeError(w, http.StatusNotFound, "instance not running")
				return
			}
			dh := detail.NewDetailHandler(id, store, engine, ws.mgr.CfgDir())
			dh.HandleUploadCSV(w, r)
			return
		default:
			writeError(w, http.StatusBadRequest, "unknown action: "+parts[1])
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		state, err := ws.mgr.GetState(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, instanceStateToMap(state))

	case http.MethodPut:
		var req model.InstanceConfig
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		req.ID = id
		if existing, ok := ws.mgr.GetConfig(id); ok {
			if req.Name == "" {
				req.Name = existing.Name
			}
			if req.IEC104Port == 0 {
				req.IEC104Port = existing.IEC104Port
			}
			if req.XLSXFile == "" {
				req.XLSXFile = existing.XLSXFile
			}
			if !req.HttpEnabled && req.HttpPort == 0 {
				req.HttpPort = existing.HttpPort
			}
		}
		if err := ws.mgr.UpdateConfig(req); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, req)

	case http.MethodDelete:
		if err := ws.mgr.DeleteConfig(id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type actionFunc func(string) error

func (ws *webServer) execAction(w http.ResponseWriter, id string, fn actionFunc) {
	if err := fn(id); err != nil {
		errMsg := err.Error()
		// "not found" and "not running" are 404; everything else is 409
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "not running") {
			writeError(w, http.StatusNotFound, errMsg)
		} else {
			writeError(w, http.StatusConflict, errMsg)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "id": id})
}

func (ws *webServer) handleInstancePoints(w http.ResponseWriter, r *http.Request, id string) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("panic recovered in handleInstancePoints", "instance", id, "recover", rec)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
	}()

	store := ws.mgr.GetStore(id)
	engine := ws.mgr.GetEngine(id)
	if store == nil || engine == nil {
		writeError(w, http.StatusNotFound, "instance not running")
		return
	}

	detailHandler := detail.NewDetailHandler(id, store, engine, ws.mgr.CfgDir())
	suffix := strings.TrimPrefix(r.URL.Path, "/api/v1/instances/"+id+"/points")
	if suffix == "" || suffix == "/" {
		detailHandler.HandlePoints(w, r)
		return
	}
	suffix = strings.TrimPrefix(suffix, "/")
	parts := strings.Split(suffix, "/")

	switch {
	case parts[0] == "export" && len(parts) == 1:
		detailHandler.HandleExportCSV(w, r)
	case parts[0] == "auto-change":
		detailHandler.HandleAutoChangeConfig(w, r, parts)
	case parts[0] == "batch" && r.Method == http.MethodPost:
		detailHandler.HandleBatchSetValue(w, r)
	default:
		ioa, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid IOA: "+parts[0])
			return
		}
		switch r.Method {
		case http.MethodGet:
			detailHandler.HandleGetSnapshot(w, uint32(ioa))
		case http.MethodPut:
			detailHandler.HandleSetValue(w, r, uint32(ioa))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}
}

func (ws *webServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	states := ws.mgr.ListStates()
	running := 0
	stopped := 0
	for _, s := range states {
		if s.Status == model.StatusRunning {
			running++
		} else {
			stopped++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":    version,
		"mode":       "serve",
		"configured": len(states),
		"running":    running,
		"stopped":    stopped,
		"max":        manager.MaxInstances,
	})
}

func (ws *webServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.ParseMultipartForm(32 << 20)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "no file provided: "+err.Error())
		return
	}
	defer file.Close()

	filename := ws.saveUploadedFile(file, header)
	if filename == "" {
		writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":   "uploaded",
		"filename": filename,
	})
}

func (ws *webServer) handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// 扫描 configDir 下的 .xlsx 文件
	entries, err := os.ReadDir(ws.cfgDir)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"files": []interface{}{}})
		return
	}
	files := make([]map[string]interface{}, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".xlsx") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, map[string]interface{}{
			"name":    name,
			"size":    info.Size(),
			"modtime": info.ModTime().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"files": files})
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func instanceStateToMap(s *model.InstanceState) map[string]interface{} {
	m := map[string]interface{}{
		"id":          s.Config.ID,
		"name":        s.Config.Name,
		"iec104_port": s.Config.IEC104Port,
		"xlsx_file":   s.Config.XLSXFile,
		"enabled":     s.Config.Enabled,
		"http_enabled": s.Config.HttpEnabled,
		"http_port":    s.Config.HttpPort,
		"status":      string(s.Status),
	}
	if s.Status == model.StatusRunning {
		m["stats"] = map[string]interface{}{
			"uptime_seconds":   s.UptimeSeconds,
			"total_points":     s.TotalPoints,
			"client_connected": s.ClientConnected,
			"interrogations":   s.Interrogations,
			"controls":         s.Controls,
			"spontaneous":      s.Spontaneous,
		}
	}
	if s.Error != "" {
		m["error"] = s.Error
	}
	return m
}

func validateConfig(cfg model.InstanceConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("name is required")
	}
	if cfg.IEC104Port < 1 || cfg.IEC104Port > 65535 {
		return fmt.Errorf("port must be 1-65535")
	}
	if cfg.XLSXFile == "" {
		return fmt.Errorf("xlsx_file is required")
	}
	if cfg.HttpEnabled && (cfg.HttpPort < 1 || cfg.HttpPort > 65535) {
		return fmt.Errorf("http_port must be 1-65535 when http is enabled")
	}
	return nil
}

func (ws *webServer) saveUploadedFile(file multipart.File, header *multipart.FileHeader) string {
	filename := filepath.Base(header.Filename)
	dst := filepath.Join(ws.cfgDir, filename)
	dstFile, err := os.Create(dst)
	if err != nil {
		return ""
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, file); err != nil {
		return ""
	}
	slog.Info("文件上传成功", "filename", filename, "path", dst)
	return filename
}

func setupLogLevel(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	slog.SetDefault(slog.New(h))
}

func parsePort(addr string) int {
	_, s, ok := strings.Cut(addr, ":")
	if !ok {
		return 0
	}
	p, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return p
}

func countByType(points []*config.Point) map[string]int {
	counts := map[string]int{"AI": 0, "DI": 0, "PI": 0, "DO": 0, "AO": 0}
	for _, p := range points {
		counts[string(p.PointType)]++
	}
	return counts
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
