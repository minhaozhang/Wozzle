package main

import "embed"

//go:embed all:web/dist
var distFS embed.FS
