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

// Package declarative serves the physical database descriptor consumed by the DBaaS
// Operator when physical database registration is driven declaratively through a
// PhysicalDatabase custom resource instead of adapter self-registration.
package declarative

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"go.uber.org/zap"

	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/dao"
	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/service"
)

const DescriptorPath = "/api/v2/adapter/physical_database"
const requiredApiVersion = dao.ApiVersion("v2")

type Descriptor struct {
	PhysicalDatabaseId string            `json:"physicalDatabaseId"`
	Type               string            `json:"type"`
	Labels             map[string]string `json:"labels,omitempty"`
	ApiVersions        dao.ApiVersions   `json:"apiVersions"`
	Features           map[string]bool   `json:"features,omitempty"`
	SupportedRoles     []string          `json:"supportedRoles,omitempty"`
	ReadOnlyHost       string            `json:"readOnlyHost,omitempty"`
}

type DescriptorService struct {
	physicalDatabaseId string
	databaseType       string
	labels             map[string]string
	adminService       service.CoreAdministrationServiceIface
	logger             *zap.Logger
}

func NewDescriptorService(
	physicalDatabaseId string,
	databaseType string,
	labels map[string]string,
	adminService service.CoreAdministrationServiceIface,
	logger *zap.Logger,
) *DescriptorService {
	return &DescriptorService{
		physicalDatabaseId: physicalDatabaseId,
		databaseType:       databaseType,
		labels:             labels,
		adminService:       adminService,
		logger:             logger,
	}
}

func (s *DescriptorService) Descriptor() (*Descriptor, error) {
	if s.physicalDatabaseId == "" {
		return nil, fmt.Errorf("physical database identifier is not configured")
	}

	if version := s.adminService.GetVersion(); version != requiredApiVersion {
		return nil, fmt.Errorf(
			"declarative registration requires adapter API %s, but %q is in use",
			requiredApiVersion, version,
		)
	}

	roles := s.adminService.GetSupportedRoles()
	if len(roles) == 0 {
		return nil, fmt.Errorf("no supported roles are available")
	}

	return &Descriptor{
		PhysicalDatabaseId: s.physicalDatabaseId,
		Type:               s.databaseType,
		Labels:             s.labels,
		ApiVersions: dao.ApiVersions{Specs: []dao.ApiVersionsSpec{
			{
				SpecRootUrl:     dao.RootUrl,
				Major:           dao.MajorAPIVersion,
				Minor:           dao.MinorAPIVersion,
				SupportedMajors: dao.SupportedMajorsVersions,
			},
		}},
		Features:       s.adminService.GetFeatures(),
		SupportedRoles: roles,
		ReadOnlyHost:   s.adminService.GetROHost(),
	}, nil
}

func (s *DescriptorService) Handler(c *fiber.Ctx) error {
	descriptor, err := s.Descriptor()
	if err != nil {
		s.logger.Error(fmt.Sprintf("Cannot serve physical database descriptor: %v", err))
		return c.Status(fiber.StatusServiceUnavailable).SendString(err.Error())
	}
	return c.JSON(descriptor)
}

func (s *DescriptorService) RegisterHandlers(app *fiber.App, user string, pass string) {
	app.Get(DescriptorPath, basicauth.New(basicauth.Config{
		Realm: "This API is for using by dbaas operator only",
		Users: map[string]string{
			user: pass,
		},
	}), s.Handler)
	s.logger.Info(fmt.Sprintf("Physical database descriptor endpoint registered at %s", DescriptorPath))
}
