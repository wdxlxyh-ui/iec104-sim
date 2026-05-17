package modbus

import "math"

func Float32ToRegisters(val float32, byteOrder string) []uint16 {
	bits := math.Float32bits(val)
	b := make([]byte, 4)
	b[0] = byte(bits >> 24)
	b[1] = byte(bits >> 16)
	b[2] = byte(bits >> 8)
	b[3] = byte(bits)
	return applyByteOrder(b, byteOrder)
}

func RegistersToFloat32(regs []uint16, byteOrder string) float32 {
	b := reorderBytes([]byte{
		byte(regs[0] >> 8), byte(regs[0]),
		byte(regs[1] >> 8), byte(regs[1]),
	}, byteOrder)
	bits := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	return math.Float32frombits(bits)
}

func Int32ToRegisters(val int32, byteOrder string) []uint16 {
	b := make([]byte, 4)
	b[0] = byte(val >> 24)
	b[1] = byte(val >> 16)
	b[2] = byte(val >> 8)
	b[3] = byte(val)
	return applyByteOrder(b, byteOrder)
}

func RegistersToInt32(regs []uint16, byteOrder string) int32 {
	b := reorderBytes([]byte{
		byte(regs[0] >> 8), byte(regs[0]),
		byte(regs[1] >> 8), byte(regs[1]),
	}, byteOrder)
	return int32(b[0])<<24 | int32(b[1])<<16 | int32(b[2])<<8 | int32(b[3])
}

func applyByteOrder(b []byte, order string) []uint16 {
	r := reorderBytes(b, order)
	return []uint16{uint16(r[0])<<8 | uint16(r[1]), uint16(r[2])<<8 | uint16(r[3])}
}

func reorderBytes(b []byte, order string) []byte {
	r := make([]byte, 4)
	copy(r, b)
	switch order {
	case "CDAB":
		r[0], r[1], r[2], r[3] = b[2], b[3], b[0], b[1]
	case "BADC":
		r[0], r[1], r[2], r[3] = b[1], b[0], b[3], b[2]
	case "DCBA":
		r[0], r[1], r[2], r[3] = b[3], b[2], b[1], b[0]
	}
	return r
}
