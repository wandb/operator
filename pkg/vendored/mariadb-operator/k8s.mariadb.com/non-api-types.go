package k8s_mariadb_com

//Types used by mariadb api types that are custom to mariadb, but defined in the apiv1 package

type GaleraState struct {
	Version         string `json:"version"`
	UUID            string `json:"uuid"`
	Seqno           int    `json:"seqno"`
	SafeToBootstrap bool   `json:"safeToBootstrap"`
}
type Bootstrap struct {
	UUID  string `json:"uuid"`
	Seqno int    `json:"seqno"`
}
