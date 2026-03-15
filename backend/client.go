package backend

import (
	"context"
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type AgentClient struct {
	ctx      context.Context
	conn     *websocket.Conn
	spssPath string
	dataPath string
}

func NewAgentClient(ctx context.Context) *AgentClient {
	return &AgentClient{
		ctx: ctx,
	}
}

// Connect opens the WebSocket to the Ruby server and sends the initialization payload.
func (c *AgentClient) Connect(url string, prompt string, spssPath string, dataPath string) error {
	log.Printf("Connecting to Agent Server: %s", url)

	c.spssPath = spssPath
	c.dataPath = dataPath

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		runtime.EventsEmit(c.ctx, "agent:error", err.Error())
		return err
	}
	c.conn = conn

	// Send Init Payload
	initMsg := map[string]interface{}{
		"type":   "init",
		"prompt": prompt,
		"schema": `{"vars": ["gender", "age"]}`, // Example schema
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

		// Specifically handle execute_syntax by mimicking SPSS (Mock for now)
		if msgType == "execute_syntax" {
			syntax, _ := payload["syntax"].(string)
			log.Printf("Executing SPSS syntax:\n%s\n", syntax)

			// TODO: Actually invoke local SPSS `stats` here
			// For now, mock success:
			mockSuccess := map[string]interface{}{
				"type":   "execution_result",
				"status": "success",
				"output": "[Mock Runner] Output for command:\n" + syntax,
			}
			c.conn.WriteJSON(mockSuccess)
		} else if msgType == "finished" {
			log.Printf("Agent finished. Closing connection.")
			break
		}
	}
}
