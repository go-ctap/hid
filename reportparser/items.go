package reportparser

type Item[T any] interface {
	Name() string
	Size() ItemSize
	Tag() ItemTag
	Value() T
}

type UsagePage uint16

func (p UsagePage) Name() string {
	return "Usage Page"
}

func (p UsagePage) Size() ItemSize {
	return ItemSize(2)
}

func (p UsagePage) Tag() ItemTag {
	return GlobalItemTagUsagePage
}

func (p UsagePage) Value() uint16 {
	return uint16(p)
}

type Usage uint16

func (u Usage) Name() string {
	return "Usage"
}

func (u Usage) Size() ItemSize {
	return ItemSize(2)
}

func (u Usage) Tag() ItemTag {
	return LocalItemTagUsage
}

func (u Usage) Value() uint16 {
	return uint16(u)
}

type Collection CollectionItemType

func (c Collection) Name() string {
	return "Collection"
}

func (c Collection) Size() ItemSize {
	return ItemSize(1)
}

func (c Collection) Tag() ItemTag {
	return MainItemTagCollection
}

func (c Collection) Value() CollectionItemType {
	return CollectionItemType(c)
}

type EndCollection struct{}

func (e EndCollection) Name() string {
	return "End Collection"
}

func (e EndCollection) Size() ItemSize {
	return ItemSize(0)
}

func (e EndCollection) Tag() ItemTag {
	return MainItemTagEndCollection
}

func (e EndCollection) Value() struct{} {
	return struct{}{}
}

type Input InputFlags

func (i Input) Name() string {
	return "Input"
}

func (i Input) Size() ItemSize {
	return ItemSize(4)
}

func (i Input) Tag() ItemTag {
	return MainItemTagInput
}

func (i Input) Value() InputFlags {
	return InputFlags(i)
}

type Output OutputFlags

func (o Output) Name() string {
	return "Output"
}

func (o Output) Size() ItemSize {
	return ItemSize(4)
}

func (o Output) Tag() ItemTag {
	return MainItemTagOutput
}

func (o Output) Value() OutputFlags {
	return OutputFlags(o)
}

type Feature FeatureFlags

func (f Feature) Name() string {
	return "Feature"
}

func (f Feature) Size() ItemSize {
	return ItemSize(4)
}

func (f Feature) Tag() ItemTag {
	return MainItemTagFeature
}

func (f Feature) Value() FeatureFlags {
	return FeatureFlags(f)
}
