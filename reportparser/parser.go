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
		case MainItemTagInput:
			r = append(r, Input(hidValue(size, b[i:])))
		case MainItemTagOutput:
			r = append(r, Output(hidValue(size, b[i:])))
		case MainItemTagFeature:
			r = append(r, Feature(hidValue(size, b[i:])))
		case MainItemTagCollection:
			r = append(r, Collection(b[i]))
		case MainItemTagEndCollection:
			r = append(r, EndCollection{})
		case GlobalItemTagUsagePage:
			r = append(r, UsagePage(hidValue(size, b[i:])))
		case GlobalItemTagLogicalMinimum:
		case GlobalItemTagLogicalMaximum:
		case GlobalItemTagPhysicalMinimum:
		case GlobalItemTagPhysicalMaximum:
		case GlobalItemTagUnitExponent:
		case GlobalItemTagUnit:
		case GlobalItemTagReportSize:
		case GlobalItemTagReportID:
		case GlobalItemTagReportCount:
		case GlobalItemTagPush:
		case GlobalItemTagPop:
		case LocalItemTagUsage:
			r = append(r, Usage(hidValue(size, b[i:])))
		case LocalItemTagUsageMinimum:
		case LocalItemTagUsageMaximum:
		case LocalItemTagDesignatorIndex:
		case LocalItemTagDesignatorMinimum:
		case LocalItemTagDesignatorMaximum:
		case LocalItemTagStringIndex:
		case LocalItemTagStringMinimum:
		case LocalItemTagStringMaximum:
		case LocalItemTagDelimiter:
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

func hidValue(size ItemSize, buf []byte) uint32 {
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
