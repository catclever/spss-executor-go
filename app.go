package main

import (
	"context"
	"fmt"
	"runtime"

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
func (a *App) ConnectServer(url string, prompt string, spssPath string, dataPath string, usePD bool, vmName string) error {
	return a.client.Connect(url, prompt, spssPath, dataPath, usePD, vmName)
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
