package detail

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"iec104-sim/internal/model"
	"iec104-sim/pkg/config"
	"iec104-sim/pkg/library"
)

type publisher interface {
	Publish(point *config.Point)
}

const maxConcurrentTasks = 100

type Engine struct {
	mu       sync.RWMutex
	store    *library.Store
	pub      publisher
	strategy *strategyRunner
	acStore  *AutoChangeStore
	tasks    map[uint32]*changeTask
	state    map[uint32]*strategyState
	instanceID string
	cfgDir   string
	wg       sync.WaitGroup
}

type changeTask struct {
	cancel context.CancelFunc
}

func NewEngine(instanceID string, store *library.Store, pub publisher, acStore *AutoChangeStore, cfgDir string, provider StoreProvider) *Engine {
	return &Engine{
		store:      store,
		pub:        pub,
		strategy:   newStrategyRunner(store, pub, cfgDir, instanceID, provider),
		acStore:    acStore,
		tasks:      make(map[uint32]*changeTask),
		state:      make(map[uint32]*strategyState),
		instanceID: instanceID,
		cfgDir:     cfgDir,
	}
}

func (e *Engine) LoadAndStart() error {
	configs, err := e.acStore.Load(e.instanceID)
	if err != nil {
		return err
	}
	for _, cfg := range configs {
		if cfg.Enabled {
			e.startTask(cfg)
		}
	}
	return nil
}

func (e *Engine) StartOrUpdate(cfg *model.AutoChangeConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stopTaskLocked(cfg.PointIOA)

	if err := e.acStore.Set(e.instanceID, cfg); err != nil {
		return err
	}

	if cfg.Enabled {
		e.startTaskLocked(cfg)
	}
	return nil
}

func (e *Engine) Remove(ioa uint32) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stopTaskLocked(ioa)
	return e.acStore.Delete(e.instanceID, ioa)
}

func (e *Engine) StopAll() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for ioa, task := range e.tasks {
		task.cancel()
		delete(e.tasks, ioa)
		delete(e.state, ioa)
	}
	e.wg.Wait()
}

func (e *Engine) IsRunning(ioa uint32) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.tasks[ioa]
	return ok
}

func (e *Engine) GetConfig(ioa uint32) (*model.AutoChangeConfig, bool) {
	return e.acStore.Get(e.instanceID, ioa)
}

func (e *Engine) AllConfigs() map[uint32]*model.AutoChangeConfig {
	return e.acStore.All(e.instanceID)
}

func (e *Engine) SaveAll(configs map[uint32]*model.AutoChangeConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for ioa, task := range e.tasks {
		task.cancel()
		delete(e.tasks, ioa)
		delete(e.state, ioa)
	}
	e.wg.Wait()
	for _, cfg := range configs {
		if cfg.Enabled {
			e.startTaskLocked(cfg)
		}
	}
	return e.acStore.Save(e.instanceID, configs)
}

func (e *Engine) CheckAPIWriteAllowed(ioa uint32) bool {
	cfg, ok := e.acStore.Get(e.instanceID, ioa)
	if !ok {
		return true
	}
	if !cfg.Enabled {
		return true
	}
	return cfg.Strategy == model.StrategyAPIUpdate || cfg.Strategy == model.StrategyManual
}

func (e *Engine) startTask(cfg *model.AutoChangeConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.startTaskLocked(cfg)
}

func (e *Engine) startTaskLocked(cfg *model.AutoChangeConfig) {
	if cfg.Strategy == model.StrategyAOFollow || cfg.Strategy == model.StrategyAPIUpdate || cfg.Strategy == model.StrategyManual {
		return
	}

	if len(e.tasks) >= maxConcurrentTasks {
		slog.Error("已达到最大并发任务数", "max", maxConcurrentTasks, "current", len(e.tasks))
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	task := &changeTask{
		cancel: cancel,
	}

	state := &strategyState{}
	if cfg.Strategy == model.StrategySOC {
		state.currentSOC = cfg.Params.InitSOC
	}
	if cfg.Strategy == model.StrategyEnergy {
		state.currentEnergy = cfg.Params.InitEnergy
	}
	if cfg.Strategy == model.StrategyCSV {
		state.csvRows = e.strategy.loadCSVRows(cfg)
	}

	period := time.Duration(cfg.Params.PeriodMs) * time.Millisecond
	if period < 100*time.Millisecond {
		period = 100 * time.Millisecond
	}

	e.tasks[cfg.PointIOA] = task
	e.state[cfg.PointIOA] = state
	e.wg.Add(1)

	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(period)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.mu.RLock()
				s := e.state[cfg.PointIOA]
				e.mu.RUnlock()
				e.strategy.runOnce(cfg, s)
			}
		}
	}()

	slog.Info("自动变化任务已启动", "ioa", cfg.PointIOA, "strategy", cfg.Strategy, "period_ms", cfg.Params.PeriodMs)
}

func (e *Engine) stopTaskLocked(ioa uint32) {
	task, ok := e.tasks[ioa]
	if !ok {
		return
	}
	task.cancel()
	delete(e.tasks, ioa)
	delete(e.state, ioa)
	slog.Info("自动变化任务已停止", "ioa", ioa)
}

func (e *Engine) HandleAOFollow(aoIOA uint32) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, cfg := range e.acStore.All(e.instanceID) {
		if cfg.Strategy == model.StrategyAOFollow && cfg.Enabled && cfg.Params.FollowAOIOA == aoIOA {
			p, ok := e.store.Get(aoIOA)
			if !ok {
				continue
			}
			e.store.SetValue(cfg.PointIOA, p.Value)
			if target, ok := e.store.Get(cfg.PointIOA); ok {
				e.pub.Publish(target)
			}
		}
	}
}

func IsAODO(pt config.PointType) bool {
	return pt == config.TypeAO || pt == config.TypeDO
}

func IsAIDIPI(pt config.PointType) bool {
	return pt == config.TypeAI || pt == config.TypeDI || pt == config.TypePI
}

func IsSetValueAllowed(pt config.PointType) bool {
	return pt == config.TypeAI || pt == config.TypeDI || pt == config.TypePI
}

func IsAutoChangeAllowed(pt config.PointType) bool {
	return pt == config.TypeAI || pt == config.TypeDI || pt == config.TypePI
}

func IsDI(pt config.PointType) bool {
	return pt == config.TypeDI
}

func FormatTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.000")
}

func FilenameSafeName(name string) string {
	r := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_")
	return r.Replace(name)
}
