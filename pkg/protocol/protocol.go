package protocol

import (
	"iec104-sim/pkg/config"
	"iec104-sim/pkg/library"
)

type Protocol interface {
	Name() string
	Start() error
	Stop()
	ClientConnected() bool
	ClientAddr() string
	Stats() (interrog, control, spont int64)
	Uptime() int64
	Publish(point *config.Point)
	SetStore(store *library.Store)
}
