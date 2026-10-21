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

package declarative

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/dao"
)

const (
	testUser = "dbaas-aggregator"
	testPass = "dbaas-aggregator-password"
)

// stubAdminService implements service.CoreAdministrationServiceIface with only the
// metadata accessors the descriptor depends on.
type stubAdminService struct {
	version  dao.ApiVersion
	roles    []string
	features map[string]bool
	roHost   string
}

func (s *stubAdminService) GetVersion() dao.ApiVersion            { return s.version }
func (s *stubAdminService) GetSupportedRoles() []string           { return s.roles }
func (s *stubAdminService) GetFeatures() map[string]bool          { return s.features }
func (s *stubAdminService) GetROHost() string                     { return s.roHost }
func (s *stubAdminService) GetDatabases(context.Context) []string { return nil }
func (s *stubAdminService) PreStart()                             {}

func (s *stubAdminService) CreateDatabase(context.Context, dao.DbCreateRequest) (any, error) {
	return nil, nil
}

func (s *stubAdminService) DropResources(context.Context, []dao.DbResource) (*[]dao.DbResource, bool) {
	return nil, false
}

func (s *stubAdminService) UpdateMetadata(context.Context, map[string]any, string) {}

func (s *stubAdminService) DescribeDatabases(context.Context, []string, bool, bool) map[string]any {
	return nil
}

func (s *stubAdminService) GetDefaultCreateRequest() dao.DbCreateRequest {
	return dao.DbCreateRequest{}
}

func (s *stubAdminService) GetDefaultUserCreateRequest() dao.UserCreateRequest {
	return dao.UserCreateRequest{}
}

func (s *stubAdminService) CreateUser(context.Context, string, dao.UserCreateRequest) (*dao.CreatedUser, error) {
	return nil, nil
}

func (s *stubAdminService) MigrateToVault(context.Context, string, string) (string, error) {
	return "", nil
}

func (s *stubAdminService) CreateRoles(context.Context, []dao.AdditionalRole) ([]dao.Success, *dao.Failure) {
	return nil, nil
}

func supportedAdminService() *stubAdminService {
	return &stubAdminService{
		version:  "v2",
		roles:    []string{"admin", "rw", "ro"},
		features: map[string]bool{"multiusers": true, "tls": false},
	}
}

func newTestApp(phydbid string, labels map[string]string, admin *stubAdminService) *fiber.App {
	app := fiber.New()
	NewDescriptorService(phydbid, "clickhouse", labels, admin, zap.NewNop()).
		RegisterHandlers(app, testUser, testPass)
	return app
}

func doRequest(t *testing.T, app *fiber.App, withAuth bool) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, DescriptorPath, nil)
	if withAuth {
		req.SetBasicAuth(testUser, testPass)
	}
	resp, err := app.Test(req)
	assert.NoError(t, err)
	return resp
}

func TestDescriptorEndpointRequiresAuthentication(t *testing.T) {
	app := newTestApp("test-ns:clickhouse", nil, supportedAdminService())

	resp := doRequest(t, app, false)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestDescriptorEndpointRejectsWrongCredentials(t *testing.T) {
	app := newTestApp("test-ns:clickhouse", nil, supportedAdminService())

	req := httptest.NewRequest(http.MethodGet, DescriptorPath, nil)
	req.SetBasicAuth(testUser, "wrong-password")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestDescriptorEndpointReturnsDescriptor(t *testing.T) {
	labels := map[string]string{"clusterName": "clickhouse"}
	app := newTestApp("test-ns:clickhouse", labels, supportedAdminService())

	resp := doRequest(t, app, true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	var descriptor Descriptor
	assert.NoError(t, json.Unmarshal(body, &descriptor))

	assert.Equal(t, "test-ns:clickhouse", descriptor.PhysicalDatabaseId)
	assert.Equal(t, "clickhouse", descriptor.Type)
	assert.Equal(t, labels, descriptor.Labels)
	assert.Equal(t, []string{"admin", "rw", "ro"}, descriptor.SupportedRoles)
	assert.Equal(t, map[string]bool{"multiusers": true, "tls": false}, descriptor.Features)

	assert.Len(t, descriptor.ApiVersions.Specs, 1)
	spec := descriptor.ApiVersions.Specs[0]
	assert.Equal(t, dao.RootUrl, spec.SpecRootUrl)
	assert.Equal(t, dao.MajorAPIVersion, spec.Major)
	assert.Equal(t, dao.MinorAPIVersion, spec.Minor)
	assert.Equal(t, dao.SupportedMajorsVersions, spec.SupportedMajors)
}

// ClickHouse exposes no separate read-only endpoint, so readOnlyHost must be omitted
// rather than reported as an empty host.
func TestDescriptorOmitsEmptyReadOnlyHost(t *testing.T) {
	app := newTestApp("test-ns:clickhouse", nil, supportedAdminService())

	resp := doRequest(t, app, true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	var raw map[string]any
	assert.NoError(t, json.Unmarshal(body, &raw))
	assert.NotContains(t, raw, "readOnlyHost")
	assert.NotContains(t, raw, "roHost")
}

func TestDescriptorReportsReadOnlyHostWhenConfigured(t *testing.T) {
	admin := supportedAdminService()
	admin.roHost = "clickhouse-ro.test-ns"
	app := newTestApp("test-ns:clickhouse", nil, admin)

	resp := doRequest(t, app, true)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	var descriptor Descriptor
	assert.NoError(t, json.Unmarshal(body, &descriptor))
	assert.Equal(t, "clickhouse-ro.test-ns", descriptor.ReadOnlyHost)
}

func TestDescriptorUnavailableWithoutPhysicalDatabaseId(t *testing.T) {
	app := newTestApp("", nil, supportedAdminService())

	resp := doRequest(t, app, true)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestDescriptorUnavailableForUnsupportedApiVersion(t *testing.T) {
	admin := supportedAdminService()
	admin.version = "v1"
	app := newTestApp("test-ns:clickhouse", nil, admin)

	resp := doRequest(t, app, true)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestDescriptorUnavailableWithoutSupportedRoles(t *testing.T) {
	admin := supportedAdminService()
	admin.roles = nil
	app := newTestApp("test-ns:clickhouse", nil, admin)

	resp := doRequest(t, app, true)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}
