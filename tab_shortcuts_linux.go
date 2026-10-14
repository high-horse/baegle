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

func installTabShortcuts(window *application.WebviewWindow) {
	window.OnWindowEvent(events.Common.WindowRuntimeReady, func(_ *application.WindowEvent) {
		application.InvokeAsync(func() {
			if native := window.NativeWindow(); native != nil {
				C.install_tab_shortcuts((*C.GtkWidget)(native))
			}
		})
	})
}
