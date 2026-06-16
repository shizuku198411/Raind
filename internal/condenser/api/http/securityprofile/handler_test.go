package securityprofile

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	core "raind/internal/condenser/core/securityprofile"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListSecurityProfiles(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/security/profiles", nil)
	w := httptest.NewRecorder()

	NewRequestHandler().ListSecurityProfiles(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Status string                      `json:"status"`
		Data   ListSecurityProfileResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp.Data.Profiles, 6)
	assert.Equal(t, core.ProfileDefault, resp.Data.Profiles[0].Name)
	assert.Equal(t, core.ProfileDev, resp.Data.Profiles[1].Name)
	assert.Equal(t, core.ProfileDeploy, resp.Data.Profiles[2].Name)
	assert.Equal(t, core.ProfileRestricted, resp.Data.Profiles[3].Name)
	assert.Equal(t, core.ProfilePrivileged, resp.Data.Profiles[4].Name)
	assert.Equal(t, core.ProfileUnconfined, resp.Data.Profiles[5].Name)
}

func TestShowSecurityProfile(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/v1/security/profiles/{name}", NewRequestHandler().ShowSecurityProfile)
	req := httptest.NewRequest(http.MethodGet, "/v1/security/profiles/dev", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Status string                      `json:"status"`
		Data   ShowSecurityProfileResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, core.ProfileDev, resp.Data.Profile.Name)
	assert.Equal(t, "raind-default", resp.Data.Profile.AppArmorProfile)
}

func TestShowDeploySecurityProfile(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/v1/security/profiles/{name}", NewRequestHandler().ShowSecurityProfile)
	req := httptest.NewRequest(http.MethodGet, "/v1/security/profiles/deploy", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Status string                      `json:"status"`
		Data   ShowSecurityProfileResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, core.ProfileDeploy, resp.Data.Profile.Name)
	assert.NotContains(t, resp.Data.Profile.Capabilities.Base, "CAP_NET_RAW")
	assert.NotContains(t, resp.Data.Profile.Capabilities.Base, "CAP_MKNOD")
}

func TestShowRestrictedSecurityProfile(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/v1/security/profiles/{name}", NewRequestHandler().ShowSecurityProfile)
	req := httptest.NewRequest(http.MethodGet, "/v1/security/profiles/restricted", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Status string                      `json:"status"`
		Data   ShowSecurityProfileResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, core.ProfileRestricted, resp.Data.Profile.Name)
	assert.Empty(t, resp.Data.Profile.Capabilities.Base)
}

func TestShowPrivilegedSecurityProfile(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/v1/security/profiles/{name}", NewRequestHandler().ShowSecurityProfile)
	req := httptest.NewRequest(http.MethodGet, "/v1/security/profiles/privileged", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Status string                      `json:"status"`
		Data   ShowSecurityProfileResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, core.ProfilePrivileged, resp.Data.Profile.Name)
	assert.Contains(t, resp.Data.Profile.Capabilities.Base, "CAP_SYS_ADMIN")
	assert.Nil(t, resp.Data.Profile.Seccomp)
}

func TestShowUnconfinedSecurityProfile(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/v1/security/profiles/{name}", NewRequestHandler().ShowSecurityProfile)
	req := httptest.NewRequest(http.MethodGet, "/v1/security/profiles/unconfined", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Status string                      `json:"status"`
		Data   ShowSecurityProfileResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, core.ProfileUnconfined, resp.Data.Profile.Name)
	assert.Equal(t, core.DefaultSecurityProfile().Capabilities.Base, resp.Data.Profile.Capabilities.Base)
	assert.Nil(t, resp.Data.Profile.Seccomp)
}

func TestShowSecurityProfileNotFound(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/v1/security/profiles/{name}", NewRequestHandler().ShowSecurityProfile)
	req := httptest.NewRequest(http.MethodGet, "/v1/security/profiles/unknown-profile", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
