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

package main

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"

	"os"

	"github.com/Netcracker/qubership-clickhouse-operator-helm/site-manager/pkg/scheduler"
	"github.com/Netcracker/qubership-clickhouse-operator-helm/site-manager/pkg/server"
	"github.com/Netcracker/qubership-clickhouse-operator-helm/site-manager/pkg/util"
	"go.uber.org/zap"
)

const caCertPath = "/tls/ca.crt"

var log = util.GetLogger()

func init() {
	if util.IsTlsEnabled() {
		setDefaultRootCA()
	}
}

func main() {

	go func() {
		if err := scheduler.ScheduleBackupsDownload(); err != nil {
			os.Exit(1)
		}
	}()

	if err := server.Serve(); err != nil {
		os.Exit(1)
	}
}

func setDefaultRootCA() {
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		log.Fatal("Cannot read ca file", zap.Error(err))
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		log.Fatal("Failed to append CA certificate")
	}

	http.DefaultTransport = &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:    caCertPool,
			MinVersion: tls.VersionTLS12,
			ClientAuth: tls.VerifyClientCertIfGiven,
			ClientCAs:  caCertPool,
		},
	}
}
