package iec104

import (
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"iec104-sim/pkg/config"
	"iec104-sim/pkg/library"

	"github.com/wendy512/go-iecp5/asdu"
	"github.com/wendy512/go-iecp5/cs104"
)

type Server struct {
	port        int
	store       *library.Store
	mu          sync.Mutex
	connect     asdu.Connect
	connected   bool
	publishCh   chan *config.Point
	connMu      sync.RWMutex
	interrogCnt atomic.Int64
	controlCnt  atomic.Int64
	spontCnt    atomic.Int64
	startTime   time.Time
	server      *cs104.Server
	started     bool
}

func NewServer(port int, store *library.Store) *Server {
	return &Server{
		port:  port,
		store: store,
	}
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	s.startTime = time.Now()
	s.publishCh = make(chan *config.Point, 1024)
	s.connected = false
	s.connect = nil

	handler := &serverHandler{srv: s}
	server := cs104.NewServer(handler)
	server.SetOnConnectionHandler(s.onConnect)
	server.SetConnectionLostHandler(s.onDisconnect)
	s.server = server

	go s.publishLoop()

	go func() {
		slog.Info("IEC104 服务端已启动", "port", s.port)
		server.ListenAndServer(s.addr())
	}()

	s.started = true
	return nil
}

func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return
	}

	slog.Info("正在停止 IEC104 服务端", "port", s.port)

	if s.server != nil {
		_ = s.server.Close()
		s.server = nil
	}

	s.connMu.Lock()
	s.connected = false
	s.connect = nil
	s.connMu.Unlock()

	s.started = false
	slog.Info("IEC104 服务端已停止", "port", s.port)
}

func (s *Server) addr() string {
	return ":" + strconv.Itoa(s.port)
}

func (s *Server) onConnect(c asdu.Connect) {
	s.connMu.Lock()
	if s.connected {
		s.connMu.Unlock()
		slog.Warn("已有客户端连接，拒绝新连接", "remote", c.UnderlyingConn().RemoteAddr())
		return
	}
	s.connect = c
	s.connected = true
	s.connMu.Unlock()

	slog.Info("客户端已连接", "remote", c.UnderlyingConn().RemoteAddr())
}

func (s *Server) onDisconnect(c asdu.Connect) {
	s.connMu.Lock()
	s.connected = false
	s.connect = nil
	s.connMu.Unlock()

	slog.Info("客户端已断开", "remote", c.UnderlyingConn().RemoteAddr())
}

func (s *Server) publishLoop() {
	for pt := range s.publishCh {
		c := s.getConnect()
		if c == nil {
			continue
		}
		s.sendSpontaneous(c, pt)
	}
}

func (s *Server) sendSpontaneous(c asdu.Connect, pt *config.Point) {
	coa := asdu.CauseOfTransmission{Cause: asdu.Spontaneous}
	commonAddr := asdu.CommonAddr(1)

	var err error
	switch pt.PointType {
	case config.TypeAI, config.TypeAO:
		info := asdu.MeasuredValueFloatInfo{
			Ioa:   asdu.InfoObjAddr(pt.IOA),
			Value: float32(pt.Value),
			Qds:   qualityToQDS(pt.QDS),
		}
		err = asdu.MeasuredValueFloat(c, false, coa, commonAddr, info)

	case config.TypeDI, config.TypeDO:
		info := asdu.SinglePointInfo{
			Ioa:   asdu.InfoObjAddr(pt.IOA),
			Value: pt.BoolValue,
			Qds:   qualityToQDS(pt.QDS),
		}
		err = asdu.Single(c, false, coa, commonAddr, info)

	case config.TypePI:
		info := asdu.BinaryCounterReadingInfo{
			Ioa: asdu.InfoObjAddr(pt.IOA),
		}
		err = asdu.IntegratedTotals(c, false, coa, commonAddr, info)
	}

	if err != nil {
		slog.Warn("发送变化上送失败", "ioa", pt.IOA, "error", err)
	} else {
		s.spontCnt.Add(1)
		slog.Info("变化上送", "ioa", pt.IOA, "value", formatPointValue(pt))
	}
}

func (s *Server) getConnect() asdu.Connect {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	return s.connect
}

func (s *Server) Publish(point *config.Point) {
	s.connMu.RLock()
	connected := s.connected
	s.connMu.RUnlock()
	if !connected {
		return
	}
	select {
	case s.publishCh <- point:
	default:
		slog.Warn("发布通道已满，丢弃变化上送", "ioa", point.IOA)
	}
}

func (s *Server) ClientConnected() bool {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	return s.connected
}

func (s *Server) ClientAddr() string {
	c := s.getConnect()
	if c == nil {
		return ""
	}
	return c.UnderlyingConn().RemoteAddr().String()
}

func (s *Server) Stats() (interrog, control, spont int64) {
	return s.interrogCnt.Load(), s.controlCnt.Load(), s.spontCnt.Load()
}

func (s *Server) Uptime() int64 {
	return int64(time.Since(s.startTime).Seconds())
}

func (s *Server) Port() int {
	return s.port
}

func formatPointValue(pt *config.Point) interface{} {
	switch pt.PointType {
	case config.TypeAI, config.TypeAO:
		return pt.Value
	case config.TypeDI, config.TypeDO:
		return pt.BoolValue
	case config.TypePI:
		return pt.IntValue
	default:
		return pt.Value
	}
}

// serverHandler implements cs104.ServerHandlerInterface
type serverHandler struct {
	srv *Server
}

func (h *serverHandler) InterrogationHandler(c asdu.Connect, a *asdu.ASDU, qoi asdu.QualifierOfInterrogation) error {
	slog.Info("收到总召", "remote", c.UnderlyingConn().RemoteAddr(), "qoi", qoi)
	h.srv.interrogCnt.Add(1)

	if err := a.SendReplyMirror(c, asdu.ActivationCon); err != nil {
		slog.Warn("发送总召 ACT_CON 失败", "error", err)
	}

	coa := asdu.CauseOfTransmission{Cause: asdu.InterrogatedByStation}
	commonAddr := a.CommonAddr

	sendPointsByType(c, h.srv.store, config.TypeAI, coa, commonAddr)
	sendPointsByType(c, h.srv.store, config.TypeDI, coa, commonAddr)
	sendPointsByType(c, h.srv.store, config.TypeAO, coa, commonAddr)
	sendPointsByType(c, h.srv.store, config.TypeDO, coa, commonAddr)

	if err := a.SendReplyMirror(c, asdu.ActivationTerm); err != nil {
		slog.Warn("发送总召 ACT_TERM 失败", "error", err)
	}

	slog.Info("总召完成", "remote", c.UnderlyingConn().RemoteAddr(), "totalPoints", h.srv.store.TotalCount())
	return nil
}

func (h *serverHandler) CounterInterrogationHandler(c asdu.Connect, a *asdu.ASDU, qcc asdu.QualifierCountCall) error {
	slog.Info("收到电度召唤", "remote", c.UnderlyingConn().RemoteAddr())

	if err := a.SendReplyMirror(c, asdu.ActivationCon); err != nil {
		slog.Warn("发送电度召唤 ACT_CON 失败", "error", err)
	}

	coa := asdu.CauseOfTransmission{Cause: asdu.RequestByGeneralCounter}
	commonAddr := a.CommonAddr
	sendPointsByType(c, h.srv.store, config.TypePI, coa, commonAddr)

	if err := a.SendReplyMirror(c, asdu.ActivationTerm); err != nil {
		slog.Warn("发送电度召唤 ACT_TERM 失败", "error", err)
	}
	return nil
}

func (h *serverHandler) ReadHandler(c asdu.Connect, a *asdu.ASDU, ioa asdu.InfoObjAddr) error {
	return nil
}

func (h *serverHandler) ClockSyncHandler(c asdu.Connect, a *asdu.ASDU, t time.Time) error {
	slog.Debug("收到时钟同步", "remote", c.UnderlyingConn().RemoteAddr())
	return nil
}

func (h *serverHandler) ResetProcessHandler(c asdu.Connect, a *asdu.ASDU, qrp asdu.QualifierOfResetProcessCmd) error {
	return nil
}

func (h *serverHandler) DelayAcquisitionHandler(c asdu.Connect, a *asdu.ASDU, delay uint16) error {
	return nil
}

func (h *serverHandler) ASDUHandler(c asdu.Connect, a *asdu.ASDU) error {
	switch a.Type {
	case asdu.C_SC_NA_1:
		return h.handleSingleCommand(c, a)
	case asdu.C_SE_NC_1:
		return h.handleSetpointCommand(c, a)
	default:
		slog.Debug("收到未处理的ASDU", "type", a.Type)
	}
	return nil
}

func (h *serverHandler) handleSingleCommand(c asdu.Connect, a *asdu.ASDU) error {
	mirror := a.Clone()

	if err := a.SendReplyMirror(c, asdu.ActivationCon); err != nil {
		slog.Warn("发送DO ACT_CON失败", "error", err)
	}

	cmd := a.GetSingleCmd()
	ioa := uint32(cmd.Ioa)
	slog.Info("收到DO控制", "ioa", ioa, "value", cmd.Value)

	pt, err := h.srv.store.SetBoolValue(ioa, cmd.Value)
	if err != nil {
		slog.Warn("DO控制更新失败", "ioa", ioa, "error", err)
	}

	if err := mirror.SendReplyMirror(c, asdu.ActivationTerm); err != nil {
		slog.Warn("发送DO ACT_TERM失败", "error", err)
	}

	h.srv.controlCnt.Add(1)
	if pt != nil {
		h.srv.Publish(pt)
	}
	return nil
}

func (h *serverHandler) handleSetpointCommand(c asdu.Connect, a *asdu.ASDU) error {
	mirror := a.Clone()

	if err := a.SendReplyMirror(c, asdu.ActivationCon); err != nil {
		slog.Warn("发送AO ACT_CON失败", "error", err)
	}

	cmd := a.GetSetpointFloatCmd()
	ioa := uint32(cmd.Ioa)
	slog.Info("收到AO控制", "ioa", ioa, "value", cmd.Value)

	pt, err := h.srv.store.SetValue(ioa, float64(cmd.Value))
	if err != nil {
		slog.Warn("AO控制更新失败", "ioa", ioa, "error", err)
	}

	if err := mirror.SendReplyMirror(c, asdu.ActivationTerm); err != nil {
		slog.Warn("发送AO ACT_TERM失败", "error", err)
	}

	h.srv.controlCnt.Add(1)
	if pt != nil {
		h.srv.Publish(pt)
	}
	return nil
}

func sendPointsByType(c asdu.Connect, store *library.Store, pt config.PointType, coa asdu.CauseOfTransmission, commonAddr asdu.CommonAddr) {
	for _, point := range store.SnapshotByType(pt) {
		switch pt {
		case config.TypeAI, config.TypeAO:
			info := asdu.MeasuredValueFloatInfo{
				Ioa:   asdu.InfoObjAddr(point.IOA),
				Value: float32(point.Value),
				Qds:   qualityToQDS(point.QDS),
			}
			asdu.MeasuredValueFloat(c, false, coa, commonAddr, info)

		case config.TypeDI, config.TypeDO:
			info := asdu.SinglePointInfo{
				Ioa:   asdu.InfoObjAddr(point.IOA),
				Value: point.BoolValue,
				Qds:   qualityToQDS(point.QDS),
			}
			asdu.Single(c, false, coa, commonAddr, info)

		case config.TypePI:
			info := asdu.BinaryCounterReadingInfo{
				Ioa: asdu.InfoObjAddr(point.IOA),
			}
			asdu.IntegratedTotals(c, false, coa, commonAddr, info)
		}
	}
}

func qualityToQDS(q config.QualityDescriptor) asdu.QualityDescriptor {
	var qds asdu.QualityDescriptor
	if q.Invalid {
		qds |= asdu.QDSInvalid
	}
	if q.NotTopical {
		qds |= asdu.QDSNotTopical
	}
	if q.Substituted {
		qds |= asdu.QDSSubstituted
	}
	if q.Overflow {
		qds |= asdu.QDSOverflow
	}
	if q.Blocked {
		qds |= asdu.QDSBlocked
	}
	return qds
}
