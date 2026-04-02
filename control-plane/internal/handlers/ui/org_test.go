package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/server/middleware"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// mockIAMServer creates a test IAM server that returns canned responses.
func mockIAMServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// GET /api/get-organizations
	mux.HandleFunc("/api/get-organizations", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]string{
			{"owner": "admin", "name": "test-org", "displayName": "Test Org"},
		})
	})

	// GET /api/get-organization
	mux.HandleFunc("/api/get-organization", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"data":   map[string]string{"owner": "admin", "name": "test-org", "displayName": "Test Org"},
		})
	})

	// POST /api/add-organization
	mux.HandleFunc("/api/add-organization", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "msg": "organization created"})
	})

	// POST /api/update-organization
	mux.HandleFunc("/api/update-organization", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// POST /api/delete-organization
	mux.HandleFunc("/api/delete-organization", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// GET /api/get-users
	mux.HandleFunc("/api/get-users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]string{
			{"name": "admin", "email": "admin@test.com"},
		})
	})

	// POST /api/add-user (for addUserToOrg)
	mux.HandleFunc("/api/add-user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	return httptest.NewServer(mux)
}

func newOrgHandler(iamURL string) *OrgHandler {
	h := NewOrgHandler()
	h.iamURL = iamURL
	return h
}

func setTestOrgContext(c *gin.Context, org string) {
	c.Set(middleware.ContextKeyOrg, org)
	c.Set(middleware.ContextKeyUser, &middleware.IAMUserInfo{
		Sub:          "test-org/test-user",
		Email:        "test@example.com",
		Organization: org,
	})
}

func TestOrgHandler_ListOrgs(t *testing.T) {
	iam := mockIAMServer(t)
	defer iam.Close()

	h := newOrgHandler(iam.URL)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/orgs", nil)
	setTestOrgContext(c, "test-org")

	h.ListOrgs(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "test-org", result[0]["name"])
}

func TestOrgHandler_GetOrg(t *testing.T) {
	iam := mockIAMServer(t)
	defer iam.Close()

	h := newOrgHandler(iam.URL)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/orgs/test-org", nil)
	c.Params = gin.Params{{Key: "orgId", Value: "test-org"}}
	setTestOrgContext(c, "test-org")

	h.GetOrg(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOrgHandler_CreateOrg(t *testing.T) {
	iam := mockIAMServer(t)
	defer iam.Close()

	h := newOrgHandler(iam.URL)

	body := `{"name":"new-org","displayName":"New Organization"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/orgs", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	setTestOrgContext(c, "test-org")

	h.CreateOrg(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOrgHandler_UpdateOrg(t *testing.T) {
	iam := mockIAMServer(t)
	defer iam.Close()

	h := newOrgHandler(iam.URL)

	body := `{"displayName":"Updated Org"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/v1/orgs/test-org", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "orgId", Value: "test-org"}}
	setTestOrgContext(c, "test-org")

	h.UpdateOrg(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOrgHandler_UpdateOrg_Forbidden(t *testing.T) {
	iam := mockIAMServer(t)
	defer iam.Close()

	h := newOrgHandler(iam.URL)

	body := `{"displayName":"Hacked"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/v1/orgs/other-org", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "orgId", Value: "other-org"}}
	setTestOrgContext(c, "test-org")

	h.UpdateOrg(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestOrgHandler_DeleteOrg(t *testing.T) {
	iam := mockIAMServer(t)
	defer iam.Close()

	h := newOrgHandler(iam.URL)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/v1/orgs/test-org", nil)
	c.Params = gin.Params{{Key: "orgId", Value: "test-org"}}
	setTestOrgContext(c, "test-org")

	h.DeleteOrg(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOrgHandler_GetOrgMembers(t *testing.T) {
	iam := mockIAMServer(t)
	defer iam.Close()

	h := newOrgHandler(iam.URL)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/orgs/test-org/members", nil)
	c.Params = gin.Params{{Key: "orgId", Value: "test-org"}}
	setTestOrgContext(c, "test-org")

	h.GetOrgMembers(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOrgHandler_ListOrgs_NoAuth(t *testing.T) {
	iam := mockIAMServer(t)
	defer iam.Close()

	h := newOrgHandler(iam.URL)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/orgs", nil)
	// No auth context set

	h.ListOrgs(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
