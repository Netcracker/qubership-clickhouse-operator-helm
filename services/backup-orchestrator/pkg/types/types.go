package types

import "fmt"

type backupJSON struct {
	Name           string `json:"name"`
	Created        string `json:"created"`
	Size           int64  `json:"size,omitempty"`
	Location       string `json:"location"`
	RequiredBackup string `json:"required"`
	Desc           string `json:"desc"`
}

type BackupJSONs []backupJSON

type ActionRow struct {
	Command string `json:"command"`
	Status  string `json:"status"`
	Start   string `json:"start,omitempty"`
	Finish  string `json:"finish,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (row ActionRow) String() string {
	return fmt.Sprintf("Command=%s, Status=%s, Error=%s", row.Command, row.Status, row.Error)
}

type ActionsRow []ActionRow

type BackupInfo struct {
	Id         string            `json:"id"`
	Failed     bool              `json:"failed"`
	Valid      bool              `json:"valid"`
	IsGranular bool              `json:"is_granular"`
	CustomVars map[string]string `json:"custom_vars"`
}

func (backup BackupInfo) String() string {
	return fmt.Sprintf("Id=%s, Failed=%s, Valid=%s, CustomVars=%s", backup.Id, backup.Failed, backup.Valid, backup.CustomVars)
}

type BackupIdsList []string

type BackupStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Error   string `json:"err"`
}

func (s BackupStatus) String() string {
	return fmt.Sprintf("Status=%s, Message=%s, Error=%s", s.Status, s.Message, s.Error)
}

func (s BackupStatus) IsSuccessful() bool {
	return s.Status == "Successful"
}

const (
	IncrementalBackup = "incremental"
	FullBackup        = "full"
)

type ChBackup struct {
	Name string `json:"name"`
}
