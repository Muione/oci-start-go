//go:build !dist

package web

import "embed"

// In the default (no -tags dist) build the frontend dist/ is absent; embed a
// placeholder so `go build` succeeds without a built frontend.
//
//go:embed stub.html
var Assets embed.FS
