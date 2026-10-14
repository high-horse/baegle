//go:build !linux || !cgo || android || server

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func installTabShortcuts(_ *application.WebviewWindow) {}
