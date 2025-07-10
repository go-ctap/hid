package reportparser

type Item[T any] interface {
	Name() string
	Tag() ItemTag
	Value() T
}

type UsagePage uint16

func (p UsagePage) Name() string {
	return "Usage Page"
}

func (p UsagePage) Tag() ItemTag {
	return ItemTagGlobalUsagePage
}

func (p UsagePage) Value() uint16 {
	return uint16(p)
}

type LogicalMinimum uint32

func (p LogicalMinimum) Name() string {
	return "Logical Minimum"
}

func (p LogicalMinimum) Tag() ItemTag {
	return ItemTagGlobalLogicalMinimum
}

func (p LogicalMinimum) Value() uint32 {
	return uint32(p)
}

type LogicalMaximum uint32

func (p LogicalMaximum) Name() string {
	return "Logical Maximum"
}

func (p LogicalMaximum) Tag() ItemTag {
	return ItemTagGlobalLogicalMaximum
}

func (p LogicalMaximum) Value() uint32 {
	return uint32(p)
}

type Usage uint16

func (u Usage) Name() string {
	return "Usage"
}

func (u Usage) Tag() ItemTag {
	return ItemTagLocalUsage
}

func (u Usage) Value() uint16 {
	return uint16(u)
}

type Input InputFlags

func (i Input) Name() string {
	return "Input"
}

func (i Input) Tag() ItemTag {
	return ItemTagMainInput
}

func (i Input) Value() InputFlags {
	return InputFlags(i)
}

type Output OutputFlags

func (o Output) Name() string {
	return "Output"
}

func (o Output) Tag() ItemTag {
	return ItemTagMainOutput
}

func (o Output) Value() OutputFlags {
	return OutputFlags(o)
}

type Feature FeatureFlags

func (f Feature) Name() string {
	return "Feature"
}

func (f Feature) Tag() ItemTag {
	return ItemTagMainFeature
}

func (f Feature) Value() FeatureFlags {
	return FeatureFlags(f)
}

type Collection CollectionItemType

func (c Collection) Name() string {
	return "Collection"
}

func (c Collection) Tag() ItemTag {
	return ItemTagMainCollection
}

func (c Collection) Value() CollectionItemType {
	return CollectionItemType(c)
}

type EndCollection struct{}

func (e EndCollection) Name() string {
	return "End Collection"
}

func (e EndCollection) Tag() ItemTag {
	return ItemTagMainEndCollection
}

func (e EndCollection) Value() struct{} {
	return struct{}{}
}

type ReportSize byte

func (r ReportSize) Name() string {
	return "Report Size"
}

func (r ReportSize) Tag() ItemTag {
	return ItemTagGlobalReportSize
}

func (r ReportSize) Value() ReportSize {
	return r
}

type ReportID byte

func (r ReportID) Name() string {
	return "Report ID"
}

func (r ReportID) Tag() ItemTag {
	return ItemTagGlobalReportID
}

func (r ReportID) Value() ReportID {
	return r
}

type ReportCount byte

func (r ReportCount) Name() string {
	return "Report Count"
}

func (r ReportCount) Tag() ItemTag {
	return ItemTagGlobalReportCount
}

func (r ReportCount) Value() ReportCount {
	return r
}
