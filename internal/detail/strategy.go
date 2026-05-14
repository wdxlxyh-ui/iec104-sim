package detail

import (
	"encoding/csv"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"iec104-sim/internal/model"
	"iec104-sim/pkg/library"
)

type strategyRunner struct {
	store      *library.Store
	publisher  publisher
	configDir  string
}

func newStrategyRunner(store *library.Store, pub publisher, cfgDir string) *strategyRunner {
	return &strategyRunner{store: store, publisher: pub, configDir: cfgDir}
}

func (sr *strategyRunner) runOnce(cfg *model.AutoChangeConfig, state *strategyState) {
	switch cfg.Strategy {
	case model.StrategyIncrement:
		sr.doIncrement(cfg, state)
	case model.StrategyRandom:
		sr.doRandom(cfg)
	case model.StrategyCSV:
		sr.doCSV(cfg, state)
	case model.StrategyMax:
		sr.doMaxMin(cfg, true)
	case model.StrategyMin:
		sr.doMaxMin(cfg, false)
	case model.StrategySOC:
		sr.doSOC(cfg, state)
	case model.StrategyEnergy:
		sr.doEnergy(cfg, state)
	case model.StrategyManual:
		// manual 策略不做自动计算，值由用户 API 置数
	}
}

type strategyState struct {
	csvRows      []csvRow
	csvIndex     int
	currentSOC   float64
	currentEnergy float64
}

type csvRow struct {
	delay time.Duration
	value float64
}

func (sr *strategyRunner) doIncrement(cfg *model.AutoChangeConfig, state *strategyState) {
	p, ok := sr.store.Get(cfg.PointIOA)
	if !ok {
		return
	}
	next := p.Value + cfg.Params.Step
	if cfg.Params.MaxValue > 0 && next > cfg.Params.MaxValue {
		next = cfg.Params.StartValue
	}
	sr.store.SetValue(cfg.PointIOA, next)
	sr.publisher.Publish(p)
}

func (sr *strategyRunner) doRandom(cfg *model.AutoChangeConfig) {
	minV := cfg.Params.MinValue
	maxV := cfg.Params.MaxValueR
	if maxV <= minV {
		maxV = minV + 1
	}
	val := minV + rand.Float64()*(maxV-minV)
	if cfg.Params.DecimalPlaces == 1 {
		val = math.Round(val*10) / 10
	} else {
		val = math.Round(val)
	}
	p, ok := sr.store.Get(cfg.PointIOA)
	if !ok {
		return
	}
	sr.store.SetValue(cfg.PointIOA, val)
	sr.publisher.Publish(p)
}

func (sr *strategyRunner) doCSV(cfg *model.AutoChangeConfig, state *strategyState) {
	if len(state.csvRows) == 0 {
		return
	}
	if state.csvIndex >= len(state.csvRows) {
		if strings.EqualFold(cfg.Params.TimeFormat, "absolute") {
			return
		}
		state.csvIndex = 0
	}
	row := state.csvRows[state.csvIndex]
	state.csvIndex++
	if state.csvIndex >= len(state.csvRows) && strings.EqualFold(cfg.Params.TimeFormat, "absolute") {
		state.csvIndex = len(state.csvRows)
	}
	p, ok := sr.store.Get(cfg.PointIOA)
	if !ok {
		return
	}
	sr.store.SetValue(cfg.PointIOA, row.value)
	sr.publisher.Publish(p)
}

func (sr *strategyRunner) loadCSVRows(cfg *model.AutoChangeConfig) []csvRow {
	csvPath := filepath.Join(sr.configDir, "csv", cfg.Params.CSVFileName)
	f, err := os.Open(csvPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil || len(rows) < 2 {
		return nil
	}

	var result []csvRow
	if strings.EqualFold(cfg.Params.TimeFormat, "absolute") {
		now := time.Now()
		startIdx := -1
		for i := 1; i < len(rows); i++ {
			if len(rows[i]) < 2 {
				continue
			}
			t, err := time.Parse("15:04:05", strings.TrimSpace(rows[i][0]))
			if err != nil {
				continue
			}
			rowTime := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, now.Location())
			val, err := strconv.ParseFloat(strings.TrimSpace(rows[i][1]), 64)
			if err != nil {
				continue
			}
			result = append(result, csvRow{value: val})
			if startIdx < 0 && !rowTime.Before(now) {
				startIdx = len(result) - 1
			}
		}
		if startIdx > 0 {
			result = append(result[startIdx:], result[:startIdx]...)
		}
	} else {
		for i := 1; i < len(rows); i++ {
			if len(rows[i]) < 2 {
				continue
			}
			delayStr := strings.TrimSpace(rows[i][0])
			val, err := strconv.ParseFloat(strings.TrimSpace(rows[i][1]), 64)
			if err != nil {
				continue
			}
			d, err := strconv.Atoi(delayStr)
			if err != nil {
				continue
			}
			unit := time.Millisecond
			if strings.EqualFold(cfg.Params.TimeUnit, "s") {
				unit = time.Second
			}
			result = append(result, csvRow{delay: time.Duration(d) * unit, value: val})
		}
	}
	return result
}

func (sr *strategyRunner) doMaxMin(cfg *model.AutoChangeConfig, isMax bool) {
	ioas := parseIOAList(cfg.Params.ParaA)
	if len(ioas) == 0 {
		return
	}
	var values []float64
	for _, ioa := range ioas {
		if p, ok := sr.store.Get(ioa); ok {
			values = append(values, p.Value)
		}
	}
	if len(values) == 0 {
		return
	}
	result := values[0]
	for _, v := range values[1:] {
		if isMax {
			if v > result {
				result = v
			}
		} else {
			if v < result {
				result = v
			}
		}
	}
	if cfg.Params.ParaB != "" {
		if b, err := strconv.ParseUint(cfg.Params.ParaB, 10, 32); err == nil {
			if p, ok := sr.store.Get(uint32(b)); ok {
				if !p.BoolValue {
					result = 0
				}
			}
		}
	}
	p, ok := sr.store.Get(cfg.PointIOA)
	if !ok {
		return
	}
	sr.store.SetValue(cfg.PointIOA, result)
	sr.publisher.Publish(p)
}

func (sr *strategyRunner) doSOC(cfg *model.AutoChangeConfig, state *strategyState) {
	if cfg.Params.RatedCap <= 0 {
		return
	}
	p, ok := sr.store.Get(cfg.Params.PowerIOA)
	if !ok {
		return
	}
	T := float64(cfg.Params.IntegralMs) / 1000.0
	deltaSOC := p.Value * T / cfg.Params.RatedCap * 100
	state.currentSOC += deltaSOC
	if state.currentSOC > 100 {
		state.currentSOC = 100
	}
	if state.currentSOC < 0 {
		state.currentSOC = 0
	}
	target, ok := sr.store.Get(cfg.PointIOA)
	if !ok {
		return
	}
	sr.store.SetValue(cfg.PointIOA, state.currentSOC)
	sr.publisher.Publish(target)
}

func (sr *strategyRunner) doEnergy(cfg *model.AutoChangeConfig, state *strategyState) {
	p, ok := sr.store.Get(cfg.Params.EnergyPowerIOA)
	if !ok {
		return
	}
	T := float64(cfg.Params.EnergyPeriodMs) / 3600000.0
	if cfg.Params.StatType == 0 && p.Value > 0 {
		state.currentEnergy += p.Value * T
	} else if cfg.Params.StatType == 1 && p.Value < 0 {
		state.currentEnergy += math.Abs(p.Value) * T
	}
	target, ok := sr.store.Get(cfg.PointIOA)
	if !ok {
		return
	}
	sr.store.SetValue(cfg.PointIOA, state.currentEnergy)
	sr.publisher.Publish(target)
}

func parseIOAList(s string) []uint32 {
	parts := strings.Split(s, ";")
	var result []uint32
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			continue
		}
		result = append(result, uint32(v))
	}
	return result
}
