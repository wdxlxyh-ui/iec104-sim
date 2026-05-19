package detail

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"iec104-sim/internal/model"
	"iec104-sim/pkg/config"
	"iec104-sim/pkg/library"
)

type resolvedIOA struct {
	InstanceID string
	IOA        uint32
	RawKey     string
}

type strategyRunner struct {
	store      *library.Store
	publisher  publisher
	configDir  string
	instanceID string
	provider   StoreProvider
}

func newStrategyRunner(store *library.Store, pub publisher, cfgDir, instanceID string, provider StoreProvider) *strategyRunner {
	return &strategyRunner{
		store:      store,
		publisher:  pub,
		configDir:  cfgDir,
		instanceID: instanceID,
		provider:   provider,
	}
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
	case model.StrategyCustomFormula:
		sr.doCustomFormula(cfg, state)
	case model.StrategyManual:
	}
}

type strategyState struct {
	csvRows      []csvRow
	csvIndex     int
	csvMTime     int64
	csvStartTime time.Time
	currentSOC   float64
	currentEnergy float64
}

type csvRow struct {
	delay   time.Duration
	absTime time.Time
	value   float64
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
	if !sr.ensureCSVRows(cfg, state) {
		return
	}
	if len(state.csvRows) == 0 {
		return
	}

	if state.csvStartTime.IsZero() {
		state.csvStartTime = time.Now()
		state.csvIndex = 0
	}

	elapsed := time.Since(state.csvStartTime)
	isAbsolute := strings.EqualFold(cfg.Params.TimeFormat, "absolute")

	for state.csvIndex < len(state.csvRows) {
		row := state.csvRows[state.csvIndex]
		var apply bool
		if isAbsolute {
			apply = !time.Now().Before(row.absTime)
		} else {
			apply = elapsed >= row.delay
		}
		if apply {
			p, ok := sr.store.Get(cfg.PointIOA)
			if !ok {
				return
			}
			sr.store.SetValue(cfg.PointIOA, row.value)
			sr.publisher.Publish(p)
			state.csvIndex++
		} else {
			break
		}
	}

	if state.csvIndex >= len(state.csvRows) {
		if isAbsolute {
			return
		}
		state.csvStartTime = time.Now()
		state.csvIndex = 0
	}
}

func (sr *strategyRunner) ensureCSVRows(cfg *model.AutoChangeConfig, state *strategyState) bool {
	csvPath := filepath.Join(sr.configDir, "csv", sr.instanceID, cfg.Params.CSVFileName)
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		csvPath = filepath.Join(sr.configDir, "csv", cfg.Params.CSVFileName)
	}

	if state.csvRows != nil {
		if info, err := os.Stat(csvPath); err == nil {
			if info.ModTime().Unix() == state.csvMTime {
				return true
			}
		}
	}

	state.csvRows = sr.loadCSVRows(cfg)
	if state.csvRows == nil {
		return false
	}

	if info, err := os.Stat(csvPath); err == nil {
		state.csvMTime = info.ModTime().Unix()
	}

	return true
}

func (sr *strategyRunner) loadCSVRows(cfg *model.AutoChangeConfig) []csvRow {
	// Check instance-specific directory first (upload-csv saves here)
	csvPath := filepath.Join(sr.configDir, "csv", sr.instanceID, cfg.Params.CSVFileName)
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		// Fall back to shared directory
		csvPath = filepath.Join(sr.configDir, "csv", cfg.Params.CSVFileName)
	}
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
			result = append(result, csvRow{absTime: rowTime, value: val})
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

func (sr *strategyRunner) doCustomFormula(cfg *model.AutoChangeConfig, state *strategyState) {
	ioas := parseCrossIOAList(cfg.Params.CustomIOAs)
	if len(ioas) < 2 {
		slog.Warn("自定义公式: 关联测点不足2个，跳过", "ioa", cfg.PointIOA)
		return
	}

	// Collect all values from local and cross-instance stores
	values := make(map[string]float64)
	hasFailure := false

	for _, ri := range ioas {
		var p *config.Point
		var ok bool

		if ri.InstanceID == "" || ri.InstanceID == sr.instanceID {
			p, ok = sr.store.Get(ri.IOA)
		} else {
			remoteStore := sr.provider.GetStore(ri.InstanceID)
			if remoteStore != nil {
				p, ok = remoteStore.Get(ri.IOA)
			} else {
				slog.Warn("自定义公式: 跨实例Store不可用",
					"targetIOA", cfg.PointIOA,
					"remoteInstance", ri.InstanceID,
					"remoteIOA", ri.IOA)
			}
		}
		if !ok {
			slog.Warn("自定义公式: 测点不可用，跳过本次计算",
				"targetIOA", cfg.PointIOA,
				"ref", ri.RawKey)
			hasFailure = true
			break
		}
		values[ri.RawKey] = p.Value
	}
	if hasFailure {
		return
	}

	formula := cfg.Params.CustomFormula
	for idx, ri := range ioas {
		placeholder := fmt.Sprintf("{%d}", idx)
		formula = strings.ReplaceAll(formula, placeholder, fmt.Sprintf("%f", values[ri.RawKey]))
	}

	result, err := evaluateExpression(formula)
	if err != nil {
		slog.Warn("自定义公式计算失败", "formula", formula, "error", err)
		return
	}
	p, ok := sr.store.Get(cfg.PointIOA)
	if !ok {
		return
	}
	sr.store.SetValue(cfg.PointIOA, result)
	sr.publisher.Publish(p)
}

// parseCrossIOAList 解析跨实例测点列表。
// 格式: "16385;inst-abc:30001;inst-def:40005"
// 本实例用 "" 表示（空 instanceID），如 "16385"。
func parseCrossIOAList(s string) []resolvedIOA {
	parts := strings.Split(s, ";")
	var result []resolvedIOA
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if idx := strings.Index(p, ":"); idx > 0 {
			instID := p[:idx]
			ioaStr := p[idx+1:]
			ioa, err := strconv.ParseUint(strings.TrimSpace(ioaStr), 10, 32)
			if err != nil || ioa == 0 {
				continue
			}
			result = append(result, resolvedIOA{
				InstanceID: instID,
				IOA:        uint32(ioa),
				RawKey:     p,
			})
		} else {
			ioa, err := strconv.ParseUint(p, 10, 32)
			if err != nil || ioa == 0 {
				continue
			}
			result = append(result, resolvedIOA{
				InstanceID: "",
				IOA:        uint32(ioa),
				RawKey:     p,
			})
		}
	}
	return result
}

// evaluateExpression evaluates a simple arithmetic expression with + - * / ( )
func evaluateExpression(expr string) (float64, error) {
	// Remove spaces
	expr = strings.ReplaceAll(expr, " ", "")
	if expr == "" {
		return 0, fmt.Errorf("empty expression")
	}
	result, pos, err := parseExpr(expr, 0)
	if err != nil {
		return 0, err
	}
	if pos < len(expr) {
		return 0, fmt.Errorf("unexpected character at position %d", pos)
	}
	return result, nil
}

func parseExpr(s string, pos int) (float64, int, error) {
	left, pos, err := parseTerm(s, pos)
	if err != nil {
		return 0, pos, err
	}
	for pos < len(s) && (s[pos] == '+' || s[pos] == '-') {
		op := s[pos]
		pos++
		right, newPos, err := parseTerm(s, pos)
		if err != nil {
			return 0, newPos, err
		}
		if op == '+' {
			left += right
		} else {
			left -= right
		}
		pos = newPos
	}
	return left, pos, nil
}

func parseTerm(s string, pos int) (float64, int, error) {
	left, pos, err := parseFactor(s, pos)
	if err != nil {
		return 0, pos, err
	}
	for pos < len(s) && (s[pos] == '*' || s[pos] == '/') {
		op := s[pos]
		pos++
		right, newPos, err := parseFactor(s, pos)
		if err != nil {
			return 0, newPos, err
		}
		if op == '*' {
			left *= right
		} else {
			if right == 0 {
				return 0, newPos, fmt.Errorf("division by zero")
			}
			left /= right
		}
		pos = newPos
	}
	return left, pos, nil
}

func parseFactor(s string, pos int) (float64, int, error) {
	if pos >= len(s) {
		return 0, pos, fmt.Errorf("unexpected end of expression")
	}
	if s[pos] == '(' {
		pos++ // skip '('
		val, pos, err := parseExpr(s, pos)
		if err != nil {
			return 0, pos, err
		}
		if pos >= len(s) || s[pos] != ')' {
			return 0, pos, fmt.Errorf("missing closing parenthesis")
		}
		pos++ // skip ')'
		return val, pos, nil
	}
	// Parse number: [-]digits[.digits]
	negative := false
	if s[pos] == '-' {
		negative = true
		pos++
	} else if s[pos] == '+' {
		pos++
	}
	start := pos
	for pos < len(s) && (isDigit(s[pos]) || s[pos] == '.') {
		pos++
	}
	if pos == start {
		return 0, pos, fmt.Errorf("expected number at position %d", start)
	}
	numStr := s[start:pos]
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, pos, fmt.Errorf("invalid number: %s", numStr)
	}
	if negative {
		val = -val
	}
	return val, pos, nil
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
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
