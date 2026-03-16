package server

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/Netcracker/qubership-clickhouse-backup-sidecar/pkg/util"
)

var log = util.GetLogger()

func InitBackupSenderServer() {
	log.Info("start")
	http.HandleFunc("/nfssync", nfssync)   // each request calls handler
	http.HandleFunc("/delete", deletePath) // each request calls handler

	if util.IsTlsEnabled() {
		log.Info(fmt.Sprint(http.ListenAndServeTLS(":7172", "/etc/clickhouse-server/certs/tls.crt", "/etc/clickhouse-server/certs/tls.key", nil)))
	} else {
		log.Info(fmt.Sprint(http.ListenAndServe(":7172", nil)))
	}
}

type sender struct {
	backupid    string `json:"backupid"`
	destination string `json:"destination"`
}

func nfssync(response http.ResponseWriter, req *http.Request) {

	var err error
	query := req.URL.Query()
	backupid := query.Get("backupid")
	if backupid == "" {
		response.WriteHeader(400)
		_, _ = fmt.Fprintf(response, "Error. backupid parameter not found")
		return
	}
	remoteBackup := query.Get("remoteBackup")
	if remoteBackup == "" {
		response.WriteHeader(400)
		_, _ = fmt.Fprintf(response, "Error. remoteBackup parameter not found")
		return
	}
	nfsPath := util.GetEnv("NFS_MOUNT_POINT", "")
	if nfsPath == "" {
		response.WriteHeader(400)
		_, _ = fmt.Fprintf(response, "Error NFS_MOUNT_POINT cannot be empty.")
		return
	}
	if !strings.HasPrefix(remoteBackup, nfsPath) {
		remoteBackup = nfsPath + "/" + remoteBackup
	}
	backupDir := util.GetEnv("BACKUP_DIR", "/var/lib/clickhouse/backup/")
	backupPath := backupDir + "/" + backupid
	log.Info(fmt.Sprintf("backup  %s  remoteBackup:%s", backupPath, remoteBackup))

	switch req.Method {
	case "GET":
		log.Debug(" GET")
		err = runrsync(remoteBackup, backupPath)
	case "POST":
		log.Debug("POST")
		err = runrsync(backupPath, remoteBackup) //--remove-source-files
		//if err == nil {
		//	//log.Info(fmt.Sprintf("Deleting file %s", backupPath))
		//	//err = os.Remove(backupPath)
		//}
	default:
		log.Debug("Response 400. Only GET and POST methods are supported")
		response.WriteHeader(400)
		_, _ = fmt.Fprintf(response, "Only GET and POST methods are supported.")
	}

	if err != nil {
		log.Error("Response 400.")
		response.WriteHeader(400)
		_, _ = fmt.Fprintf(response, fmt.Sprintf("%s", err))
	} else {
		log.Debug("Response 200.")
		response.WriteHeader(200)
		_, _ = fmt.Fprintf(response, "Backup successfully transferred")
	}
}

func runrsync(source string, dst string) error {
	if err := os.MkdirAll(dst, 0777); err != nil {
		log.Error(fmt.Sprintf("Error create reciver path.   %s", err))
		return err
	}
	dst = strings.TrimSuffix(dst, "/")
	cmd := exec.Command("rsync", "-a", "-H", "--delete", "--progress", "--numeric-ids", source+"/", dst)
	log.Info(fmt.Sprintf("%s", cmd))
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		log.Error(errb.String())
		return err
	}
	log.Info(outb.String())
	return nil
}

func deletePath(response http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	path := query.Get("path")
	nfsPath := util.GetEnv("NFS_MOUNT_POINT", "")
	if nfsPath == "" {
		response.WriteHeader(400)
		_, _ = fmt.Fprintf(response, "Error NFS_MOUNT_POINT cannot be empty.")
		return
	}
	if !strings.HasPrefix(path, nfsPath) {
		path = nfsPath + "/" + path
	}
	log.Info(fmt.Sprintf("backup %s will be deleted. NFS_MOUNT_POINT:%s", path, nfsPath))
	err := os.Remove(path)
	if err != nil {
		log.Error(fmt.Sprintf("failed to delete. error:%s", err))
		response.WriteHeader(500)
		_, _ = fmt.Fprintf(response, fmt.Sprintf("%s", err))
	} else {
		response.WriteHeader(200)
		_, _ = fmt.Fprintf(response, fmt.Sprintf("%s successfully delete", path))
	}
}
