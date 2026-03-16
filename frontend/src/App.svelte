<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { ConnectServer, GetDevEnvironment, CancelExecution } from '../wailsjs/go/main/App.js';
  import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime.js';

  let osType: string = "";
  let serverUrl: string = "ws://localhost:9292";
  let spssPath: string = "";
  let dataPath: string = "";
  let promptText: string = "我想要计算 gender 变量各个类别的数量以及所占百分比";
  
  // Parallels Desktop settings
  let usePD: boolean = false;
  let vmName: string = "Windows 11";

  // Flag to prevent reactive saves during initial mount load
  let isLoaded: boolean = false;

  let isConnected: boolean = false;
  let statusText: string = "Disconnected";
  
  // To store the stream of messages and executions
  let logs: { type: string, text: string, time: string }[] = [];

  // Reactive statement to auto-switch default paths based on Parallels toggle
  // Only execute this if the user actively toggles it (after load) rather than on initial load
  $: if (isLoaded && (osType === 'darwin' || osType === 'mac')) {
    if (usePD) {
      // If switching to PD and current path is the Mac default, change to Windows default
      if (spssPath === "/Applications/IBM SPSS Statistics/SPSS Statistics.app/Contents/MacOS/stats") {
        spssPath = "C:\\Program Files\\IBM\\SPSS Statistics\\28\\stats.exe";
      }
    } else {
      // If switching off PD and current path is the Windows default, change to Mac default
      if (spssPath === "C:\\Program Files\\IBM\\SPSS Statistics\\28\\stats.exe") {
        spssPath = "/Applications/IBM SPSS Statistics/SPSS Statistics.app/Contents/MacOS/stats";
      }
    }
  }

  // Reactive statements to save state to localStorage
  $: if (isLoaded) localStorage.setItem('spssAgent_serverUrl', serverUrl);
  $: if (isLoaded) localStorage.setItem('spssAgent_spssPath', spssPath);
  $: if (isLoaded) localStorage.setItem('spssAgent_dataPath', dataPath);
  $: if (isLoaded) localStorage.setItem('spssAgent_promptText', promptText);
  $: if (isLoaded) localStorage.setItem('spssAgent_usePD', String(usePD));
  $: if (isLoaded) localStorage.setItem('spssAgent_vmName', vmName);

  function addLog(type: string, text: string) {
    const time = new Date().toLocaleTimeString();
    logs = [...logs, { type, text, time }];
    
    // Auto-scroll logic could be added here
    setTimeout(() => {
      const el = document.getElementById("log-container");
      if(el) el.scrollTop = el.scrollHeight;
    }, 100);
  }

  onMount(async () => {
    // Load persisted state from localStorage
    const savedUrl = localStorage.getItem('spssAgent_serverUrl');
    const savedSpss = localStorage.getItem('spssAgent_spssPath');
    const savedData = localStorage.getItem('spssAgent_dataPath');
    const savedPrompt = localStorage.getItem('spssAgent_promptText');
    const savedUsePD = localStorage.getItem('spssAgent_usePD');
    const savedVmName = localStorage.getItem('spssAgent_vmName');

    if (savedUrl) serverUrl = savedUrl;
    if (savedPrompt) promptText = savedPrompt;
    if (savedUsePD) usePD = savedUsePD === 'true';
    if (savedVmName) vmName = savedVmName;

    // Detect OS and set defaults only if no saved path exists
    try {
      const env = await GetDevEnvironment();
      osType = env.os;
      if (!savedSpss || !savedData) {
        if (osType === "windows") {
          spssPath = savedSpss || "C:\\Program Files\\IBM\\SPSS Statistics\\28\\stats.exe";
          dataPath = savedData || "C:\\Data\\example.sav";
        } else {
          spssPath = savedSpss || "/Applications/IBM SPSS Statistics/SPSS Statistics.app/Contents/MacOS/stats";
          dataPath = savedData || "/Users/kael/Data/example.sav";
        }
      } else {
        spssPath = savedSpss;
        dataPath = savedData;
      }
    } catch (e) {
      console.error("Failed to get dev env", e);
    }

    isLoaded = true; // Enable reactive saving

    EventsOn("agent:status", (status: string) => {
      statusText = status;
      if (status === "Disconnected" || status === "Cancelled") {
        isConnected = false;
      }
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
      } else if (msg.type === "spss_output") {
        addLog("spss-out", "SPSS Output:\n" + msg.message);
      } else if (msg.type === "status") {
        addLog("info", "Agent: " + msg.message);
      } else if (msg.type === "finished") {
        addLog("success", "Agent Task Completed!");
        
        if (msg.analysis_summary) {
          addLog("info", "Analysis Summary:\n" + msg.analysis_summary);
        }
        
        if (msg.final_syntax) {
          addLog("syntax", "Final Executable Syntax:\n" + msg.final_syntax);
        }

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
    
    if (!serverUrl || !promptText || !spssPath || !dataPath) {
      addLog("error", "All fields are required!");
      return;
    }

    isConnected = true;
    statusText = "Connecting...";
    addLog("system", "Dialing " + serverUrl + " ...");

    ConnectServer(serverUrl, promptText, spssPath, dataPath, usePD, vmName).catch(err => {
      addLog("error", "Failed to connect: " + err);
      isConnected = false;
      statusText = "Disconnected";
    });
  }

  function cancelTask() {
    if (!isConnected) return;
    addLog("system", "Sending cancellation signal...");
    CancelExecution().catch(err => {
      addLog("error", "Failed to cancel: " + err);
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
      <label for="spss">
        {usePD ? 'SPSS Binary Path (Inside VM)' : 'SPSS Binary Path'}
      </label>
      <input id="spss" type="text" bind:value={spssPath} disabled={isConnected} />
    </div>

    <div class="form-group">
      <label for="data">Dataset Path (.sav)</label>
      <input id="data" type="text" bind:value={dataPath} disabled={isConnected} />
    </div>

    {#if osType === 'darwin' || osType === 'mac'}
      <div class="form-group pd-toggle">
        <label>
          <input type="checkbox" bind:checked={usePD} disabled={isConnected} />
          <span class="checkbox-label">Use Parallels Desktop VM</span>
        </label>
      </div>
      
      {#if usePD}
        <div class="form-group indent">
          <label for="vmname">VM Name</label>
          <input id="vmname" type="text" bind:value={vmName} disabled={isConnected} />
        </div>
      {/if}
    {/if}

    <div class="form-group">
      <label for="prompt">Your Goal / Prompt</label>
      <textarea id="prompt" bind:value={promptText} rows="5" disabled={isConnected}></textarea>
    </div>

    <button class="btn-connect" on:click={connect} disabled={isConnected}>
      {isConnected ? 'Running Agent...' : 'Start Execution'}
    </button>

    {#if isConnected}
      <button class="btn-cancel" on:click={cancelTask}>
        Cancel Task
      </button>
    {/if}

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

  .pd-toggle label {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
  }
  
  .pd-toggle input[type="checkbox"] {
    margin: 0;
    cursor: pointer;
  }

  .checkbox-label {
    color: #f9e2af;
  }

  .indent {
    margin-left: 10px;
    border-left: 2px solid #45475a;
    padding-left: 10px;
  }

  input[type="text"], textarea {
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

  .btn-cancel {
    background-color: #f38ba8;
    color: #11111b;
    border: none;
    padding: 12px;
    border-radius: 6px;
    font-weight: 600;
    cursor: pointer;
    transition: background-color 0.2s, transform 0.1s;
    margin-top: -10px; /* pull closer to the connect button */
  }

  .btn-cancel:hover {
    background-color: #eba0ac;
  }

  .btn-cancel:active {
    transform: scale(0.98);
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
  .log-entry.type-spss-out { border-left-color: #fab387; background-color: #1e1e2e; color: #bac2de; font-size: 0.8rem; }
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
