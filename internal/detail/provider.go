package detail

import "iec104-sim/pkg/library"

// StoreProvider 提供跨实例的 Store 查询能力。
// Manager 实现了此接口，注入到 Engine 后使其可查询其他运行中实例的测点值。
type StoreProvider interface {
	// GetStore 返回指定实例的 Store。
	// 实例未运行或不存在时返回 nil（不会 panic）。
	GetStore(instanceID string) *library.Store
}
