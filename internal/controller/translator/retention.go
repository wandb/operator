package translator

type OnDeletePolicy string

const (
	Purge    OnDeletePolicy = "purge"
	Preserve OnDeletePolicy = "preserve"
)
