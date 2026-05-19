package protocol

import (
	"fmt"

	"iec104-sim/internal/model"
	"iec104-sim/pkg/protocol/modbus"
)

func New(cfg model.InstanceConfig) (Protocol, error) {
	switch cfg.Protocol {
	case "modbus_tcp":
		port := cfg.IEC104Port
		if cfg.ModbusConfig != nil && cfg.ModbusConfig.Port > 0 {
			port = cfg.ModbusConfig.Port
		}
		slaveID := uint8(1)
		byteOrder := "ABCD"
		if cfg.ModbusConfig != nil {
			if cfg.ModbusConfig.SlaveID > 0 {
				slaveID = cfg.ModbusConfig.SlaveID
			}
			if cfg.ModbusConfig.ByteOrder != "" {
				byteOrder = cfg.ModbusConfig.ByteOrder
			}
		}
		return modbus.NewTCPServer(port, slaveID, byteOrder), nil
	case "", "iec104":
		return NewIEC104Wrapper(cfg.IEC104Port), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", cfg.Protocol)
	}
}

func SupportedProtocols() []string {
	return []string{"iec104", "modbus_tcp"}
}
