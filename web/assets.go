package web

import "embed"

//go:embed template/*.html template/partials/*.html static/*
var FS embed.FS
