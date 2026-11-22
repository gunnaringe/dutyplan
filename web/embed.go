// Package web holds the frontend files that are compiled into the binary.
package web

import "embed"

//go:embed index.html i18n.js app.js style.css landing.png demo-en.mp4 demo-nb.mp4 logo.svg llms.txt
var Files embed.FS
