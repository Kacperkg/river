package handlers

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"river-api/internal/middleware"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requestRouter(repo *fakeSettingRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewRequestHandler(services.NewSettingsService(repo))
	r := gin.New()
	r.POST("/admin/settings/integrations/test", h.TestConnection)
	r.GET("/request/calendar", h.Calendar)
	return r
}

func TestRequestHandler_TestConnection_NotConfigured(t *testing.T) {
	r := requestRouter(&fakeSettingRepo{m: map[string]string{}})
	w := doJSON(r, http.MethodPost, "/admin/settings/integrations/test", `{"service":"radarr"}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"ok":false`)
	assert.Contains(t, w.Body.String(), "not configured")
}

func TestRequestHandler_TestConnection_InvalidService(t *testing.T) {
	r := requestRouter(&fakeSettingRepo{m: map[string]string{}})
	w := doJSON(r, http.MethodPost, "/admin/settings/integrations/test", `{"service":"plex"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRequestHandler_TestConnection_Success(t *testing.T) {
	// Fake Radarr that answers the system/status probe.
	arr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "/api/v3/system/status", req.URL.Path)
		assert.Equal(t, "k1", req.Header.Get("X-Api-Key"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"5.2.0"}`))
	}))
	defer arr.Close()

	repo := &fakeSettingRepo{m: map[string]string{"radarr.url": arr.URL, "radarr.api_key": "k1"}}
	r := requestRouter(repo)

	w := doJSON(r, http.MethodPost, "/admin/settings/integrations/test", `{"service":"radarr"}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"ok":true`)
	assert.Contains(t, w.Body.String(), "5.2.0")
}

func TestRequestHandler_TestConnection_UpstreamError(t *testing.T) {
	arr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer arr.Close()

	repo := &fakeSettingRepo{m: map[string]string{"sonarr.url": arr.URL, "sonarr.api_key": "bad"}}
	r := requestRouter(repo)

	w := doJSON(r, http.MethodPost, "/admin/settings/integrations/test", `{"service":"sonarr"}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"ok":false`)
}

func TestRequestHandler_Calendar_Unconfigured503(t *testing.T) {
	r := requestRouter(&fakeSettingRepo{m: map[string]string{}})
	w := doJSON(r, http.MethodGet, "/request/calendar?start=2026-01-01&end=2026-02-01", "")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// fakeArr is a stand-in Radarr/Sonarr answering the endpoints the request
// handler calls. All responses are application/json (arrGet requires it).
func fakeArr() *httptest.Server {
	return fakeArrWithPostStatus(http.StatusCreated)
}

func fakeArrWithPostStatus(postStatus int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v3/movie/lookup":
			_, _ = w.Write([]byte(`[{"tmdbId":603,"title":"The Matrix","year":1999,"overview":"o","remotePoster":"http://p"}]`))
		case r.URL.Path == "/api/v3/series/lookup":
			_, _ = w.Write([]byte(`[{"tvdbId":78,"title":"Dragnet","year":1951,"overview":"o","remotePoster":"http://p"}]`))
		case r.URL.Path == "/api/v3/rootfolder":
			_, _ = w.Write([]byte(`[{"path":"/data"}]`))
		case r.URL.Path == "/api/v3/qualityprofile":
			_, _ = w.Write([]byte(`[{"id":1}]`))
		case r.URL.Path == "/api/v3/languageprofile":
			_, _ = w.Write([]byte(`[{"id":1}]`))
		case r.URL.Path == "/api/v3/calendar" && r.URL.Query().Get("includeSeries") == "true":
			_, _ = w.Write([]byte(`[{"title":"Ep","overview":"o","airDateUtc":"2026-01-15T00:00:00Z","seasonNumber":1,"episodeNumber":2,"series":{"title":"Dragnet"}}]`))
		case r.URL.Path == "/api/v3/calendar":
			_, _ = w.Write([]byte(`[{"title":"The Matrix","overview":"o","digitalRelease":"2026-01-10T00:00:00Z"}]`))
		case r.URL.Path == "/api/v3/movie" && r.Method == http.MethodPost:
			w.WriteHeader(postStatus)
		case r.URL.Path == "/api/v3/series" && r.Method == http.MethodPost:
			w.WriteHeader(postStatus)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func fullRequestRouter(arrURL string) *gin.Engine {
	return fullRequestRouterWithUsername(arrURL, "alice")
}

func fullRequestRouterWithUsername(arrURL, username string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	repo := &fakeSettingRepo{m: map[string]string{
		"radarr.url": arrURL, "radarr.api_key": "k",
		"sonarr.url": arrURL, "sonarr.api_key": "k",
	}}
	h := NewRequestHandler(services.NewSettingsService(repo))
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("claims", &middleware.Claims{Username: username, UserID: "user-123"})
		c.Next()
	})
	r.GET("/request/movies", h.SearchMovies)
	r.POST("/request/movies", h.AddMovie)
	r.GET("/request/shows", h.SearchShows)
	r.POST("/request/shows", h.AddShow)
	r.GET("/request/calendar", h.Calendar)
	return r
}

func TestRequestHandler_SearchMovies(t *testing.T) {
	arr := fakeArr()
	defer arr.Close()
	r := fullRequestRouter(arr.URL)
	w := doJSON(r, http.MethodGet, "/request/movies?q=matrix", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "The Matrix")
}

func TestRequestHandler_SearchMovies_MissingQuery(t *testing.T) {
	arr := fakeArr()
	defer arr.Close()
	r := fullRequestRouter(arr.URL)
	w := doJSON(r, http.MethodGet, "/request/movies", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRequestHandler_SearchShows(t *testing.T) {
	arr := fakeArr()
	defer arr.Close()
	r := fullRequestRouter(arr.URL)
	w := doJSON(r, http.MethodGet, "/request/shows?q=drag", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Dragnet")
}

func TestRequestHandler_AddMovie(t *testing.T) {
	arr := fakeArr()
	defer arr.Close()
	r := fullRequestRouter(arr.URL)
	logs := captureRequestLogs(t)
	w := doJSON(r, http.MethodPost, "/request/movies", `{"tmdbId":603,"title":"The Matrix","year":1999}`)
	assert.Equal(t, http.StatusNoContent, w.Code)
	const auditEntry = `INFO request: user alice (user-123) requested movie "The Matrix" (1999) tmdbId=603`
	assert.Equal(t, 1, strings.Count(logs.String(), auditEntry))
}

func TestRequestHandler_AddMovie_UpstreamErrorDoesNotLog(t *testing.T) {
	arr := fakeArrWithPostStatus(http.StatusInternalServerError)
	defer arr.Close()
	r := fullRequestRouter(arr.URL)
	logs := captureRequestLogs(t)
	w := doJSON(r, http.MethodPost, "/request/movies", `{"tmdbId":603,"title":"The Matrix","year":1999}`)
	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.NotContains(t, logs.String(), "INFO request:")
}

func TestRequestHandler_AddMovie_InvalidInputDoesNotLog(t *testing.T) {
	arr := fakeArr()
	defer arr.Close()
	r := fullRequestRouter(arr.URL)
	logs := captureRequestLogs(t)
	w := doJSON(r, http.MethodPost, "/request/movies", `{"title":"The Matrix","year":1999}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NotContains(t, logs.String(), "INFO request:")
}

func TestRequestHandler_AddMovie_EscapesControlCharactersInUsername(t *testing.T) {
	arr := fakeArr()
	defer arr.Close()
	r := fullRequestRouterWithUsername(arr.URL, "alice\nINFO request: forged")
	logs := captureRequestLogs(t)
	w := doJSON(r, http.MethodPost, "/request/movies", `{"tmdbId":603,"title":"The Matrix","year":1999}`)
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Contains(t, logs.String(), `user alice\nINFO request: forged (user-123)`)
	assert.NotContains(t, logs.String(), "\nINFO request: forged")
}

func TestRequestHandler_AddShow(t *testing.T) {
	arr := fakeArr()
	defer arr.Close()
	r := fullRequestRouter(arr.URL)
	logs := captureRequestLogs(t)
	w := doJSON(r, http.MethodPost, "/request/shows", `{"tvdbId":78,"title":"Dragnet","year":1951}`)
	assert.Equal(t, http.StatusNoContent, w.Code)
	const auditEntry = `INFO request: user alice (user-123) requested show "Dragnet" (1951) tvdbId=78`
	assert.Equal(t, 1, strings.Count(logs.String(), auditEntry))
}

func TestRequestHandler_AddShow_UpstreamErrorDoesNotLog(t *testing.T) {
	arr := fakeArrWithPostStatus(http.StatusInternalServerError)
	defer arr.Close()
	r := fullRequestRouter(arr.URL)
	logs := captureRequestLogs(t)
	w := doJSON(r, http.MethodPost, "/request/shows", `{"tvdbId":78,"title":"Dragnet","year":1951}`)
	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.NotContains(t, logs.String(), "INFO request:")
}

func TestRequestHandler_AddShow_InvalidInputDoesNotLog(t *testing.T) {
	arr := fakeArr()
	defer arr.Close()
	r := fullRequestRouter(arr.URL)
	logs := captureRequestLogs(t)
	w := doJSON(r, http.MethodPost, "/request/shows", `{"title":"Dragnet","year":1951}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NotContains(t, logs.String(), "INFO request:")
}

func captureRequestLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })
	return &logs
}

func TestRequestHandler_Calendar_CombinesSources(t *testing.T) {
	arr := fakeArr()
	defer arr.Close()
	r := fullRequestRouter(arr.URL)
	w := doJSON(r, http.MethodGet, "/request/calendar?start=2026-01-01&end=2026-02-01", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "The Matrix") // radarr movie
	assert.Contains(t, w.Body.String(), "Dragnet")    // sonarr episode's series
}

func TestRequestHandler_Availability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := func(repo *fakeSettingRepo) *gin.Engine {
		h := NewRequestHandler(services.NewSettingsService(repo))
		r := gin.New()
		r.GET("/request/availability", h.Availability)
		return r
	}

	t.Run("neither configured -> disabled", func(t *testing.T) {
		w := doJSON(router(&fakeSettingRepo{m: map[string]string{}}), http.MethodGet, "/request/availability", "")
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"enabled":false`)
		assert.Contains(t, w.Body.String(), `"radarr":false`)
	})

	t.Run("radarr configured -> enabled", func(t *testing.T) {
		repo := &fakeSettingRepo{m: map[string]string{"radarr.url": "http://radarr"}}
		w := doJSON(router(repo), http.MethodGet, "/request/availability", "")
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"enabled":true`)
		assert.Contains(t, w.Body.String(), `"radarr":true`)
		assert.Contains(t, w.Body.String(), `"sonarr":false`)
	})
}
