package reportparser

import (
	"encoding/binary"
)

type Items []any

func ParseReport(b []byte) Items {
	r := make(Items, 0)

	for i := 0; i < len(b); {
		size := ItemSize(b[i] & 0b00000011)
		tag := ItemTag((b[i] & 0b11111100) >> 2)

		i++

		switch tag {
		case ItemTagMainInput:
			r = append(r, Input(parseUintValue(size, b[i:])))
		case ItemTagMainOutput:
			r = append(r, Output(parseUintValue(size, b[i:])))
		case ItemTagMainFeature:
			r = append(r, Feature(parseUintValue(size, b[i:])))
		case ItemTagMainCollection:
			r = append(r, Collection(b[i]))
		case ItemTagMainEndCollection:
			r = append(r, EndCollection{})
		case ItemTagGlobalUsagePage:
			r = append(r, UsagePage(parseUintValue(size, b[i:])))
		case ItemTagGlobalLogicalMinimum:
			r = append(r, LogicalMinimum(parseUintValue(size, b[i:])))
		case ItemTagGlobalLogicalMaximum:
			r = append(r, LogicalMaximum(parseUintValue(size, b[i:])))
		case ItemTagGlobalPhysicalMinimum:
		case ItemTagGlobalPhysicalMaximum:
		case ItemTagGlobalUnitExponent:
		case ItemTagGlobalUnit:
		case ItemTagGlobalReportSize:
			r = append(r, ReportSize(b[i]))
		case ItemTagGlobalReportID:
			r = append(r, ReportID(b[i]))
		case ItemTagGlobalReportCount:
			r = append(r, ReportCount(b[i]))
		case ItemTagGlobalPush:
		case ItemTagGlobalPop:
		case ItemTagLocalUsage:
			r = append(r, Usage(parseUintValue(size, b[i:])))
		case ItemTagLocalUsageMinimum:
		case ItemTagLocalUsageMaximum:
		case ItemTagLocalDesignatorIndex:
		case ItemTagLocalDesignatorMinimum:
		case ItemTagLocalDesignatorMaximum:
		case ItemTagLocalStringIndex:
		case ItemTagLocalStringMinimum:
		case ItemTagLocalStringMaximum:
		case ItemTagLocalDelimiter:
		}

		// skip read bytes
		switch size {
		case ItemSize0:
			i += 0
		case ItemSize8:
			i += 1
		case ItemSize16:
			i += 2
		case ItemSize32:
			i += 4
		}
	}

	return r
}

func parseUintValue(size ItemSize, buf []byte) uint32 {
	switch size {
	case ItemSize0:
		return 0
	case ItemSize8:
		return uint32(buf[0])
	case ItemSize16:
		return uint32(binary.LittleEndian.Uint16(buf))
	case ItemSize32:
		return binary.LittleEndian.Uint32(buf)
	}
	return 0
}
