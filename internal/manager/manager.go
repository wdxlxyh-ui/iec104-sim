package manager

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"iec104-sim/internal/model"
	"iec104-sim/internal/storage"
	"iec104-sim/pkg/config"
	"iec104-sim/pkg/iec104"
	"iec104-sim/pkg/library"
)

func generateID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Instance wraps a running IEC104 server instance.
type Instance struct {
	Config model.InstanceConfig
	Server *iec104.Server
	Store  *library.Store
}

// MaxInstances is the maximum number of concurrent instances allowed.
const MaxInstances = 10

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

	if m.store.Count() >= MaxInstances {
		return model.InstanceConfig{}, fmt.Errorf("maximum %d instances allowed", MaxInstances)
	}

	// Check port conflict with other configs
	for _, existing := range m.store.List() {
		if existing.IEC104Port == cfg.IEC104Port {
			return model.InstanceConfig{}, fmt.Errorf("port %d already configured for instance %s", cfg.IEC104Port, existing.ID)
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

// UpdateConfig updates an instance configuration, stopping it if running.
func (m *Manager) UpdateConfig(cfg model.InstanceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop if running
	if inst, ok := m.instances[cfg.ID]; ok {
		inst.Server.Stop()
		delete(m.instances, cfg.ID)
	}

	return m.store.Update(cfg)
}

// DeleteConfig deletes an instance configuration, stopping it if running.
func (m *Manager) DeleteConfig(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if inst, ok := m.instances[id]; ok {
		inst.Server.Stop()
		delete(m.instances, id)
	}

	return m.store.Delete(id)
}

// StartInstance starts an IEC104 server for the given instance config.
func (m *Manager) StartInstance(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Already running
	if _, ok := m.instances[id]; ok {
		return fmt.Errorf("instance %s already running", id)
	}

	cfg, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	// Check port availability against running instances
	for _, inst := range m.instances {
		if inst.Config.IEC104Port == cfg.IEC104Port {
			return fmt.Errorf("port %d already in use by instance %s", cfg.IEC104Port, inst.Config.ID)
		}
	}

	// Quick TCP port check
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.IEC104Port))
	if err != nil {
		return fmt.Errorf("port %d not available: %w", cfg.IEC104Port, err)
	}
	ln.Close()

	// Resolve xlsx path:
	//   1. Try as-is (relative to CWD / package root) → e.g. samples/point.xlsx
	//   2. Fall back to config directory          → e.g. config/uploaded.xlsx
	xlsxPath := cfg.XLSXFile
	if !filepath.IsAbs(xlsxPath) {
		if _, err := os.Stat(xlsxPath); os.IsNotExist(err) {
			xlsxPath = filepath.Join(m.cfgDir, xlsxPath)
		}
	}

	// Load point table
	points, err := config.LoadFromXLSX(xlsxPath)
	if err != nil {
		return fmt.Errorf("load xlsx: %w", err)
	}

	store := library.NewStore(points)
	server := iec104.NewServer(cfg.IEC104Port, store)
	if err := server.Start(); err != nil {
		return fmt.Errorf("start iec104 server: %w", err)
	}

	m.instances[id] = &Instance{
		Config: cfg,
		Server: server,
		Store:  store,
	}

	slog.Info("实例已启动", "id", id, "port", cfg.IEC104Port, "name", cfg.Name, "points", len(points))
	return nil
}

// StopInstance stops a running IEC104 server.
func (m *Manager) StopInstance(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("instance %s not running", id)
	}

	inst.Server.Stop()
	delete(m.instances, id)

	slog.Info("实例已停止", "id", id, "name", inst.Config.Name)
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

	if inst, running := m.instances[id]; running && inst.Server != nil {
		state.Status = model.StatusRunning
		state.UptimeSeconds = inst.Server.Uptime()
		state.TotalPoints = inst.Store.TotalCount()
		state.ClientConnected = inst.Server.ClientConnected()
		interrog, control, spont := inst.Server.Stats()
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
		if inst, ok := m.instances[cfg.ID]; ok && inst.Server != nil {
			state.Status = model.StatusRunning
			state.UptimeSeconds = inst.Server.Uptime()
			state.TotalPoints = inst.Store.TotalCount()
			state.ClientConnected = inst.Server.ClientConnected()
			interrog, control, spont := inst.Server.Stats()
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
		inst.Server.Stop()
		delete(m.instances, id)
		slog.Info("实例已停止", "id", id, "name", inst.Config.Name)
	}
}
