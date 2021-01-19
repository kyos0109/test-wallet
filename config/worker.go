package config

type worker struct {
	DBSyncNums int
}

var (
	Worker = &worker{DBSyncNums: 50}
)
