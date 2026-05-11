package library

import (
	"fmt"
	"sync"
	"time"

	"iec104-sim/pkg/config"
)

type Store struct {
	mu     sync.RWMutex
	points map[uint32]*config.Point
	byType map[config.PointType][]*config.Point
}

func NewStore(points []*config.Point) *Store {
	s := &Store{
		points: make(map[uint32]*config.Point),
		byType: make(map[config.PointType][]*config.Point),
	}
	for _, p := range points {
		s.points[p.IOA] = p
		s.byType[p.PointType] = append(s.byType[p.PointType], p)
	}
	return s
}

func (s *Store) Get(ioa uint32) (*config.Point, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.points[ioa]
	return p, ok
}

func (s *Store) GetAll() []*config.Point {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*config.Point, 0, len(s.points))
	for _, p := range s.points {
		result = append(result, p)
	}
	return result
}

func (s *Store) GetByType(pt config.PointType) []*config.Point {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byType[pt]
}

func (s *Store) CountByType() map[config.PointType]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[config.PointType]int)
	for pt, list := range s.byType {
		result[pt] = len(list)
	}
	return result
}

func (s *Store) SnapshotByType(pt config.PointType) []*config.Point {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := s.byType[pt]
	snap := make([]*config.Point, len(list))
	for i, p := range list {
		cp := *p
		snap[i] = &cp
	}
	return snap
}

func (s *Store) SetValue(ioa uint32, value float64) (*config.Point, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.points[ioa]
	if !ok {
		return nil, fmt.Errorf("IOA %d not found", ioa)
	}

	switch p.PointType {
	case config.TypeAI, config.TypeAO:
		p.Value = value
	case config.TypeDI, config.TypeDO:
		p.BoolValue = int64(value) != 0
	case config.TypePI:
		p.IntValue = int32(value)
	}

	p.Timestamp = time.Now()
	p.Changed = true
	return p, nil
}

func (s *Store) SetBoolValue(ioa uint32, val bool) (*config.Point, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.points[ioa]
	if !ok {
		return nil, fmt.Errorf("IOA %d not found", ioa)
	}

	p.BoolValue = val
	p.Timestamp = time.Now()
	p.Changed = true
	return p, nil
}

func (s *Store) SetIntValue(ioa uint32, val int32) (*config.Point, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.points[ioa]
	if !ok {
		return nil, fmt.Errorf("IOA %d not found", ioa)
	}

	p.IntValue = val
	p.Timestamp = time.Now()
	p.Changed = true
	return p, nil
}

func (s *Store) SetQDS(ioa uint32, qds config.QualityDescriptor) (*config.Point, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.points[ioa]
	if !ok {
		return nil, fmt.Errorf("IOA %d not found", ioa)
	}

	p.QDS = qds
	return p, nil
}

func (s *Store) CollectChanged() []*config.Point {
	s.mu.Lock()
	defer s.mu.Unlock()

	var changed []*config.Point
	for _, p := range s.points {
		if p.Changed {
			changed = append(changed, p)
			p.Changed = false
		}
	}
	return changed
}

func (s *Store) TotalCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.points)
}
