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
	MainItemTagInput         ItemTag = 0b100000
	MainItemTagOutput        ItemTag = 0b100100
	MainItemTagFeature       ItemTag = 0b101100
	MainItemTagCollection    ItemTag = 0b101000
	MainItemTagEndCollection ItemTag = 0b110000

	GlobalItemTagUsagePage       ItemTag = 0b000001
	GlobalItemTagLogicalMinimum  ItemTag = 0b000101
	GlobalItemTagLogicalMaximum  ItemTag = 0b001001
	GlobalItemTagPhysicalMinimum ItemTag = 0b001101
	GlobalItemTagPhysicalMaximum ItemTag = 0b010001
	GlobalItemTagUnitExponent    ItemTag = 0b010101
	GlobalItemTagUnit            ItemTag = 0b011001
	GlobalItemTagReportSize      ItemTag = 0b011101
	GlobalItemTagReportID        ItemTag = 0b100001
	GlobalItemTagReportCount     ItemTag = 0b100101
	GlobalItemTagPush            ItemTag = 0b101001
	GlobalItemTagPop             ItemTag = 0b101101

	LocalItemTagUsage             ItemTag = 0b000010
	LocalItemTagUsageMinimum      ItemTag = 0b000110
	LocalItemTagUsageMaximum      ItemTag = 0b001010
	LocalItemTagDesignatorIndex   ItemTag = 0b001110
	LocalItemTagDesignatorMinimum ItemTag = 0b010010
	LocalItemTagDesignatorMaximum ItemTag = 0b010110
	LocalItemTagStringIndex       ItemTag = 0b011110
	LocalItemTagStringMinimum     ItemTag = 0b100010
	LocalItemTagStringMaximum     ItemTag = 0b100110
	LocalItemTagDelimiter         ItemTag = 0b101010
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
