// Package ui sirve la interfaz local de la rockola.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/*
var assets embed.FS

func Handler() http.Handler {
	web, err := fs.Sub(assets, "web")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(web))
}
