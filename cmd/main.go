package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/backup"
	terminate "github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/cancel"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/delete"
	k8sHelper "github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/helper"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/restore"
	"github.com/Netcracker/qubership-clickhouse-backup-orchestrator/pkg/utils"
	"go.uber.org/zap"
)

var (
	Log          = utils.GetLogger()
	cfg          = Config{}
	flagset      = flag.CommandLine
	isReplicator = utils.GetEnvBool("REPLICATOR", false)
)

type Config struct {
	Action        string
	BackupPath    string
	Dbs           string
	DbMap         string
	DropSrcDb     string
	AllowEviction string
}

func init() {
	flagset.StringVar(&cfg.Action, "action", "", "Set action type for Clickhouse Backup Daemon")
	flagset.StringVar(&cfg.BackupPath, "backupPath", "", "Set backupId for restore action")
	flagset.StringVar(&cfg.Dbs, "d", "", "List of Databases")
	flagset.StringVar(&cfg.DbMap, "m", "", "Mapping of  databases")
	flagset.StringVar(&cfg.DropSrcDb, "dropsrcdb", "", "Delete the source database when mapping")
	flagset.StringVar(&cfg.AllowEviction, "allowEviction", "false", "Allow eviction during backup")
}

func main() {
	if err := flagset.Parse(os.Args[1:]); err != nil {
		Log.Error(fmt.Sprintf("there is an error during flagset Parse %v", zap.Error(err)))
		os.Exit(1)
	}

	client, err := utils.GetClient()
	if err != nil {
		Log.Error(fmt.Sprintf("there is an error during k8s client assembly %v", zap.Error(err)))
		os.Exit(1)
	}

	httpClient := http.Client{}
	if utils.IsTlsEnabled() {
		httpClient = http.Client{
			Transport: &http.Transport{
				TLSClientConfig: utils.GetTlsConfig(),
			},
		}
	}
	helper := k8sHelper.Init(client, httpClient)
	switch cfg.Action {
	case "backup":
		Log.Info("backup is called")
		backupProcedure := &backup.Backup{
			Helper:        helper,
			Log:           Log,
			RawDbs:        cfg.Dbs,
			BackupPath:    cfg.BackupPath,
			AllowEviction: cfg.AllowEviction,
		}

		if err := backupProcedure.PerformBackup(); err != nil {
			Log.Error("can't perform backup", zap.Error(err))
			os.Exit(1)
		}
		Log.Info("backup successful")
		os.Exit(0)
	case "restore":
		Log.Info("restore is called")

		restoreProcedure := &restore.Restore{
			Helper:     helper,
			Log:        Log,
			RawDbs:     cfg.Dbs,
			BackupPath: cfg.BackupPath,
			ResetCache: isReplicator,
			DbMap:      cfg.DbMap,
			DropSrcDb:  cfg.DropSrcDb,
		}
		if err := restoreProcedure.PerformRestore(); err != nil {
			Log.Error("can't perform restore", zap.Error(err))
			os.Exit(1)
		}
		Log.Info("restore successful")
		os.Exit(0)
	case "delete":
		deleteAction(helper, isReplicator)
	case "incrementalDelete":
		deleteAction(helper, false)
	case "dblist":
		Log.Info("list is called")
		dbListProcedure := &backup.DbList{
			Log:        Log,
			BackupPath: cfg.BackupPath,
		}
		if err := dbListProcedure.GetDbList(); err != nil {
			Log.Error("can't get databases list", zap.Error(err))
			os.Exit(1)
		}
		Log.Info("list successful")
		os.Exit(0)
	case "incremental-backup":
		Log.Info("incremental backup called")

		backupProcedure := &backup.Backup{
			Helper:     helper,
			Log:        Log,
			RawDbs:     cfg.Dbs,
			BackupPath: cfg.BackupPath,
		}

		if err := backupProcedure.PerformIncrementalBackup(); err != nil {
			Log.Error("can't perform incremental backup", zap.Error(err))
			os.Exit(1)
		}
		Log.Info("backup successful")
		os.Exit(0)
	case "cancel":
		Log.Info("cancel called")
		terminateProcedure := &terminate.Terminate{
			Helper:     helper,
			Log:        Log,
			BackupPath: cfg.BackupPath,
		}
		if err := terminateProcedure.KillAction(); err != nil {
			Log.Error("can't cancel action", zap.Error(err))
			os.Exit(1)
		}
	}
}

func deleteAction(helper *k8sHelper.Helper, fullEvict bool) {
	Log.Info("delete is called")
	deleteProcedure := &delete.Delete{
		Helper:     helper,
		Log:        Log,
		RawDbs:     cfg.Dbs,
		BackupPath: cfg.BackupPath,
		FullEvict:  fullEvict,
	}
	if err := deleteProcedure.PerformDelete(); err != nil {
		Log.Error("can't perform restore", zap.Error(err))
		os.Exit(1)
	}
	Log.Info("delete successful")
	os.Exit(0)
}
