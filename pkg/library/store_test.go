package library

import (
	"testing"

	"iec104-sim/pkg/config"
)

func makeTestPoints() []*config.Point {
	return []*config.Point{
		{IOA: 1001, Name: "AI_01", PointType: config.TypeAI, ValueType: config.VTFloat, Value: 220.0, Efficient: 1.0, BaseValue: 220.0},
		{IOA: 2001, Name: "DI_01", PointType: config.TypeDI, ValueType: config.VTBit, BoolValue: false},
		{IOA: 3001, Name: "PI_01", PointType: config.TypePI, ValueType: config.VTInt, IntValue: 1000},
	}
}

func TestNewStore(t *testing.T) {
	points := makeTestPoints()
	s := NewStore(points)

	if s.TotalCount() != 3 {
		t.Errorf("expected 3 points, got %d", s.TotalCount())
	}
}

func TestStore_Get(t *testing.T) {
	s := NewStore(makeTestPoints())

	p, ok := s.Get(1001)
	if !ok {
		t.Fatal("expected to find IOA 1001")
	}
	if p.Name != "AI_01" {
		t.Errorf("expected AI_01, got %s", p.Name)
	}

	_, ok = s.Get(9999)
	if ok {
		t.Error("expected not to find IOA 9999")
	}
}

func TestStore_SetValue(t *testing.T) {
	s := NewStore(makeTestPoints())

	p, err := s.SetValue(1001, 235.5)
	if err != nil {
		t.Fatalf("SetValue failed: %v", err)
	}
	if p.Value != 235.5 {
		t.Errorf("expected 235.5, got %f", p.Value)
	}
	if !p.Changed {
		t.Error("expected Changed=true")
	}

	// Wrong type for IOA 2001 (DI expects bool)
	p, err = s.SetValue(2001, 1.0)
	if err != nil {
		t.Fatalf("SetValue on DI failed: %v", err)
	}
	if !p.BoolValue {
		t.Error("expected BoolValue=true")
	}

	// Non-existent IOA
	_, err = s.SetValue(9999, 0)
	if err == nil {
		t.Error("expected error for non-existent IOA")
	}
}

func TestStore_SetBoolValue(t *testing.T) {
	s := NewStore(makeTestPoints())

	p, err := s.SetBoolValue(2001, true)
	if err != nil {
		t.Fatalf("SetBoolValue failed: %v", err)
	}
	if !p.BoolValue {
		t.Error("expected BoolValue=true")
	}

	_, err = s.SetBoolValue(9999, true)
	if err == nil {
		t.Error("expected error for non-existent IOA")
	}
}

func TestStore_SetIntValue(t *testing.T) {
	s := NewStore(makeTestPoints())

	p, err := s.SetIntValue(3001, 9999)
	if err != nil {
		t.Fatalf("SetIntValue failed: %v", err)
	}
	if p.IntValue != 9999 {
		t.Errorf("expected 9999, got %d", p.IntValue)
	}
}

func TestStore_CollectChanged(t *testing.T) {
	s := NewStore(makeTestPoints())

	s.SetValue(1001, 150.0)
	s.SetBoolValue(2001, true)

	changed := s.CollectChanged()
	if len(changed) != 2 {
		t.Errorf("expected 2 changed points, got %d", len(changed))
	}

	// After collect, all changed flags should be reset
	changed2 := s.CollectChanged()
	if len(changed2) != 0 {
		t.Errorf("expected 0 changed after collect, got %d", len(changed2))
	}
}

func TestStore_GetByType(t *testing.T) {
	s := NewStore(makeTestPoints())

	aiPoints := s.GetByType(config.TypeAI)
	if len(aiPoints) != 1 {
		t.Errorf("expected 1 AI point, got %d", len(aiPoints))
	}

	diPoints := s.GetByType(config.TypeDI)
	if len(diPoints) != 1 {
		t.Errorf("expected 1 DI point, got %d", len(diPoints))
	}

	doPoints := s.GetByType(config.TypeDO)
	if len(doPoints) != 0 {
		t.Errorf("expected 0 DO points, got %d", len(doPoints))
	}
}

func TestStore_CountByType(t *testing.T) {
	s := NewStore(makeTestPoints())

	counts := s.CountByType()
	if counts[config.TypeAI] != 1 {
		t.Errorf("expected 1 AI, got %d", counts[config.TypeAI])
	}
	if counts[config.TypeDI] != 1 {
		t.Errorf("expected 1 DI, got %d", counts[config.TypeDI])
	}
	if counts[config.TypePI] != 1 {
		t.Errorf("expected 1 PI, got %d", counts[config.TypePI])
	}
}

func TestStore_SetQDS(t *testing.T) {
	s := NewStore(makeTestPoints())

	qds := config.QualityDescriptor{Invalid: true, Blocked: true}
	p, err := s.SetQDS(1001, qds)
	if err != nil {
		t.Fatalf("SetQDS failed: %v", err)
	}
	if !p.QDS.Invalid || !p.QDS.Blocked {
		t.Errorf("QDS not set correctly: %+v", p.QDS)
	}

	_, err = s.SetQDS(9999, qds)
	if err == nil {
		t.Error("expected error for non-existent IOA")
	}
}
