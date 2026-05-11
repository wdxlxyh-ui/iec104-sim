package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

func LoadFromXLSX(path string) ([]*Point, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	sheet := "point"
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("get rows from sheet %q: %w", sheet, err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("sheet %q has no data rows", sheet)
	}

	var points []*Point
	// 不同测点类型（AI/AO/DI/DO）可以复用相同的 IOA 地址空间
	seen := make(map[string]bool)

	for i, row := range rows[1:] {
		if len(row) < 6 {
			continue
		}

		name := strings.TrimSpace(row[0])
		if name == "" {
			continue
		}

		ioaStr := strings.TrimSpace(row[1])
		ioa, err := strconv.ParseUint(ioaStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid IOA %q: %w", i+2, ioaStr, err)
		}

		ptRaw := strings.TrimSpace(row[3])
		var pt PointType
		switch strings.ToUpper(ptRaw) {
		case "AI":
			pt = TypeAI
		case "DI":
			pt = TypeDI
		case "PI":
			pt = TypePI
		case "DO":
			pt = TypeDO
		case "AO":
			pt = TypeAO
		default:
			return nil, fmt.Errorf("row %d: unknown point-type %q", i+2, ptRaw)
		}

		key := string(pt) + ":" + ioaStr
		if seen[key] {
			return nil, fmt.Errorf("row %d: duplicate %s IOA %d", i+2, pt, ioa)
		}
		seen[key] = true

		vtRaw := strings.TrimSpace(row[2])
		var vt ValueType
		switch strings.ToUpper(vtRaw) {
		case "FLOAT":
			vt = VTFloat
		case "DOUBLE":
			vt = VTDouble
		case "INT":
			vt = VTInt
		case "BIT":
			vt = VTBit
		default:
			vt = VTFloat
		}

		efficient := 1.0
		if len(row) > 4 {
			eStr := strings.TrimSpace(row[4])
			if eStr != "" {
				efficient, err = strconv.ParseFloat(eStr, 64)
				if err != nil {
					return nil, fmt.Errorf("row %d: invalid efficient %q: %w", i+2, eStr, err)
				}
			}
		}

		baseValue := 0.0
		if len(row) > 5 {
			bStr := strings.TrimSpace(row[5])
			if bStr != "" {
				baseValue, err = strconv.ParseFloat(bStr, 64)
				if err != nil {
					return nil, fmt.Errorf("row %d: invalid base-value %q: %w", i+2, bStr, err)
				}
			}
		}

		alias := ""
		if len(row) > 6 {
			alias = strings.TrimSpace(row[6])
		}

		p := &Point{
			IOA:       uint32(ioa),
			Name:      name,
			ValueType: vt,
			PointType: pt,
			Efficient: efficient,
			BaseValue: baseValue,
			Alias:     alias,
		}

		switch pt {
		case TypeAI, TypeAO:
			p.Value = baseValue * efficient
		case TypeDI, TypeDO:
			p.BoolValue = int64(baseValue) != 0
		case TypePI:
			p.IntValue = int32(baseValue)
		}

		points = append(points, p)
	}

	return points, nil
}
