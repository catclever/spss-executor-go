package main

import (
	"context"
	"fmt"
	"runtime"

	wails_runtime "github.com/wailsapp/wails/v2/pkg/runtime"
	
	"spss-executor-go/backend"
)

// App struct
type App struct {
	ctx    context.Context
	client *backend.AgentClient
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.client = backend.NewAgentClient(ctx)
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// ConnectServer initiates the WebSocket connection to the Ruby Agent Server
func (a *App) ConnectServer(url string, prompt string, spssPath string, dataPath string, usePD bool, vmName string, workingNote string, llmConfigStr string) error {
	return a.client.Connect(url, prompt, spssPath, dataPath, usePD, vmName, workingNote, llmConfigStr)
}

// CancelExecution requests the backend to forcefully terminate the task and agent websocket
func (a *App) CancelExecution() {
	if a.client != nil {
		a.client.CancelExecution()
	}
}

// GetDevEnvironment returns current OS and architecture data for frontend UI logic
func (a *App) GetDevEnvironment() map[string]string {
	return map[string]string{
		"os":   runtime.GOOS,
		"arch": runtime.GOARCH,
	}
}

// SelectSPSSBinary opens a native file dialog and returns the path to the selected executable
func (a *App) SelectSPSSBinary() (string, error) {
	selection, err := wails_runtime.OpenFileDialog(a.ctx, wails_runtime.OpenDialogOptions{
		Title: "Select SPSS Executable",
	})
	if err != nil {
		return "", err
	}
	return selection, nil
}

// SelectDataFile opens a native file dialog for .sav datasets
func (a *App) SelectDataFile() (string, error) {
	selection, err := wails_runtime.OpenFileDialog(a.ctx, wails_runtime.OpenDialogOptions{
		Title: "Select Dataset (*.sav)",
		Filters: []wails_runtime.FileFilter{
			{
				DisplayName: "SPSS Dataset (*.sav)",
				Pattern:     "*.sav",
			},
		},
	})
	if err != nil {
		return "", err
	}
	return selection, nil
}

// FetchDictionary runs a preliminary DISPLAY DICTIONARY. command to extract dataset metadata 
// bypassing the full agent reasoning loop, storing it as a session working_note.
func (a *App) FetchDictionary(spssPath string, dataPath string, usePD bool, vmName string) (string, error) {
	output, err := backend.RunSPSS(a.ctx, spssPath, dataPath, "DISPLAY DICTIONARY.", usePD, vmName)
	if err != nil {
		return "", err
	}
	return output, nil
}
