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

package basic

import (
	"context"
	"fmt"

	"github.com/Netcracker/qubership-clickhouse-dbaas-adapter/adapter/cluster"
	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/dao"
	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/utils"
)

const (
	FeatureMultiUsers = "multiusers"
	FeatureTls        = "tls"

	AdminRole = "admin"
	RORole    = "ro"
	RWRole    = "rw"
)

func (c ClickhouseServiceAdapter) grantUsersAccordingRoles(conn cluster.Conn, dbName string, users map[string]string) error {
	for user, role := range users {
		if role == AdminRole {
			_, err := conn.Exec(grantAdminQuery(dbName, user))
			if err != nil {
				return err
			}
			_, err = conn.Exec(GrantAdminRemoteQuery(user))
			if err != nil {
				return err
			}
			_, err = conn.Exec(GrantDictionaryReloadQuery(user))
			if err != nil {
				return err
			}
		} else if role == RORole {
			_, err := conn.Exec(grantROQuery(dbName, user))
			if err != nil {
				return err
			}
		} else if role == RWRole {
			_, err := conn.Exec(grantRWQuery(dbName, user))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c ClickhouseServiceAdapter) CreateRoles(ctx context.Context, roles []dao.AdditionalRole) (success []dao.Success, failure *dao.Failure) {
	log := utils.GetLogger(false)
	utils.AddLoggerContext(log, ctx)

	var additionalRoleIdInProcess string

	defer func() {
		if r := recover(); r != nil {
			log.Error(fmt.Sprintf("error during additional roles creation %s", r))
			if failure == nil {
				failure = &dao.Failure{
					Id:      additionalRoleIdInProcess,
					Message: fmt.Sprintf("%s", r),
				}
			}
		}
	}()

	for _, additionalRole := range roles {
		existingRoles := make([]string, 0)
		additionalRoleIdInProcess = additionalRole.Id
		dbName := additionalRole.DbName

		for _, connectionProperties := range additionalRole.ConnectionProperties {
			roleForCheck := connectionProperties["role"].(string) //TODO
			existingRoles = append(existingRoles, roleForCheck)
		}
		newConProps := make([]dao.ConnectionProperties, 0)
		newResources := make([]dao.DbResource, 0)
		for _, role := range c.GetSupportedRoles() {
			if !Contains(existingRoles, role) {
				userCreateRequest := dao.UserCreateRequest{
					DbName: dbName,
					Role:   role,
				}

				createdUser, err := c.CreateUserWithError(ctx, "", userCreateRequest)
				if err != nil {
					failure = &dao.Failure{
						Id:      additionalRoleIdInProcess,
						Message: err.Error(),
					}
					break
				}

				newConProps = append(newConProps, createdUser.ConnectionProperties)
				newResources = append(newResources, createdUser.Resources...)
			}
		}

		success = append(success, dao.Success{
			Id:                   additionalRole.Id,
			ConnectionProperties: newConProps,
			Resources:            newResources,
			DbName:               dbName,
		})
	}

	return success, failure
}
