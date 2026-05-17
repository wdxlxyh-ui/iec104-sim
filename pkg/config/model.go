package config

import "time"

type PointType string

const (
	TypeAI PointType = "AI"
	TypeDI PointType = "DI"
	TypePI PointType = "PI"
	TypeDO PointType = "DO"
	TypeAO PointType = "AO"
)

type ValueType string

const (
	VTFloat  ValueType = "FLOAT"
	VTDouble ValueType = "DOUBLE"
	VTInt    ValueType = "INT"
	VTBit    ValueType = "BIT"
)

type QualityDescriptor struct {
	Invalid     bool `json:"invalid"`
	NotTopical  bool `json:"not_topical"`
	Substituted bool `json:"substituted"`
	Overflow    bool `json:"overflow"`
	Blocked     bool `json:"blocked"`
}

type Point struct {
	IOA       uint32            `json:"ioa"`
	Name      string            `json:"name"`
	ValueType ValueType         `json:"value_type"`
	PointType PointType         `json:"point_type"`
	Value     float64           `json:"value"`
	BoolValue bool              `json:"bool_value"`
	IntValue  int32             `json:"int_value"`
	Efficient float64           `json:"efficient"`
	BaseValue float64           `json:"base_value"`
	QDS       QualityDescriptor `json:"qds"`
	Alias     string            `json:"alias"`
	Timestamp time.Time         `json:"timestamp"`
	Changed   bool              `json:"-"`

	FunctionCode    uint8  `json:"function_code,omitempty"`
	RegisterAddress uint16 `json:"register_address,omitempty"`
	ByteOrder       string `json:"byte_order,omitempty"`
}
