//go:build !linux || !cgo || android || server

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func installWindowShortcuts(_ *application.WebviewWindow, _ func()) bool { return false }
