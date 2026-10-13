package main

import (
	"db-lens/internal/app"
	"db-lens/internal/db"
	"embed"
	"fmt"

	"log"
	"os"
	"os/exec"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	if runtime.GOOS == "linux" {
		if err := os.Setenv("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS", "1"); err != nil {
			log.Fatalf("failed to configure WebKit sandbox: %v", err)
		}
	}
	if _, err := db.InitDb(); err != nil {
		log.Fatalf("failed to initialise local database: %v", err)
	}

	// GTK4 registers Options.Name on D-Bus even without SingleInstance options.
	// Distinct identities let independent connection processes activate windows.
	applicationName := "baegle"
	if runtime.GOOS == "linux" {
		applicationName = fmt.Sprintf("baegle %d", os.Getpid())
	}

	dbService := app.NewDbService(db.NewHistoryRepository(db.Conn))

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	app := application.New(application.Options{
		Name:        applicationName,
		Description: "A database viewer",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(dbService),
			application.NewService(app.NewDatabaseService()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'Mac' options tailor the window when running on macOS.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "baegle",
		// Window sized to the golden ratio (1000 / 618 ≈ 1.618).
		Width:  1000,
		Height: 618,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	// Each process owns its active database connection and result cursors.
	// A second webview in this process would otherwise share that state.
	app.KeyBinding.Add("Ctrl+Shift+N", func(window application.Window) {
		if err := openConnectionWindow(); err != nil {
			log.Printf("failed to open connection window: %v", err)
			app.Dialog.Error().SetTitle("New connection window").
				SetMessage("Could not open a new window: " + err.Error()).Show()
		}
	})

	// Run the application. This blocks until the application has been exited.
	err := app.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}

// openConnectionWindow starts the app at its connection screen with an
// independent backend. Do not forward command-line flags from the parent.
func openConnectionWindow() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	// Relaunch the original AppImage rather than its temporary mounted binary.
	if runtime.GOOS == "linux" && os.Getenv("APPIMAGE") != "" {
		executable = os.Getenv("APPIMAGE")
	}
	cmd := exec.Command(executable)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("connection window exited: %v", err)
		}
	}()
	return nil
}
