package protocol

import (
	"iec104-sim/pkg/config"
	"iec104-sim/pkg/iec104"
	"iec104-sim/pkg/library"
)

type IEC104Wrapper struct {
	server *iec104.Server
	port   int
	store  *library.Store
}

func NewIEC104Wrapper(port int) *IEC104Wrapper {
	return &IEC104Wrapper{port: port}
}

func (w *IEC104Wrapper) Name() string { return "iec104" }

func (w *IEC104Wrapper) Start() error {
	w.server = iec104.NewServer(w.port, w.store)
	return w.server.Start()
}

func (w *IEC104Wrapper) Stop() {
	if w.server != nil {
		w.server.Stop()
	}
}

func (w *IEC104Wrapper) ClientConnected() bool {
	if w.server == nil {
		return false
	}
	return w.server.ClientConnected()
}

func (w *IEC104Wrapper) ClientAddr() string {
	if w.server == nil {
		return ""
	}
	return w.server.ClientAddr()
}

func (w *IEC104Wrapper) Stats() (interrog, control, spont int64) {
	if w.server == nil {
		return 0, 0, 0
	}
	return w.server.Stats()
}

func (w *IEC104Wrapper) Uptime() int64 {
	if w.server == nil {
		return 0
	}
	return w.server.Uptime()
}

func (w *IEC104Wrapper) Publish(point *config.Point) {
	if w.server != nil {
		w.server.Publish(point)
	}
}

func (w *IEC104Wrapper) SetStore(store *library.Store) {
	w.store = store
}
