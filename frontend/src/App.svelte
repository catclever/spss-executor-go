<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { ConnectServer } from '../wailsjs/go/main/App.js';
  import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime.js';

  let serverUrl: string = "ws://localhost:9292";
  let promptText: string = "我想要计算 gender 变量各个类别的数量以及所占百分比";
  let isConnected: boolean = false;
  let statusText: string = "Disconnected";
  
  // To store the stream of messages and executions
  let logs: { type: string, text: string, time: string }[] = [];

  function addLog(type: string, text: string) {
    const time = new Date().toLocaleTimeString();
    logs = [...logs, { type, text, time }];
    
    // Auto-scroll logic could be added here
    setTimeout(() => {
      const el = document.getElementById("log-container");
      if(el) el.scrollTop = el.scrollHeight;
    }, 100);
  }

  onMount(() => {
    EventsOn("agent:status", (status: string) => {
      statusText = status;
      addLog("info", status);
    });

    EventsOn("agent:error", (error: string) => {
      statusText = "Error!";
      isConnected = false;
      addLog("error", error);
    });

    EventsOn("agent:message", (msg: any) => {
      if (msg.type === "execute_syntax") {
        addLog("syntax", "Executing Syntax:\n" + msg.syntax);
      } else if (msg.type === "status") {
        addLog("info", "Agent: " + msg.message);
      } else if (msg.type === "finished") {
        addLog("success", "Agent Task Completed!");
        isConnected = false;
        statusText = "Finished";
      } else if (msg.type === "error") {
        addLog("error", "Agent Error: " + msg.message);
        isConnected = false;
      }
    });
  });

  onDestroy(() => {
    EventsOff("agent:status");
    EventsOff("agent:error");
    EventsOff("agent:message");
  });

  function connect() {
    if (isConnected) return;
    
    if (!serverUrl || !promptText) {
      addLog("error", "URL and Prompt are required!");
      return;
    }

    isConnected = true;
    statusText = "Connecting...";
    addLog("system", "Dialing " + serverUrl + " ...");

    ConnectServer(serverUrl, promptText).catch(err => {
      addLog("error", "Failed to connect: " + err);
      isConnected = false;
      statusText = "Disconnected";
    });
  }
</script>

<main class="app-container">
  <div class="sidebar">
    <h2>Descartes Agent</h2>
    <div class="form-group">
      <label for="url">Ruby Server URL</label>
      <input id="url" type="text" bind:value={serverUrl} disabled={isConnected} />
    </div>

    <div class="form-group">
      <label for="prompt">Your Goal / Prompt</label>
      <textarea id="prompt" bind:value={promptText} rows="5" disabled={isConnected}></textarea>
    </div>

    <button class="btn-connect" on:click={connect} disabled={isConnected}>
      {isConnected ? 'Running Agent...' : 'Start Execution'}
    </button>

    <div class="status-indicator">
      Status: <span class={isConnected ? 'active' : ''}>{statusText}</span>
    </div>
  </div>

  <div class="log-panel" id="log-container">
    {#if logs.length === 0}
      <div class="empty-state">Waiting for execution to start...</div>
    {/if}
    
    {#each logs as log}
      <div class="log-entry type-{log.type}">
        <div class="log-time">[{log.time}]</div>
        <div class="log-text">{log.text}</div>
      </div>
    {/each}
  </div>
</main>

<style>
  :global(body) {
    margin: 0;
    font-family: 'Inter', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    background-color: #1e1e2e;
    color: #cdd6f4;
  }

  .app-container {
    display: flex;
    height: 100vh;
    width: 100vw;
    overflow: hidden;
  }

  .sidebar {
    width: 320px;
    background-color: #181825;
    border-right: 1px solid #313244;
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 20px;
    box-shadow: 2px 0 10px rgba(0,0,0,0.2);
  }

  .sidebar h2 {
    margin: 0;
    font-size: 1.5rem;
    color: #89b4fa;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  label {
    font-size: 0.85rem;
    color: #a6adc8;
    font-weight: 600;
  }

  input, textarea {
    background-color: #11111b;
    border: 1px solid #313244;
    color: #cdd6f4;
    padding: 10px;
    border-radius: 6px;
    font-size: 0.9rem;
    outline: none;
    transition: border-color 0.2s;
  }

  input:focus, textarea:focus {
    border-color: #89b4fa;
  }

  input:disabled, textarea:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .btn-connect {
    background-color: #89b4fa;
    color: #11111b;
    border: none;
    padding: 12px;
    border-radius: 6px;
    font-weight: 600;
    cursor: pointer;
    transition: background-color 0.2s, transform 0.1s;
  }

  .btn-connect:hover:not(:disabled) {
    background-color: #b4befe;
  }

  .btn-connect:active:not(:disabled) {
    transform: scale(0.98);
  }

  .btn-connect:disabled {
    background-color: #45475a;
    color: #a6adc8;
    cursor: not-allowed;
  }

  .status-indicator {
    margin-top: auto;
    font-size: 0.85rem;
    color: #a6adc8;
  }

  .status-indicator span.active {
    color: #a6e3a1;
    font-weight: bold;
  }

  .log-panel {
    flex: 1;
    background-color: #1e1e2e;
    padding: 20px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .empty-state {
    margin: auto;
    color: #585b70;
    font-style: italic;
  }

  .log-entry {
    background-color: #181825;
    border-radius: 8px;
    padding: 12px 16px;
    border-left: 4px solid #45475a;
    font-family: 'Fira Code', monospace;
    font-size: 0.85rem;
    word-break: break-all;
    white-space: pre-wrap;
  }

  .log-entry.type-info { border-left-color: #89b4fa; }
  .log-entry.type-syntax { border-left-color: #cba6f7; background-color: #313244; }
  .log-entry.type-error { border-left-color: #f38ba8; color: #f38ba8; }
  .log-entry.type-success { border-left-color: #a6e3a1; }
  .log-entry.type-system { border-left-color: #f9e2af; }

  .log-time {
    font-size: 0.7rem;
    color: #6c7086;
    margin-bottom: 4px;
    user-select: none;
  }
</style>
