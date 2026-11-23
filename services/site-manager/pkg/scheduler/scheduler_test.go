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

package scheduler

import (
	"errors"
	"reflect"
	"testing"
)

func TestSelectBackupChain(t *testing.T) {
	tests := []struct {
		name       string
		backups    []string
		latestFull string
		want       []string
	}{
		{
			name: "duplicates from full and incremental lists are removed, stale backups before latest full are dropped",
			backups: []string{
				"20261120T084500", "20261121T173000", "20261122T173335",
				"20261120T084500", "20261121T173000", "20261122T173335",
			},
			latestFull: "20261122T173335",
			want:       []string{"20261122T173335"},
		},
		{
			name:       "incrementals after latest full are kept in chronological order",
			backups:    []string{"20261122T180000", "20261122T170000", "20261122T173000", "20261122T174500"},
			latestFull: "20261122T173000",
			want:       []string{"20261122T173000", "20261122T174500", "20261122T180000"},
		},
		{
			name:       "no latest full backup - keep all, deduplicated and sorted",
			backups:    []string{"20261121T000000", "20261120T000000", "20261121T000000"},
			latestFull: "",
			want:       []string{"20261120T000000", "20261121T000000"},
		},
		{
			name:       "empty list",
			backups:    nil,
			latestFull: "20261122T173335",
			want:       []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectBackupChain(tt.backups, tt.latestFull)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("selectBackupChain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNotFoundOnRemoteStorage(t *testing.T) {
	if isNotFoundOnRemoteStorage(nil) {
		t.Error("nil error must not be treated as not found")
	}
	notFound := errors.New("desired action ends with error: '20261120T084500_chi-cluster-replicated-0-0' is not found on remote storage. exiting")
	if !isNotFoundOnRemoteStorage(notFound) {
		t.Error("expected not found error to be detected")
	}
	if isNotFoundOnRemoteStorage(errors.New("desired action ends with error: connection refused. exiting")) {
		t.Error("other errors must not be treated as not found")
	}
}
