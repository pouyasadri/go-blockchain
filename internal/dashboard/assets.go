package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets/*
var embeddedAssets embed.FS

// AssetsHandler returns an http.Handler that serves embedded images and static assets.
// This guarantees assets load reliably in Docker containers and standalone single binaries.
func AssetsHandler() http.Handler {
	sub, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(sub))
}
