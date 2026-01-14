package defaults

type Size string

const (
	SizeDev    Size = "dev"
	SizeSmall  Size = "small"
	SizeMedium Size = "medium"
	SizeLarge  Size = "large"
)

const DefaultOnDeleteRetentionPolicy = "preserve"
