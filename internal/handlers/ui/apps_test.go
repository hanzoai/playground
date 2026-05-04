package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/internal/server/middleware"
	"github.com/stretchr/testify/assert"
)

// mockIAMServerApps creates a test IAM server for application CRUD.
func mockIAMServerApps(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/get-applications", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]string{
			{"owner": "test-org", "name": "my-app", "displayName": "My App", "clientId": "my-app"},
		})
	})

	mux.HandleFunc("/api/get-application", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"owner": "test-org", "name": "my-app", "displayName": "My App",
			"clientId": "my-app", "clientSecret": "secret-123",
			"redirectUris": []string{"http://localhost:3000/callback"},
		})
	})

	mux.HandleFunc("/api/add-application", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/update-application", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/delete-application", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	return httptest.NewServer(mux)
}

func newAppHandler(iamURL string) *AppHandler {
	h := NewAppHandler()
	h.iamURL = iamURL
	return h
}

func setTestAppContext(c *gin.Context, org string) {
	c.Set(middleware.ContextKeyOrg, org)
	c.Set(middleware.ContextKeyUser, &middleware.IAMUserInfo{
		Sub:          org + "/test-user",
		Email:        "test@example.com",
		Organization: org,
	})
}

func TestAppHandler_ListApps(t *testing.T) {
	iam := mockIAMServerApps(t)
	defer iam.Close()

	h := newAppHandler(iam.URL)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/orgs/test-org/apps", nil)
	c.Params = gin.Params{{Key: "orgId", Value: "test-org"}}
	setTestAppContext(c, "test-org")

	h.ListApps(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var result []map[string]string
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Len(t, result, 1)
	assert.Equal(t, "my-app", result[0]["name"])
}

func TestAppHandler_CreateApp(t *testing.T) {
	iam := mockIAMServerApps(t)
	defer iam.Close()

	h := newAppHandler(iam.URL)

	body := `{"name":"new-app","displayName":"New App","redirectUris":["http://localhost:3000/cb"],"enablePassword":true}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/orgs/test-org/apps", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "orgId", Value: "test-org"}}
	setTestAppContext(c, "test-org")

	h.CreateApp(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAppHandler_GetApp(t *testing.T) {
	iam := mockIAMServerApps(t)
	defer iam.Close()

	h := newAppHandler(iam.URL)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/orgs/test-org/apps/my-app", nil)
	c.Params = gin.Params{
		{Key: "orgId", Value: "test-org"},
		{Key: "appId", Value: "my-app"},
	}
	setTestAppContext(c, "test-org")

	h.GetApp(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAppHandler_UpdateApp(t *testing.T) {
	iam := mockIAMServerApps(t)
	defer iam.Close()

	h := newAppHandler(iam.URL)

	body := `{"displayName":"Updated App"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/v1/orgs/test-org/apps/my-app", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{
		{Key: "orgId", Value: "test-org"},
		{Key: "appId", Value: "my-app"},
	}
	setTestAppContext(c, "test-org")

	h.UpdateApp(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAppHandler_DeleteApp(t *testing.T) {
	iam := mockIAMServerApps(t)
	defer iam.Close()

	h := newAppHandler(iam.URL)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/v1/orgs/test-org/apps/my-app", nil)
	c.Params = gin.Params{
		{Key: "orgId", Value: "test-org"},
		{Key: "appId", Value: "my-app"},
	}
	setTestAppContext(c, "test-org")

	h.DeleteApp(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAppHandler_OrgAccessDenied(t *testing.T) {
	iam := mockIAMServerApps(t)
	defer iam.Close()

	h := newAppHandler(iam.URL)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/orgs/other-org/apps", nil)
	c.Params = gin.Params{{Key: "orgId", Value: "other-org"}}
	setTestAppContext(c, "test-org") // user belongs to test-org, not other-org

	h.ListApps(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAppHandler_NoAuth(t *testing.T) {
	iam := mockIAMServerApps(t)
	defer iam.Close()

	h := newAppHandler(iam.URL)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/orgs/test-org/apps", nil)
	c.Params = gin.Params{{Key: "orgId", Value: "test-org"}}
	// No auth context

	h.ListApps(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
