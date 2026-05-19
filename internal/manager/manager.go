package manager

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"iec104-sim/internal/detail"
	"iec104-sim/internal/model"
	"iec104-sim/internal/storage"
	"iec104-sim/pkg/api"
	"iec104-sim/pkg/config"
	"iec104-sim/pkg/firewall"
	"iec104-sim/pkg/library"
	"iec104-sim/pkg/protocol"
)

func generateID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Instance wraps a running protocol server instance.
type Instance struct {
	Config     model.InstanceConfig
	Protocol   protocol.Protocol
	Store      *library.Store
	HTTPServer *http.Server
	AutoEngine *detail.Engine
	Logger     *InstanceLogger
}

// MaxInstances is the maximum number of concurrent instances allowed.
// Set to 0 for unlimited (disabled check)
const MaxInstances = 1000

// Manager manages multiple IEC104 server instances.
type Manager struct {
	mu        sync.RWMutex
	instances map[string]*Instance
	store     *storage.ConfigStore
	cfgDir    string
}

// New creates a new Manager.
func New(store *storage.ConfigStore, cfgDir string) *Manager {
	return &Manager{
		instances: make(map[string]*Instance),
		store:     store,
		cfgDir:    cfgDir,
	}
}

// Store returns the underlying ConfigStore.
func (m *Manager) Store() *storage.ConfigStore {
	return m.store
}

// ListConfigs returns all instance configurations.
func (m *Manager) ListConfigs() []model.InstanceConfig {
	return m.store.List()
}

// GetConfig returns an instance configuration by ID.
func (m *Manager) GetConfig(id string) (model.InstanceConfig, bool) {
	return m.store.Get(id)
}

// CreateConfig creates a new instance configuration with an auto-generated ID.
// Returns the created config with the assigned ID.
func (m *Manager) CreateConfig(cfg model.InstanceConfig) (model.InstanceConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if MaxInstances > 0 && m.store.Count() >= MaxInstances {
		return model.InstanceConfig{}, fmt.Errorf("maximum %d instances allowed", MaxInstances)
	}

	// Check IEC104 port conflict with other configs
	for _, existing := range m.store.List() {
		if existing.IEC104Port == cfg.IEC104Port {
			return model.InstanceConfig{}, fmt.Errorf("port %d already configured for instance %s", cfg.IEC104Port, existing.ID)
		}
		if cfg.HttpEnabled && existing.HttpEnabled && existing.HttpPort == cfg.HttpPort {
			return model.InstanceConfig{}, fmt.Errorf("http port %d already configured for instance %s", cfg.HttpPort, existing.ID)
		}
	}

	// Auto-generate ID if not provided by client
	if cfg.ID == "" {
		cfg.ID = generateID()
	}

	if err := m.store.Add(cfg); err != nil {
		return model.InstanceConfig{}, err
	}
	return cfg, nil
}

func (m *Manager) StartInstance(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.instances[id]; ok {
		return fmt.Errorf("instance %s already running", id)
	}

	cfg, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	for _, inst := range m.instances {
		if inst.Config.IEC104Port == cfg.IEC104Port {
			return fmt.Errorf("port %d already in use by instance %s", cfg.IEC104Port, inst.Config.ID)
		}
		if cfg.HttpEnabled && inst.Config.HttpEnabled && inst.Config.HttpPort == cfg.HttpPort {
			return fmt.Errorf("http port %d already in use by instance %s", cfg.HttpPort, inst.Config.ID)
		}
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.IEC104Port))
	if err != nil {
		return fmt.Errorf("port %d not available: %w", cfg.IEC104Port, err)
	}
	ln.Close()

	if cfg.HttpEnabled && cfg.HttpPort > 0 {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.HttpPort))
		if err != nil {
			return fmt.Errorf("http port %d not available: %w", cfg.HttpPort, err)
		}
		ln.Close()
	}

	xlsxPath := cfg.XLSXFile
	if !filepath.IsAbs(xlsxPath) {
		if _, err := os.Stat(xlsxPath); os.IsNotExist(err) {
			xlsxPath = filepath.Join(m.cfgDir, xlsxPath)
		}
	}

	points, err := config.LoadFromXLSX(xlsxPath, cfg.Protocol)
	if err != nil {
		return fmt.Errorf("load xlsx: %w", err)
	}

	store := library.NewStore(points)

	proto, err := protocol.New(cfg)
	if err != nil {
		return fmt.Errorf("create protocol: %w", err)
	}
	proto.SetStore(store)
	if err := proto.Start(); err != nil {
		return fmt.Errorf("start protocol: %w", err)
	}

	acStore := detail.NewAutoChangeStore(m.cfgDir)
	engine := detail.NewEngine(cfg.ID, store, proto, acStore, m.cfgDir, m)
	if p, ok := proto.(interface{ SetAOFollowHandler(func(aoIOA uint32)) }); ok {
		p.SetAOFollowHandler(engine.HandleAOFollow)
	}
	if err := engine.LoadAndStart(); err != nil {
		slog.Warn("自动变化引擎加载失败", "id", id, "error", err)
	}

	logger, err := NewInstanceLogger(m.cfgDir, cfg.ID, cfg.IEC104Port)
	if err != nil {
		slog.Warn("创建实例日志目录失败", "id", id, "error", err)
	}

	inst := &Instance{
		Config:     cfg,
		Protocol:   proto,
		Store:      store,
		AutoEngine: engine,
		Logger:     logger,
	}

	if cfg.HttpEnabled && cfg.HttpPort > 0 {
		apiHandler := api.NewHandler(store, proto, proto)
		detailHandler := detail.NewDetailHandler(cfg.ID, store, engine, m.cfgDir)
		httpMux := http.NewServeMux()
		apiHandler.Register(httpMux)
		detailHandler.Register(httpMux)
		httpSrv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.HttpPort), Handler: httpMux}
		go func() {
			slog.Info("实例HTTP API已启动", "id", id, "port", cfg.HttpPort)
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("实例HTTP API失败", "id", id, "error", err)
			}
		}()
		inst.HTTPServer = httpSrv
	}

	firewall.EnsurePort(cfg.IEC104Port, "iec104-sim-instance")
	if cfg.HttpEnabled && cfg.HttpPort > 0 {
		firewall.EnsurePort(cfg.HttpPort, "iec104-sim-instance")
	}

	m.instances[id] = inst

	slog.Info("实例已启动", "id", id, "port", cfg.IEC104Port, "name", cfg.Name, "points", len(points))
	return nil
}

// UpdateConfig updates an instance configuration, stopping it if running.
func (m *Manager) UpdateConfig(cfg model.InstanceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if inst, ok := m.instances[cfg.ID]; ok {
		oldPort := inst.Config.IEC104Port
		inst.Protocol.Stop()
		if inst.HTTPServer != nil {
			inst.HTTPServer.Close()
			inst.HTTPServer = nil
		}
		if inst.Logger != nil {
			inst.Logger.Close()
			if oldPort != cfg.IEC104Port {
				RenameInstanceLogDir(m.cfgDir, cfg.ID, cfg.ID, oldPort, cfg.IEC104Port)
			}
		}
		delete(m.instances, cfg.ID)
	}

	return m.store.Update(cfg)
}

func (m *Manager) DeleteConfig(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if inst, ok := m.instances[id]; ok {
		inst.Protocol.Stop()
		if inst.AutoEngine != nil {
			inst.AutoEngine.StopAll()
		}
		if inst.HTTPServer != nil {
			inst.HTTPServer.Close()
			inst.HTTPServer = nil
		}
		if inst.Logger != nil {
			inst.Logger.Close()
			RemoveInstanceLogDir(m.cfgDir, inst.Config.ID, inst.Config.IEC104Port)
		}
		firewall.RemovePort(inst.Config.IEC104Port)
		if inst.Config.HttpEnabled && inst.Config.HttpPort > 0 {
			firewall.RemovePort(inst.Config.HttpPort)
		}
		delete(m.instances, id)
	} else {
		cfg, _ := m.store.Get(id)
		if cfg.ID != "" {
			RemoveInstanceLogDir(m.cfgDir, cfg.ID, cfg.IEC104Port)
		}
	}

	return m.store.Delete(id)
}

func (m *Manager) StopInstance(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("instance %s not running", id)
	}

	inst.Protocol.Stop()
	if inst.AutoEngine != nil {
		inst.AutoEngine.StopAll()
	}
	if inst.HTTPServer != nil {
		inst.HTTPServer.Close()
		inst.HTTPServer = nil
	}

	if inst.Logger != nil {
		inst.Logger.Close()
		RemoveInstanceLogDir(m.cfgDir, inst.Config.ID, inst.Config.IEC104Port)
	}

	firewall.RemovePort(inst.Config.IEC104Port)
	if inst.Config.HttpEnabled && inst.Config.HttpPort > 0 {
		firewall.RemovePort(inst.Config.HttpPort)
	}

	delete(m.instances, id)

	slog.Info("实例已停止", "id", id, "name", inst.Config.Name)
	return nil
}

func (m *Manager) CfgDir() string {
	return m.cfgDir
}

func (m *Manager) GetStore(id string) *library.Store {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if inst, ok := m.instances[id]; ok {
		return inst.Store
	}
	return nil
}

func (m *Manager) GetEngine(id string) *detail.Engine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if inst, ok := m.instances[id]; ok {
		return inst.AutoEngine
	}
	return nil
}

// RestartInstance stops and starts an instance.
// If the instance is not running, it is started directly.
func (m *Manager) RestartInstance(id string) error {
	if err := m.StopInstance(id); err != nil {
		// If the instance is not running, just start it
		if strings.Contains(err.Error(), "not running") {
			return m.StartInstance(id)
		}
		return err
	}
	return m.StartInstance(id)
}

// GetState returns the runtime state of an instance.
func (m *Manager) GetState(id string) (*model.InstanceState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg, ok := m.store.Get(id)
	if !ok {
		return nil, fmt.Errorf("instance %s not found", id)
	}

	state := &model.InstanceState{
		Config: cfg,
		Status: model.StatusStopped,
	}

	if inst, running := m.instances[id]; running && inst.Protocol != nil {
		state.Status = model.StatusRunning
		state.UptimeSeconds = inst.Protocol.Uptime()
		state.TotalPoints = inst.Store.TotalCount()
		state.ClientConnected = inst.Protocol.ClientConnected()
		interrog, control, spont := inst.Protocol.Stats()
		state.Interrogations = interrog
		state.Controls = control
		state.Spontaneous = spont
	}

	return state, nil
}

// ListStates returns runtime states for all configured instances.
func (m *Manager) ListStates() []*model.InstanceState {
	cfgs := m.store.List()
	states := make([]*model.InstanceState, 0, len(cfgs))

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, cfg := range cfgs {
		state := &model.InstanceState{
			Config: cfg,
			Status: model.StatusStopped,
		}
		if inst, ok := m.instances[cfg.ID]; ok && inst.Protocol != nil {
			state.Status = model.StatusRunning
			state.UptimeSeconds = inst.Protocol.Uptime()
			state.TotalPoints = inst.Store.TotalCount()
			state.ClientConnected = inst.Protocol.ClientConnected()
			interrog, control, spont := inst.Protocol.Stats()
			state.Interrogations = interrog
			state.Controls = control
			state.Spontaneous = spont
		}
		states = append(states, state)
	}

	return states
}

// RunningCount returns the number of running instances.
func (m *Manager) RunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.instances)
}

// StopAll stops all running instances.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, inst := range m.instances {
		inst.Protocol.Stop()
		if inst.AutoEngine != nil {
			inst.AutoEngine.StopAll()
		}
		delete(m.instances, id)
		slog.Info("实例已停止", "id", id, "name", inst.Config.Name)
	}
}
