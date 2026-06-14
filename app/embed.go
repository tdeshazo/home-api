package app

import "embed"

// Files contains the static frontend application.
//
//go:embed *.html assets
var Files embed.FS
