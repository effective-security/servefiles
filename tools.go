// Package tools for go mod

//go:build tools
// +build tools

package tools

import (
	_ "github.com/go-phorce/cov-report/cmd/cov-report"
	_ "github.com/mattn/goveralls"
	_ "golang.org/x/lint/golint"
)
