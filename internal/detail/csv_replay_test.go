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

// pollUntilValue repeatedly calls doCSV until the store value matches expected or timeout.
func pollUntilValue(t *testing.T, runner *strategyRunner, cfg *model.AutoChangeConfig, state *strategyState, store *library.Store, expected float64, timeout time.Duration) (float64, bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		runner.doCSV(cfg, state)
		p, _ := store.Get(1)
		if math.Abs(p.Value-expected) < 0.01 {
			return p.Value, true
		}
		time.Sleep(10 * time.Millisecond)
	}
	p, _ := store.Get(1)
	return p.Value, false
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
		val, ok := pollUntilValue(t, runner, cfg, state, store, cp.expected, 200*time.Millisecond)
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
		val, ok := pollUntilValue(t, runner, cfg, state, store, cp.expected, 300*time.Millisecond)
		if !ok {
			t.Errorf("%s: got value=%.0f, expected=%.0f", cp.label, val, cp.expected)
		} else {
			t.Logf("%s: value=%.0f OK", cp.label, val)
		}
	}
}

func TestCSVReplayAbsolute(t *testing.T) {
	now := time.Now()
	t0 := now.Add(500 * time.Millisecond).Format("15:04:05")
	t1 := now.Add(1500 * time.Millisecond).Format("15:04:05")
	t2 := now.Add(2500 * time.Millisecond).Format("15:04:05")
	t3 := now.Add(3500 * time.Millisecond).Format("15:04:05")

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
		{700 * time.Millisecond, 10, "t=0.7s (row0)"},
		{1000 * time.Millisecond, 20, "t=1.5s (row1)"},
		{1200 * time.Millisecond, 30, "t=2.5s (row2)"},
		{1200 * time.Millisecond, 40, "t=3.5s (row3)"},
	}

	for _, cp := range checkpoints {
		time.Sleep(cp.wait)
		val, ok := pollUntilValue(t, runner, cfg, state, store, cp.expected, 300*time.Millisecond)
		if !ok {
			t.Errorf("%s: got value=%.0f, expected=%.0f", cp.label, val, cp.expected)
		} else {
			t.Logf("%s: value=%.0f OK", cp.label, val)
		}
	}
}
