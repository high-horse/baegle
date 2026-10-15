//go:build linux && cgo && !android && !server

package main

/*
#cgo !gtk3 pkg-config: gtk4 webkitgtk-6.0
#cgo gtk3 pkg-config: gtk+-3.0 webkit2gtk-4.1
#include "tab_shortcuts_linux.h"
*/
import "C"

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// The native capture handler consumes Ctrl+Shift+N before Wails and WebKit
// can both dispatch it. Keep the callback on Go's side for normal error dialogs.
var newConnectionWindowShortcut func()

//export dispatchNewConnectionWindowShortcut
func dispatchNewConnectionWindowShortcut() {
	if newConnectionWindowShortcut != nil {
		newConnectionWindowShortcut()
	}
}

func installWindowShortcuts(window *application.WebviewWindow, openWindow func()) bool {
	newConnectionWindowShortcut = openWindow
	window.OnWindowEvent(events.Common.WindowRuntimeReady, func(_ *application.WindowEvent) {
		application.InvokeAsync(func() {
			if native := window.NativeWindow(); native != nil {
				C.install_tab_shortcuts((*C.GtkWidget)(native))
			}
		})
	})
	return true
}
