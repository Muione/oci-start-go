//go:build dist

package web

import "embed"

// In the production build (-tags dist) the Vite build output (frontend/dist,
// built to internal/web/dist) is embedded.
//
//go:embed all:dist
var Assets embed.FS
