// Copyright 2024-2025 NetCracker Technology Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
