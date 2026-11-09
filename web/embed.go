// Package web holds the frontend files that are compiled into the binary.
package web

import "embed"

//go:embed index.html i18n.js app.js style.css
var Files embed.FS
