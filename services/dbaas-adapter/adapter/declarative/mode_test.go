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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	fiber2 "github.com/Netcracker/qubership-dbaas-adapter-core/pkg/impl/fiber"
	"github.com/Netcracker/qubership-dbaas-adapter-core/pkg/service"
)

// buildDeclarativeApp wires the adapter the way main.go does in declarative mode: the
// core handlers get no physical registration service, and the descriptor endpoint is
// registered alongside them.
func buildDeclarativeApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New()
	admin := supportedAdminService()

	var physicalRegistration *service.PhysicalDatabaseRegistrationService
	fiber2.BuildFiberDBaaSAdapterHandlers(
		app,
		testUser,
		testPass,
		"/clickhouse",
		admin,
		physicalRegistration,
		nil,
		map[string]bool{},
		zap.NewNop(),
		false,
		"declarative-mode-test",
	)

	NewDescriptorService("test-ns:clickhouse", "clickhouse", nil, admin, zap.NewNop()).
		RegisterHandlers(app, testUser, testPass)
	return app
}

func get(t *testing.T, app *fiber.App, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.SetBasicAuth(testUser, testPass)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	return resp
}

// Without a registration service the core must not expose handlers that dereference it,
// and the adapter must still start and serve traffic.
func TestDeclarativeModeServesWithoutRegistrationService(t *testing.T) {
	app := buildDeclarativeApp(t)

	resp := get(t, app, "/health")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "adapter must stay healthy without self-registration")

	resp = get(t, app, DescriptorPath)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "descriptor must be served in declarative mode")
}

func TestDeclarativeModeHidesLegacyRegistrationRoutes(t *testing.T) {
	app := buildDeclarativeApp(t)

	for _, path := range []string{
		"/api/v2/dbaas/adapter/clickhouse/physical_database",
		"/api/v2/dbaas/adapter/physical_database/force_registration",
	} {
		resp := get(t, app, path)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "legacy registration route %s must not be exposed", path)
	}
}

// Normal database administration routes must keep working; they are unrelated to
// registration and are only reachable with the adapter credentials.
func TestDeclarativeModeKeepsAdapterRoutes(t *testing.T) {
	app := buildDeclarativeApp(t)

	resp := get(t, app, "/api/v2/dbaas/adapter/clickhouse/databases")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/dbaas/adapter/clickhouse/databases", nil)
	unauthorized, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, unauthorized.StatusCode)
}
