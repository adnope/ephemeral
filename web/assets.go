package web

import "embed"

//go:embed dist/* static/*
var FS embed.FS
