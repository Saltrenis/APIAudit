//go:build !dev

package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:frontend/dist
var embeddedFS embed.FS

func staticHandler() http.Handler {
	sub, err := fs.Sub(embeddedFS, "frontend/dist")
	if err != nil {
		panic("embed: frontend/dist not found: " + err.Error())
	}
	return http.FileServer(http.FS(sub))
}
