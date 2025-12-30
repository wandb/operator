package common

type RetentionPolicy string

const (
	NoPolicy     RetentionPolicy = "NoPolicy"
	PurgePolicy  RetentionPolicy = "Purge"
	RetainPolicy RetentionPolicy = "Retain"
)

type CrudAction string

const (
	NoAction     CrudAction = ""
	CreateAction CrudAction = "Create"
	DeleteAction CrudAction = "Delete"
	UpdateAction CrudAction = "Update"
)
