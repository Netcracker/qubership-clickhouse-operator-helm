package basic

import (
	"context"
	"sync"

	"github.com/Netcracker/qubership-clickhouse-dbaas-adapter/adapter/cluster"
	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/dao"
)

type ClickhouseServiceAdapter struct {
	Ctx context.Context
	cluster.ClusterAdapter
	dao.ApiVersion
	roles    []string
	features map[string]bool
	Mutex    *sync.Mutex
	Generator
}
