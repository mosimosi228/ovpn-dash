package web

import "embed"

// Dist is the Vite-built ovpn-dash dashboard (Vue).
//
//go:embed all:dist
var Dist embed.FS
