package modbus

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"iec104-sim/pkg/config"
	"iec104-sim/pkg/library"
)

type ModbusTCPServer struct {
	port      int
	slaveID   uint8
	byteOrder string
	store     *library.Store

	mu        sync.Mutex
	conn      net.Conn
	connected bool
	connMu    sync.RWMutex

	startTime   time.Time
	interrogCnt atomic.Int64
	controlCnt  atomic.Int64
	spontCnt    atomic.Int64

	listener net.Listener
	stopCh   chan struct{}
}

func NewTCPServer(port int, slaveID uint8, byteOrder string) *ModbusTCPServer {
	return &ModbusTCPServer{
		port:      port,
		slaveID:   slaveID,
		byteOrder: byteOrder,
		stopCh:    make(chan struct{}),
	}
}

func (s *ModbusTCPServer) Name() string { return "modbus_tcp" }

func (s *ModbusTCPServer) SetStore(store *library.Store) {
	s.store = store
}

func (s *ModbusTCPServer) Start() error {
	s.startTime = time.Now()
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("listen tcp :%d: %w", s.port, err)
	}
	s.listener = ln
	slog.Info("Modbus TCP 服务端已启动", "port", s.port, "slave_id", s.slaveID)
	go s.acceptLoop(ln)
	return nil
}

func (s *ModbusTCPServer) Stop() {
	slog.Info("正在停止 Modbus TCP 服务端", "port", s.port)
	if s.listener != nil {
		s.listener.Close()
	}
	close(s.stopCh)
	s.connMu.Lock()
	if s.conn != nil {
		s.conn.Close()
	}
	s.connected = false
	s.conn = nil
	s.connMu.Unlock()
	slog.Info("Modbus TCP 服务端已停止", "port", s.port)
}

func (s *ModbusTCPServer) ClientConnected() bool {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	return s.connected
}

func (s *ModbusTCPServer) ClientAddr() string {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	if s.conn == nil {
		return ""
	}
	return s.conn.RemoteAddr().String()
}

func (s *ModbusTCPServer) Stats() (interrog, control, spont int64) {
	return s.interrogCnt.Load(), s.controlCnt.Load(), s.spontCnt.Load()
}

func (s *ModbusTCPServer) Uptime() int64 {
	return int64(time.Since(s.startTime).Seconds())
}

func (s *ModbusTCPServer) Publish(point *config.Point) {
	s.spontCnt.Add(1)
}

func (s *ModbusTCPServer) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				slog.Warn("Modbus TCP accept error", "error", err)
				continue
			}
		}

		s.connMu.Lock()
		if s.connected {
			s.connMu.Unlock()
			slog.Warn("Modbus TCP: 已有客户端连接，拒绝新连接", "remote", conn.RemoteAddr())
			conn.Close()
			continue
		}
		s.conn = conn
		s.connected = true
		s.connMu.Unlock()

		slog.Info("Modbus TCP 客户端已连接", "remote", conn.RemoteAddr())
		s.handleConnection(conn)

		s.connMu.Lock()
		s.connected = false
		s.conn = nil
		s.connMu.Unlock()
		slog.Info("Modbus TCP 客户端已断开", "remote", conn.RemoteAddr())
	}
}

func (s *ModbusTCPServer) handleConnection(conn net.Conn) {
	for {
		mbap := make([]byte, 7)
		if _, err := io.ReadFull(conn, mbap); err != nil {
			return
		}

		length := binary.BigEndian.Uint16(mbap[4:6])
		pdu := make([]byte, length-1)
		if _, err := io.ReadFull(conn, pdu); err != nil {
			return
		}

		unitID := pdu[0]
		functionCode := pdu[1]
		data := pdu[2:]

		_ = unitID

		response := s.handleRequest(functionCode, data)
		if response == nil {
			continue
		}

		respLen := len(response) + 1
		respMBAP := make([]byte, 7)
		copy(respMBAP[0:2], mbap[0:2])
		binary.BigEndian.PutUint16(respMBAP[2:4], 0)
		binary.BigEndian.PutUint16(respMBAP[4:6], uint16(respLen))
		respMBAP[6] = s.slaveID

		resp := append(respMBAP, response...)
		if _, err := conn.Write(resp); err != nil {
			return
		}
	}
}

func (s *ModbusTCPServer) handleRequest(fc uint8, data []byte) []byte {
	switch fc {
	case 0x01:
		return s.readCoils(data)
	case 0x02:
		return s.readDiscreteInputs(data)
	case 0x03:
		return s.readHoldingRegisters(data)
	case 0x04:
		return s.readInputRegisters(data)
	case 0x05:
		return s.writeSingleCoil(data)
	case 0x06:
		return s.writeSingleRegister(data)
	case 0x0F:
		return s.writeMultipleCoils(data)
	case 0x10:
		return s.writeMultipleRegisters(data)
	default:
		return s.errorResponse(fc, 0x01)
	}
}

func (s *ModbusTCPServer) readCoils(data []byte) []byte {
	if len(data) < 4 {
		return s.errorResponse(0x01, 0x02)
	}
	startAddr := binary.BigEndian.Uint16(data[0:2])
	quantity := binary.BigEndian.Uint16(data[2:4])
	if quantity < 1 || quantity > 2000 {
		return s.errorResponse(0x01, 0x03)
	}

	points := s.store.GetByFunctionCodeRange(0x01, startAddr, quantity)
	byteCount := (quantity + 7) / 8
	coilBytes := make([]byte, byteCount)

	for _, p := range points {
		offset := p.RegisterAddress - startAddr
		if offset < quantity {
			if p.BoolValue {
				coilBytes[offset/8] |= (1 << (offset % 8))
			}
		}
	}

	resp := make([]byte, 2+byteCount)
	resp[0] = 0x01
	resp[1] = byte(byteCount)
	copy(resp[2:], coilBytes)
	return resp
}

func (s *ModbusTCPServer) readDiscreteInputs(data []byte) []byte {
	if len(data) < 4 {
		return s.errorResponse(0x02, 0x02)
	}
	startAddr := binary.BigEndian.Uint16(data[0:2])
	quantity := binary.BigEndian.Uint16(data[2:4])
	if quantity < 1 || quantity > 2000 {
		return s.errorResponse(0x02, 0x03)
	}

	points := s.store.GetByFunctionCodeRange(0x02, startAddr, quantity)
	byteCount := (quantity + 7) / 8
	coilBytes := make([]byte, byteCount)

	for _, p := range points {
		offset := p.RegisterAddress - startAddr
		if offset < quantity {
			if p.BoolValue {
				coilBytes[offset/8] |= (1 << (offset % 8))
			}
		}
	}

	resp := make([]byte, 2+byteCount)
	resp[0] = 0x02
	resp[1] = byte(byteCount)
	copy(resp[2:], coilBytes)
	return resp
}

func (s *ModbusTCPServer) readHoldingRegisters(data []byte) []byte {
	if len(data) < 4 {
		return s.errorResponse(0x03, 0x02)
	}
	startAddr := binary.BigEndian.Uint16(data[0:2])
	quantity := binary.BigEndian.Uint16(data[2:4])
	if quantity < 1 || quantity > 125 {
		return s.errorResponse(0x03, 0x03)
	}

	points := s.store.GetByFunctionCodeRange(0x03, startAddr, quantity)
	regBytes := make([]byte, quantity*2)

	for _, p := range points {
		offset := p.RegisterAddress - startAddr
		if offset < quantity {
			var regs []uint16
			switch p.PointType {
			case config.TypeAI, config.TypeAO:
				regs = Float32ToRegisters(float32(p.Value), s.byteOrder)
			case config.TypePI:
				regs = Int32ToRegisters(p.IntValue, s.byteOrder)
			case config.TypeDI, config.TypeDO:
				val := uint16(0)
				if p.BoolValue {
					val = 0xFFFF
				}
				regs = []uint16{val}
			}
			for i, r := range regs {
				idx := (offset + uint16(i)) - startAddr
				if idx < quantity {
					binary.BigEndian.PutUint16(regBytes[(offset+uint16(i))*2:], r)
				}
			}
		}
	}

	resp := make([]byte, 2+quantity*2)
	resp[0] = 0x03
	resp[1] = byte(quantity * 2)
	copy(resp[2:], regBytes)
	return resp
}

func (s *ModbusTCPServer) readInputRegisters(data []byte) []byte {
	if len(data) < 4 {
		return s.errorResponse(0x04, 0x02)
	}
	startAddr := binary.BigEndian.Uint16(data[0:2])
	quantity := binary.BigEndian.Uint16(data[2:4])
	if quantity < 1 || quantity > 125 {
		return s.errorResponse(0x04, 0x03)
	}

	points := s.store.GetByFunctionCodeRange(0x04, startAddr, quantity)
	regBytes := make([]byte, quantity*2)

	for _, p := range points {
		offset := p.RegisterAddress - startAddr
		if offset < quantity {
			var regs []uint16
			switch p.PointType {
			case config.TypeAI, config.TypeAO:
				regs = Float32ToRegisters(float32(p.Value), s.byteOrder)
			case config.TypePI:
				regs = Int32ToRegisters(p.IntValue, s.byteOrder)
			case config.TypeDI, config.TypeDO:
				val := uint16(0)
				if p.BoolValue {
					val = 0xFFFF
				}
				regs = []uint16{val}
			}
			for i, r := range regs {
				idx := offset + uint16(i)
				if idx < quantity {
					binary.BigEndian.PutUint16(regBytes[idx*2:], r)
				}
			}
		}
	}

	resp := make([]byte, 2+quantity*2)
	resp[0] = 0x04
	resp[1] = byte(quantity * 2)
	copy(resp[2:], regBytes)
	return resp
}

func (s *ModbusTCPServer) writeSingleCoil(data []byte) []byte {
	if len(data) < 4 {
		return s.errorResponse(0x05, 0x02)
	}
	addr := binary.BigEndian.Uint16(data[0:2])
	value := binary.BigEndian.Uint16(data[2:4])

	pt, err := s.store.GetByFunctionCodeAndAddress(0x05, addr)
	if err != nil {
		pt, err = s.store.GetByFunctionCodeAndAddress(0x01, addr)
	}
	if err != nil {
		return s.errorResponse(0x05, 0x02)
	}

	on := value == 0xFF00
	s.store.SetBoolValue(pt.IOA, on)
	s.controlCnt.Add(1)
	slog.Info("Modbus TCP 写线圈", "ioa", pt.IOA, "addr", addr, "value", on)

	resp := make([]byte, 4)
	resp[0] = 0x05
	copy(resp[1:], data[:4])
	return resp
}

func (s *ModbusTCPServer) writeSingleRegister(data []byte) []byte {
	if len(data) < 4 {
		return s.errorResponse(0x06, 0x02)
	}
	addr := binary.BigEndian.Uint16(data[0:2])
	regVal := binary.BigEndian.Uint16(data[2:4])

	pt, err := s.store.GetByFunctionCodeAndAddress(0x06, addr)
	if err != nil {
		pt, err = s.store.GetByFunctionCodeAndAddress(0x03, addr)
	}
	if err != nil {
		return s.errorResponse(0x06, 0x02)
	}

	switch pt.PointType {
	case config.TypeAI, config.TypeAO:
		regs := []uint16{regVal}
		fval := RegistersToFloat32(regs, s.byteOrder)
		s.store.SetValue(pt.IOA, float64(fval))
	case config.TypePI:
		regs := []uint16{regVal, 0}
		ival := RegistersToInt32(regs, s.byteOrder)
		s.store.SetIntValue(pt.IOA, ival)
	}
	s.controlCnt.Add(1)
	slog.Info("Modbus TCP 写寄存器", "ioa", pt.IOA, "addr", addr, "value", regVal)

	resp := make([]byte, 4)
	resp[0] = 0x06
	copy(resp[1:], data[:4])
	return resp
}

func (s *ModbusTCPServer) writeMultipleCoils(data []byte) []byte {
	if len(data) < 5 {
		return s.errorResponse(0x0F, 0x02)
	}
	startAddr := binary.BigEndian.Uint16(data[0:2])
	quantity := binary.BigEndian.Uint16(data[2:4])
	byteCount := data[4]
	if int(byteCount) != len(data)-5 {
		return s.errorResponse(0x0F, 0x02)
	}

	coilData := data[5:]
	written := 0
	for i := uint16(0); i < quantity; i++ {
		addr := startAddr + i
		on := (coilData[i/8] & (1 << (i % 8))) != 0

		pt, err := s.store.GetByFunctionCodeAndAddress(0x05, addr)
		if err != nil {
			pt, err = s.store.GetByFunctionCodeAndAddress(0x01, addr)
		}
		if err != nil {
			continue
		}
		s.store.SetBoolValue(pt.IOA, on)
		written++
	}

	s.controlCnt.Add(int64(written))
	slog.Info("Modbus TCP 写多个线圈", "start", startAddr, "quantity", quantity, "written", written)

	resp := make([]byte, 5)
	resp[0] = 0x0F
	binary.BigEndian.PutUint16(resp[1:3], startAddr)
	binary.BigEndian.PutUint16(resp[3:5], quantity)
	return resp
}

func (s *ModbusTCPServer) writeMultipleRegisters(data []byte) []byte {
	if len(data) < 5 {
		return s.errorResponse(0x10, 0x02)
	}
	startAddr := binary.BigEndian.Uint16(data[0:2])
	quantity := binary.BigEndian.Uint16(data[2:4])
	byteCount := data[4]
	if int(byteCount) != len(data)-5 {
		return s.errorResponse(0x10, 0x02)
	}

	regData := data[5:]
	written := 0
	for i := uint16(0); i < quantity; i++ {
		addr := startAddr + i
		regVal := binary.BigEndian.Uint16(regData[i*2 : i*2+2])

		pt, err := s.store.GetByFunctionCodeAndAddress(0x10, addr)
		if err != nil {
			pt, err = s.store.GetByFunctionCodeAndAddress(0x03, addr)
		}
		if err != nil {
			continue
		}

		switch pt.PointType {
		case config.TypeAI, config.TypeAO:
			regs := []uint16{regVal}
			if i+1 < quantity {
				nextVal := binary.BigEndian.Uint16(regData[(i+1)*2 : (i+1)*2+2])
				regs = []uint16{regVal, nextVal}
			}
			fval := RegistersToFloat32(regs, s.byteOrder)
			s.store.SetValue(pt.IOA, float64(fval))
		case config.TypePI:
			regs := []uint16{regVal}
			if i+1 < quantity {
				nextVal := binary.BigEndian.Uint16(regData[(i+1)*2 : (i+1)*2+2])
				regs = []uint16{regVal, nextVal}
			}
			ival := RegistersToInt32(regs, s.byteOrder)
			s.store.SetIntValue(pt.IOA, ival)
		}
		written++
	}

	s.controlCnt.Add(int64(written))
	slog.Info("Modbus TCP 写多个寄存器", "start", startAddr, "quantity", quantity, "written", written)

	resp := make([]byte, 5)
	resp[0] = 0x10
	binary.BigEndian.PutUint16(resp[1:3], startAddr)
	binary.BigEndian.PutUint16(resp[3:5], quantity)
	return resp
}

func (s *ModbusTCPServer) errorResponse(fc uint8, code uint8) []byte {
	return []byte{fc | 0x80, code}
}

func (s *ModbusTCPServer) Port() int {
	return s.port
}

func (s *ModbusTCPServer) Addr() string {
	return ":" + strconv.Itoa(s.port)
}
