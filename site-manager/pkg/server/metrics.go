package server

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Netcracker/qubership-clickhouse-operator-helm/site-manager/pkg/util"
)

const (
	RestoreProgresMetric = "clickhouse_dr_restore_in_progress"
	RestoreFailureMetric = "clickhouse_dr_restore_failed"
)

var lastRestore = Restore{}

type Restore struct {
	inProgress     bool
	failed         bool
	failureMessage string
	failureTime    string
	sync.Mutex
}

func (r *Restore) setFailed(message string) {
	r.Lock()
	defer r.Unlock()

	r.inProgress = false
	r.failed = true
	r.failureMessage = message
	r.failureTime = time.Now().String()
}

func (r *Restore) stop() {
	r.inProgress = false
}

func (r *Restore) start() {
	r.Lock()
	defer r.Unlock()

	r.inProgress = true
	r.failed = false
	r.failureMessage = ""
	r.failureTime = ""
}

func (smHandler *siteManagerHandler) Metrics(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case "GET":
		lastRestore.Lock()
		defer lastRestore.Unlock()

		statusMetric := fmt.Sprintf("%s{} %d\n", RestoreProgresMetric, util.BoolToInt(lastRestore.inProgress))
		sendResponseStr(writer, statusMetric)

		failedMetric := fmt.Sprintf("%s{failureMessage=\"%s\", failureTime=\"%s\"} %d\n",
			RestoreFailureMetric,
			lastRestore.failureMessage,
			lastRestore.failureTime,
			util.BoolToInt(lastRestore.failed))
		sendResponseStr(writer, failedMetric)
	}
}
