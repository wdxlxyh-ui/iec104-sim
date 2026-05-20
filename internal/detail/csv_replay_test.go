package detail

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"iec104-sim/internal/model"
	"iec104-sim/pkg/config"
	"iec104-sim/pkg/library"
)

type mockPublisher struct {
	published []*config.Point
}

func (m *mockPublisher) Publish(p *config.Point) {
	m.published = append(m.published, p)
}

func setupCSVTestEnv(t *testing.T, csvContent string, csvFilename string) (*strategyRunner, *model.AutoChangeConfig, *strategyState, *library.Store) {
	t.Helper()

	tmpDir := t.TempDir()
	csvDir := filepath.Join(tmpDir, "csv")
	os.MkdirAll(csvDir, 0755)
	csvPath := filepath.Join(csvDir, csvFilename)
	os.WriteFile(csvPath, []byte(csvContent), 0644)

	point := &config.Point{
		IOA:       1,
		Name:      "TestPoint",
		ValueType: config.VTFloat,
		PointType: config.TypeAI,
		Value:     0,
		Efficient: 1,
		BaseValue: 0,
	}
	store := library.NewStore([]*config.Point{point})
	pub := &mockPublisher{}

	runner := newStrategyRunner(store, pub, tmpDir, "test-inst", nil)

	cfg := &model.AutoChangeConfig{
		PointIOA: 1,
		Strategy: model.StrategyCSV,
		Enabled:  true,
		Params: model.StrategyParams{
			CSVFileName: csvFilename,
		},
	}
	state := &strategyState{}

	return runner, cfg, state, store
}

func setupMultiPointCSVEnv(t *testing.T, csvContent string, csvFilename string, colMapJSON string, extraPoints []*config.Point) (*strategyRunner, *model.AutoChangeConfig, *strategyState, *library.Store) {
	t.Helper()

	tmpDir := t.TempDir()
	csvDir := filepath.Join(tmpDir, "csv")
	os.MkdirAll(csvDir, 0755)
	csvPath := filepath.Join(csvDir, csvFilename)
	os.WriteFile(csvPath, []byte(csvContent), 0644)

	allPoints := []*config.Point{
		{IOA: 1, Name: "Point1", ValueType: config.VTFloat, PointType: config.TypeAI, Value: 0, Efficient: 1, BaseValue: 0},
	}
	allPoints = append(allPoints, extraPoints...)
	store := library.NewStore(allPoints)
	pub := &mockPublisher{}

	runner := newStrategyRunner(store, pub, tmpDir, "test-inst", nil)

	cfg := &model.AutoChangeConfig{
		PointIOA: 1,
		Strategy: model.StrategyCSV,
		Enabled:  true,
		Params: model.StrategyParams{
			CSVFileName:  csvFilename,
			CSVColumnMap: colMapJSON,
		},
	}
	state := &strategyState{}

	return runner, cfg, state, store
}

// pollUntilValue repeatedly calls doCSV until the store value matches expected or timeout.
func pollUntilValue(t *testing.T, runner *strategyRunner, cfg *model.AutoChangeConfig, state *strategyState, store *library.Store, ioa uint32, expected float64, timeout time.Duration) (float64, bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		runner.doCSV(cfg, state)
		p, _ := store.Get(ioa)
		if math.Abs(p.Value-expected) < 0.01 {
			return p.Value, true
		}
		time.Sleep(10 * time.Millisecond)
	}
	p, _ := store.Get(ioa)
	return p.Value, false
}

func pollUntilMultiValue(t *testing.T, runner *strategyRunner, cfg *model.AutoChangeConfig, state *strategyState, store *library.Store, expected map[uint32]float64, timeout time.Duration) (bool, map[uint32]float64) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		runner.doCSV(cfg, state)
		match := true
		actual := make(map[uint32]float64)
		for ioa, expVal := range expected {
			p, _ := store.Get(ioa)
			actual[ioa] = p.Value
			if math.Abs(p.Value-expVal) >= 0.01 {
				match = false
			}
		}
		if match {
			return true, actual
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false, nil
}

func TestCSVReplayRelativeMs(t *testing.T) {
	csvContent := `time,value
0,100
500,200
1000,300
1500,400
`
	runner, cfg, state, store := setupCSVTestEnv(t, csvContent, "test_ms.csv")
	cfg.Params.TimeFormat = "relative"
	cfg.Params.TimeUnit = "ms"

	checkpoints := []struct {
		wait     time.Duration
		expected float64
		label    string
	}{
		{50 * time.Millisecond, 100, "t=50ms (row0)"},
		{600 * time.Millisecond, 200, "t=600ms (row1)"},
		{600 * time.Millisecond, 300, "t=1200ms (row2)"},
		{600 * time.Millisecond, 400, "t=1800ms (row3)"},
	}

	for _, cp := range checkpoints {
		time.Sleep(cp.wait)
		val, ok := pollUntilValue(t, runner, cfg, state, store, 1, cp.expected, 200*time.Millisecond)
		if !ok {
			t.Errorf("%s: got value=%.0f, expected=%.0f", cp.label, val, cp.expected)
		} else {
			t.Logf("%s: value=%.0f OK", cp.label, val)
		}
	}
}

func TestCSVReplayRelativeS(t *testing.T) {
	csvContent := `time,value
0,10
1,20
2,30
3,40
`
	runner, cfg, state, store := setupCSVTestEnv(t, csvContent, "test_s.csv")
	cfg.Params.TimeFormat = "relative"
	cfg.Params.TimeUnit = "s"

	checkpoints := []struct {
		wait     time.Duration
		expected float64
		label    string
	}{
		{200 * time.Millisecond, 10, "t=0.2s (row0)"},
		{1200 * time.Millisecond, 20, "t=1.2s (row1)"},
		{1200 * time.Millisecond, 30, "t=2.2s (row2)"},
		{1200 * time.Millisecond, 40, "t=3.2s (row3)"},
	}

	for _, cp := range checkpoints {
		time.Sleep(cp.wait)
		val, ok := pollUntilValue(t, runner, cfg, state, store, 1, cp.expected, 300*time.Millisecond)
		if !ok {
			t.Errorf("%s: got value=%.0f, expected=%.0f", cp.label, val, cp.expected)
		} else {
			t.Logf("%s: value=%.0f OK", cp.label, val)
		}
	}
}

func TestCSVReplayAbsolute(t *testing.T) {
	now := time.Now()
	t0 := now.Add(800 * time.Millisecond).Format("15:04:05")
	t1 := now.Add(1800 * time.Millisecond).Format("15:04:05")
	t2 := now.Add(2800 * time.Millisecond).Format("15:04:05")
	t3 := now.Add(3800 * time.Millisecond).Format("15:04:05")

	csvContent := fmt.Sprintf(`time,value
%s,10
%s,20
%s,30
%s,40
`, t0, t1, t2, t3)

	runner, cfg, state, store := setupCSVTestEnv(t, csvContent, "test_abs.csv")
	cfg.Params.TimeFormat = "absolute"

	checkpoints := []struct {
		wait     time.Duration
		expected float64
		label    string
	}{
		{1000 * time.Millisecond, 10, "t=1.0s (row0)"},
		{1200 * time.Millisecond, 20, "t=1.8s (row1)"},
		{1200 * time.Millisecond, 30, "t=2.8s (row2)"},
		{1200 * time.Millisecond, 40, "t=3.8s (row3)"},
	}

	for _, cp := range checkpoints {
		time.Sleep(cp.wait)
		val, ok := pollUntilValue(t, runner, cfg, state, store, 1, cp.expected, 400*time.Millisecond)
		if !ok {
			t.Errorf("%s: got value=%.0f, expected=%.0f", cp.label, val, cp.expected)
		} else {
			t.Logf("%s: value=%.0f OK", cp.label, val)
		}
	}
}

func TestCSVMultiPointRelativeMs(t *testing.T) {
	csvContent := `time,value1,value2,value3
0,100,200,300
500,110,210,310
1000,120,220,320
`
	colMap := `{"1": 1, "2": 2, "3": 3}`
	extraPoints := []*config.Point{
		{IOA: 2, Name: "Point2", ValueType: config.VTFloat, PointType: config.TypeAI, Value: 0, Efficient: 1, BaseValue: 0},
		{IOA: 3, Name: "Point3", ValueType: config.VTFloat, PointType: config.TypeAI, Value: 0, Efficient: 1, BaseValue: 0},
	}
	runner, cfg, state, store := setupMultiPointCSVEnv(t, csvContent, "test_multi_ms.csv", colMap, extraPoints)
	cfg.Params.TimeFormat = "relative"
	cfg.Params.TimeUnit = "ms"

	checkpoints := []struct {
		wait     time.Duration
		expected map[uint32]float64
		label    string
	}{
		{50 * time.Millisecond, map[uint32]float64{1: 100, 2: 200, 3: 300}, "t=50ms (row0)"},
		{600 * time.Millisecond, map[uint32]float64{1: 110, 2: 210, 3: 310}, "t=600ms (row1)"},
		{600 * time.Millisecond, map[uint32]float64{1: 120, 2: 220, 3: 320}, "t=1200ms (row2)"},
	}

	for _, cp := range checkpoints {
		time.Sleep(cp.wait)
		ok, actual := pollUntilMultiValue(t, runner, cfg, state, store, cp.expected, 200*time.Millisecond)
		if !ok {
			t.Errorf("%s: mismatch", cp.label)
			for ioa, exp := range cp.expected {
				t.Errorf("  IOA %d: got %.0f, expected %.0f", ioa, actual[ioa], exp)
			}
		} else {
			t.Logf("%s: all points OK", cp.label)
		}
	}
}

func TestCSVMultiPointAbsolute(t *testing.T) {
	now := time.Now()
	t0 := now.Add(800 * time.Millisecond).Format("15:04:05")
	t1 := now.Add(1800 * time.Millisecond).Format("15:04:05")
	t2 := now.Add(2800 * time.Millisecond).Format("15:04:05")

	csvContent := fmt.Sprintf(`time,value1,value2
%s,10,100
%s,20,200
%s,30,300
`, t0, t1, t2)

	colMap := `{"1": 1, "2": 2}`
	extraPoints := []*config.Point{
		{IOA: 2, Name: "Point2", ValueType: config.VTFloat, PointType: config.TypeAI, Value: 0, Efficient: 1, BaseValue: 0},
	}
	runner, cfg, state, store := setupMultiPointCSVEnv(t, csvContent, "test_multi_abs.csv", colMap, extraPoints)
	cfg.Params.TimeFormat = "absolute"

	checkpoints := []struct {
		wait     time.Duration
		expected map[uint32]float64
		label    string
	}{
		{1000 * time.Millisecond, map[uint32]float64{1: 10, 2: 100}, "t=1.0s (row0)"},
		{1200 * time.Millisecond, map[uint32]float64{1: 20, 2: 200}, "t=1.8s (row1)"},
		{1200 * time.Millisecond, map[uint32]float64{1: 30, 2: 300}, "t=2.8s (row2)"},
	}

	for _, cp := range checkpoints {
		time.Sleep(cp.wait)
		ok, actual := pollUntilMultiValue(t, runner, cfg, state, store, cp.expected, 400*time.Millisecond)
		if !ok {
			t.Errorf("%s: mismatch", cp.label)
			for ioa, exp := range cp.expected {
				t.Errorf("  IOA %d: got %.0f, expected %.0f", ioa, actual[ioa], exp)
			}
		} else {
			t.Logf("%s: all points OK", cp.label)
		}
	}
}

func TestCSVMultiPointRelativeS(t *testing.T) {
	csvContent := `time,value1,value2
0,50,500
1,60,600
2,70,700
`
	colMap := `{"1": 1, "2": 2}`
	extraPoints := []*config.Point{
		{IOA: 2, Name: "Point2", ValueType: config.VTFloat, PointType: config.TypeAI, Value: 0, Efficient: 1, BaseValue: 0},
	}
	runner, cfg, state, store := setupMultiPointCSVEnv(t, csvContent, "test_multi_s.csv", colMap, extraPoints)
	cfg.Params.TimeFormat = "relative"
	cfg.Params.TimeUnit = "s"

	checkpoints := []struct {
		wait     time.Duration
		expected map[uint32]float64
		label    string
	}{
		{200 * time.Millisecond, map[uint32]float64{1: 50, 2: 500}, "t=0.2s (row0)"},
		{1200 * time.Millisecond, map[uint32]float64{1: 60, 2: 600}, "t=1.2s (row1)"},
		{1200 * time.Millisecond, map[uint32]float64{1: 70, 2: 700}, "t=2.2s (row2)"},
	}

	for _, cp := range checkpoints {
		time.Sleep(cp.wait)
		ok, actual := pollUntilMultiValue(t, runner, cfg, state, store, cp.expected, 300*time.Millisecond)
		if !ok {
			t.Errorf("%s: mismatch", cp.label)
			for ioa, exp := range cp.expected {
				t.Errorf("  IOA %d: got %.0f, expected %.0f", ioa, actual[ioa], exp)
			}
		} else {
			t.Logf("%s: all points OK", cp.label)
		}
	}
}
