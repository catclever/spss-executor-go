package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/gorilla/websocket"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type AgentClient struct {
	ctx        context.Context
	runCtx     context.Context
	cancelFunc context.CancelFunc
	conn       *websocket.Conn
	spssPath   string
	dataPath   string
	usePD      bool
	vmName     string
}

func NewAgentClient(ctx context.Context) *AgentClient {
	return &AgentClient{
		ctx: ctx,
	}
}

// Connect opens the WebSocket to the Ruby server and sends the initialization payload.
func (c *AgentClient) Connect(url string, prompt string, spssPath string, dataPath string, usePD bool, vmName string, workingNote string, llmConfigStr string) error {
	log.Printf("Connecting to Agent Server: %s", url)

	c.spssPath = spssPath
	c.dataPath = dataPath
	c.usePD = usePD
	c.vmName = vmName

	// PRE-FLIGHT VALIDATION 1: Check Data Path (Must exist on the Host OS)
	if _, err := os.Stat(c.dataPath); os.IsNotExist(err) {
		return fmt.Errorf("dataset file not found on Host OS at path: %s", c.dataPath)
	}

	// PRE-FLIGHT VALIDATION 2: Check SPSS Executable Path
	if c.usePD {
		// Parallels VM Check: Run a simple cmd.exe check inside the VM
		log.Printf("Validating SPSS path inside Parallels VM: %s", c.spssPath)
		checkCmdStr := fmt.Sprintf("if exist \"%s\" (exit 0) else (exit 1)", c.spssPath)
		cmd := exec.Command("prlctl", "exec", c.vmName, "cmd.exe", "/c", checkCmdStr)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("SPSS executable not found inside Parallels VM ('%s') at path: %s. \nEnsure the VM is running and the path is correct.", c.vmName, c.spssPath)
		}
	} else {
		// Native Local Check (Windows or Mac natively)
		if _, err := os.Stat(c.spssPath); os.IsNotExist(err) {
			return fmt.Errorf("SPSS executable not found on Host OS at path: %s", c.spssPath)
		}
	}

	// Clean up previous context if any
	if c.cancelFunc != nil {
		c.cancelFunc()
	}

	// Create a new cancellable context for this execution run
	var cancel context.CancelFunc
	c.runCtx, cancel = context.WithCancel(c.ctx)
	c.cancelFunc = cancel

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		runtime.EventsEmit(c.ctx, "agent:error", err.Error())
		return err
	}
	c.conn = conn

	initMsg := map[string]interface{}{
		"type":         "init",
		"prompt":       prompt,
		"schema":       `{"vars": ["gender", "age"]}`, // Example schema
		"working_note": workingNote,
	}

	if llmConfigStr != "" && llmConfigStr != "{}" {
		var llmMap map[string]interface{}
		if err := json.Unmarshal([]byte(llmConfigStr), &llmMap); err == nil {
			initMsg["llm_config"] = llmMap
		}
	}
	if err := c.conn.WriteJSON(initMsg); err != nil {
		return err
	}

	runtime.EventsEmit(c.ctx, "agent:status", "Connected to Server. Waiting for agent...")

	// Wait and listen for messages in a goroutine
	go c.listen()
	return nil
}

func (c *AgentClient) listen() {
	defer c.conn.Close()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			runtime.EventsEmit(c.ctx, "agent:error", "Connection lost")
			break
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(message, &payload); err != nil {
			log.Println("Invalid JSON payload:", string(message))
			continue
		}

		msgType, ok := payload["type"].(string)
		if !ok {
			continue
		}

		// Emit the exact message to the frontend React/Svelte side
		runtime.EventsEmit(c.ctx, "agent:message", payload)

		// Handle execute_syntax by invoking real SPSS runner
		if msgType == "execute_syntax" {
			syntax, _ := payload["syntax"].(string)
			log.Printf("Executing SPSS syntax:\n%s\n", syntax)

			runtime.EventsEmit(c.ctx, "agent:status", "Waiting SPSS...")

			// Invoke the real runner with configured paths and PD settings
			outputStr, err := RunSPSS(c.runCtx, c.spssPath, c.dataPath, syntax, c.usePD, c.vmName)
			
			runtime.EventsEmit(c.ctx, "agent:status", "Waiting Agent...")

			status := "success"
			if err != nil {
				status = "error"
				outputStr = err.Error() + "\n" + outputStr
			}

			executionResult := map[string]interface{}{
				"type":   "execution_result",
				"status": status,
				"output": outputStr,
			}
			
			// New Feature: Stream the actual SPSS execution output to the Wails UI
			frontendOutputMsg := map[string]interface{}{
				"type":    "spss_output",
				"message": outputStr,
			}
			runtime.EventsEmit(c.ctx, "agent:message", frontendOutputMsg)

			c.conn.WriteJSON(executionResult)
			
		} else if msgType == "finished" {
			log.Printf("Agent finished. Closing connection.")
			break
		}
	}
}

// CancelExecution forcefuly terminates the current execution context and websocket
func (c *AgentClient) CancelExecution() {
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
	if c.conn != nil {
		c.conn.Close()
	}
	runtime.EventsEmit(c.ctx, "agent:status", "Cancelled")
	runtime.EventsEmit(c.ctx, "agent:error", "Task was cancelled by user.")
}
