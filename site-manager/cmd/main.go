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
			RootCAs: caCertPool,
		},
	}
}
