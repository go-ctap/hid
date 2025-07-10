//go:generate stringer -type=ItemSize,ItemType,ItemTag,CollectionItemType,InputFlags,OutputFlags,FeatureFlags -output=consts_string.go
package reportparser

type ItemSize uint8

const (
	ItemSize0 ItemSize = iota
	ItemSize8
	ItemSize16
	ItemSize32
)

type ItemType uint8

const (
	ItemTypeMain ItemType = iota
	ItemTypeGlobal
	ItemTypeLocal
	ItemTypeReserved
)

type ItemTag uint8

const (
	ItemTagMainInput         ItemTag = 0b100000
	ItemTagMainOutput        ItemTag = 0b100100
	ItemTagMainFeature       ItemTag = 0b101100
	ItemTagMainCollection    ItemTag = 0b101000
	ItemTagMainEndCollection ItemTag = 0b110000

	ItemTagGlobalUsagePage       ItemTag = 0b000001
	ItemTagGlobalLogicalMinimum  ItemTag = 0b000101
	ItemTagGlobalLogicalMaximum  ItemTag = 0b001001
	ItemTagGlobalPhysicalMinimum ItemTag = 0b001101
	ItemTagGlobalPhysicalMaximum ItemTag = 0b010001
	ItemTagGlobalUnitExponent    ItemTag = 0b010101
	ItemTagGlobalUnit            ItemTag = 0b011001
	ItemTagGlobalReportSize      ItemTag = 0b011101
	ItemTagGlobalReportID        ItemTag = 0b100001
	ItemTagGlobalReportCount     ItemTag = 0b100101
	ItemTagGlobalPush            ItemTag = 0b101001
	ItemTagGlobalPop             ItemTag = 0b101101

	ItemTagLocalUsage             ItemTag = 0b000010
	ItemTagLocalUsageMinimum      ItemTag = 0b000110
	ItemTagLocalUsageMaximum      ItemTag = 0b001010
	ItemTagLocalDesignatorIndex   ItemTag = 0b001110
	ItemTagLocalDesignatorMinimum ItemTag = 0b010010
	ItemTagLocalDesignatorMaximum ItemTag = 0b010110
	ItemTagLocalStringIndex       ItemTag = 0b011110
	ItemTagLocalStringMinimum     ItemTag = 0b100010
	ItemTagLocalStringMaximum     ItemTag = 0b100110
	ItemTagLocalDelimiter         ItemTag = 0b101010
)

func (t ItemTag) Type() ItemType {
	typ := t & 0b000011
	switch typ {
	case 0b00:
		return ItemTypeMain
	case 0b01:
		return ItemTypeGlobal
	case 0b10:
		return ItemTypeLocal
	default:
		return ItemTypeReserved
	}
}

type CollectionItemType byte

const (
	CollectionItemTypePhysical CollectionItemType = iota
	CollectionItemTypeApplication
	CollectionItemTypeLogical
	CollectionItemTypeReport
	CollectionItemTypeNamedArray
	CollectionItemTypeUsageSwitch
	CollectionItemTypeUsageModifier
)

type InputFlags uint32

const (
	InputFlagConstant InputFlags = 1 << iota
	InputFlagVariable
	InputFlagRelative
	InputFlagWrap
	InputFlagNonLinear
	InputFlagNoPreferred
	InputFlagNullState
	_
	_
	InputFlagBufferedBytes
)

type OutputFlags uint32

const (
	OutputFlagConstant OutputFlags = 1 << iota
	OutputFlagVariable
	OutputFlagRelative
	OutputFlagWrap
	OutputFlagNonLinear
	OutputFlagNoPreferred
	OutputFlagNullState
	OutputFlagVolatile
	_
	OutputFlagBufferedBytes
)

type FeatureFlags uint32

const (
	FeatureFlagConstant FeatureFlags = 1 << iota
	FeatureFlagVariable
	FeatureFlagRelative
	FeatureFlagWrap
	FeatureFlagNonLinear
	FeatureFlagNoPreferred
	FeatureFlagNullState
	FeatureFlagVolatile
	_
	FeatureFlagBufferedBytes
)
