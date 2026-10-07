package web

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
)

//go:embed assets/*
var embeddedAssets embed.FS

func AssetHandler() http.Handler {
	assets, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		panic(err)
	}

	fileServer := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.HasSuffix(r.URL.Path, ".js") {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		}
		// Embedded assets cannot change while this process is running. A short
		// shared cache avoids downloading the same CSS and JavaScript on every
		// navigation while still letting a production deploy age out quickly.
		w.Header().Set("Cache-Control", "public, max-age=3600, stale-while-revalidate=86400")
		http.StripPrefix("/assets/", fileServer).ServeHTTP(w, r)
	})
}

var assetDigests sync.Map

func assetURL(name string) string {
	if cached, ok := assetDigests.Load(name); ok {
		return cached.(string)
	}
	data, err := embeddedAssets.ReadFile("assets/" + name)
	if err != nil {
		return "/assets/" + name
	}
	sum := sha256.Sum256(data)
	url := fmt.Sprintf("/assets/%s?v=%x", name, sum[:8])
	assetDigests.Store(name, url)
	return url
}
