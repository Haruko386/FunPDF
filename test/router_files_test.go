package internal_test

import (
	"FunPDF/internal"
	"FunPDF/internal/handler"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRouterRegistersFileRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	internal.NewRouter(
		handler.NewFileHandler(),
		handler.NewAlbumHandler(),
		handler.NewTranslatorHandler(),
		handler.NewProviderHandler(),
		handler.NewModelHandler(),
		handler.NewChatSessionHandler(),
	).Setup(engine)

	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	expected := []string{
		"GET /api/files",
		"POST /api/files",
		"POST /api/files/import-path",
		"PUT /api/files/:file_id",
		"DELETE /api/files/:file_id",
		"GET /api/files/:file_id/content",
		"GET /api/files/:file_id/state",
		"GET /api/files/:file_id/thumbnail",
		"PATCH /api/files/:file_id/state",
		"GET /api/files/:file_id/album",
		"DELETE /api/files/:file_id/cache",
	}
	for _, route := range expected {
		if !routes[route] {
			t.Fatalf("missing route %s", route)
		}
	}
	if len(routes) < len(expected) {
		t.Fatalf("registered route set is unexpectedly smaller than expected")
	}
	for route := range routes {
		if strings.HasPrefix(route, "GET /api/files") ||
			strings.HasPrefix(route, "POST /api/files") ||
			strings.HasPrefix(route, "PUT /api/files") ||
			strings.HasPrefix(route, "DELETE /api/files") ||
			strings.HasPrefix(route, "PATCH /api/files") {
			found := false
			for _, expectedRoute := range expected {
				if route == expectedRoute {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("unexpected file route %s", route)
			}
		}
	}
}
