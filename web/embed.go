// Package web exposes the built single-page app so the server binary can serve it.
package web

import "embed"

// Dist holds the Vite build output. Run `npm run build` in this directory first;
// with only the placeholder present, the server falls back to an API-only mode.
//
//go:embed all:dist
var Dist embed.FS
