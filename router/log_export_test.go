package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestUsageLogExportRoutesRejectUnauthenticatedDownloads(t *testing.T) {
	g := gin.New()
	SetApiRouter(g)
	for _, path := range []string{"/api/log/export", "/api/log/self/export"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			g.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Empty(t, w.Header().Get("Content-Disposition"))
		})
	}
}
