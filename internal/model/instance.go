package model

// InstanceConfig represents a persisted instance configuration.
type InstanceConfig struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IEC104Port  int    `json:"iec104_port"`
	XLSXFile    string `json:"xlsx_file"`
	Enabled     bool   `json:"enabled"`
	HttpEnabled bool   `json:"http_enabled"`
	HttpPort    int    `json:"http_port"`

	Protocol     string                `json:"protocol,omitempty"`
	ModbusConfig *ModbusInstanceConfig `json:"modbus_config,omitempty"`
}

type ModbusInstanceConfig struct {
	Port      int    `json:"port,omitempty"`
	ByteOrder string `json:"byte_order,omitempty"`
	SlaveID   uint8  `json:"slave_id,omitempty"`
}

// InstanceStatus represents the runtime status of an instance.
type InstanceStatus string

const (
	StatusStopped InstanceStatus = "stopped"
	StatusRunning InstanceStatus = "running"
	StatusError   InstanceStatus = "error"
)

// InstanceState is the runtime state of a managed instance.
type InstanceState struct {
	Config          InstanceConfig `json:"config"`
	Status          InstanceStatus `json:"status"`
	UptimeSeconds   int64          `json:"uptime_seconds,omitempty"`
	TotalPoints     int            `json:"total_points,omitempty"`
	ClientConnected bool           `json:"client_connected"`
	Interrogations  int64          `json:"interrogations,omitempty"`
	Controls        int64          `json:"controls,omitempty"`
	Spontaneous     int64          `json:"spontaneous,omitempty"`
	Error           string         `json:"error,omitempty"`
}
