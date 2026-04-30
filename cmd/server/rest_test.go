package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRESTLoginAndProtectedEndpoint(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/items" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(WfmEnvelope[[]WfmItem]{
			ApiVersion: "v0.23.0",
			Data: []WfmItem{{
				Id:   "item-1",
				Slug: "nikana_prime_set",
				I18n: WfmI18n{En: WfmLangDetail{Name: "Nikana Prime Set"}},
			}},
		})
	}))
	defer upstream.Close()

	watcher := newWatcherServerFromConfigWithAuth(upstream.URL, "", "en", "aoi umi", "blue_sea_30", upstream.Client())
	watcher.minRequestGap = 0
	handler := newRESTServer(watcher)

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/items", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"aoi umi","password":"blue_sea_30"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRes := httptest.NewRecorder()
	handler.ServeHTTP(loginRes, loginReq)
	if loginRes.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginRes.Code, loginRes.Body.String())
	}

	var loginBody struct {
		Success      bool   `json:"success"`
		SessionToken string `json:"session_token"`
	}
	if err := json.Unmarshal(loginRes.Body.Bytes(), &loginBody); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if !loginBody.Success || loginBody.SessionToken == "" {
		t.Fatalf("unexpected login response: %+v", loginBody)
	}

	itemsReq := httptest.NewRequest(http.MethodGet, "/api/items?language=en", nil)
	itemsReq.Header.Set("Authorization", "Bearer "+loginBody.SessionToken)
	itemsRes := httptest.NewRecorder()
	handler.ServeHTTP(itemsRes, itemsReq)
	if itemsRes.Code != http.StatusOK {
		t.Fatalf("items status = %d, body = %s", itemsRes.Code, itemsRes.Body.String())
	}
	if !strings.Contains(itemsRes.Body.String(), "nikana_prime_set") {
		t.Fatalf("items response missing expected slug: %s", itemsRes.Body.String())
	}
}

func TestRESTLogoutInvalidatesToken(t *testing.T) {
	watcher := newWatcherServerFromConfigWithAuth("http://example.test", "", "en", "aoi umi", "blue_sea_30", nil)
	watcher.minRequestGap = 0
	handler := newRESTServer(watcher)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"aoi umi","password":"blue_sea_30"}`))
	loginRes := httptest.NewRecorder()
	handler.ServeHTTP(loginRes, loginReq)

	var loginBody struct {
		SessionToken string `json:"session_token"`
	}
	if err := json.Unmarshal(loginRes.Body.Bytes(), &loginBody); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+loginBody.SessionToken)
	logoutRes := httptest.NewRecorder()
	handler.ServeHTTP(logoutRes, logoutReq)
	if logoutRes.Code != http.StatusOK {
		t.Fatalf("logout status = %d, body = %s", logoutRes.Code, logoutRes.Body.String())
	}

	itemsReq := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	itemsReq.Header.Set("Authorization", "Bearer "+loginBody.SessionToken)
	itemsRes := httptest.NewRecorder()
	handler.ServeHTTP(itemsRes, itemsReq)
	if itemsRes.Code != http.StatusUnauthorized {
		t.Fatalf("after logout status = %d, want %d", itemsRes.Code, http.StatusUnauthorized)
	}
}

func TestRESTCORSPreflight(t *testing.T) {
	handler := newRESTServer(newWatcherServerFromConfigWithAuth("http://example.test", "", "en", "aoi umi", "blue_sea_30", nil))
	req := httptest.NewRequest(http.MethodOptions, "/api/items", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if res.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Fatalf("missing CORS allow headers")
	}
}
