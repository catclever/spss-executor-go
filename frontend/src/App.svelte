<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { ConnectServer, GetDevEnvironment, CancelExecution, SelectSPSSBinary, SelectDataFile, FetchDictionary, GetAppConfig } from '../wailsjs/go/main/App.js';
  import { EventsOn, EventsOff, BrowserOpenURL } from '../wailsjs/runtime/runtime.js';

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
  
  // Session tracking variables
  let activeSessionDataPath: string = "";
  let workingNote: string = "";
  
  // To store the stream of messages and executions
  let logs: { type: string, text: string, time: string }[] = [];
  let expandedLogs: Set<number> = new Set();
  
  // Settings / LLM configs
  let showConfig: boolean = false;
  let llmFormat: string = "openai";
  let llmBaseUrl: string = "https://open.bigmodel.cn/api/coding/paas/v4";
  let llmModel: string = "glm-4.7";
  let llmApiKey: string = "";
  let llmTemperature: number = 0.95;

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

  $: if (isLoaded) localStorage.setItem('spssAgent_serverUrl', serverUrl);
  $: if (isLoaded) localStorage.setItem('spssAgent_spssPath', spssPath);
  $: if (isLoaded) localStorage.setItem('spssAgent_dataPath', dataPath);
  $: if (isLoaded) localStorage.setItem('spssAgent_promptText', promptText);
  $: if (isLoaded) localStorage.setItem('spssAgent_usePD', String(usePD));
  $: if (isLoaded) localStorage.setItem('spssAgent_vmName', vmName);
  $: if (isLoaded) localStorage.setItem('spssAgent_llmFormat', llmFormat);
  $: if (isLoaded) localStorage.setItem('spssAgent_llmBaseUrl', llmBaseUrl);
  $: if (isLoaded) localStorage.setItem('spssAgent_llmModel', llmModel);
  $: if (isLoaded) localStorage.setItem('spssAgent_llmApiKey', llmApiKey);
  $: if (isLoaded) localStorage.setItem('spssAgent_llmTemperature', String(llmTemperature));

  function addLog(type: string, text: string) {
    const time = new Date().toLocaleTimeString();
    logs = [...logs, { type, text, time }];
    
    // Auto-scroll logic could be added here
    setTimeout(() => {
      const el = document.getElementById("log-container");
      if(el) el.scrollTop = el.scrollHeight;
    }, 100);
  }

  function toggleExpand(index: number) {
    if (expandedLogs.has(index)) {
      expandedLogs.delete(index);
    } else {
      expandedLogs.add(index);
    }
    expandedLogs = expandedLogs; // trigger reactivity
  }

  onMount(async () => {
    // Load persisted state from localStorage
    const savedUrl = localStorage.getItem('spssAgent_serverUrl');
    const savedSpss = localStorage.getItem('spssAgent_spssPath');
    const savedData = localStorage.getItem('spssAgent_dataPath');
    const savedPrompt = localStorage.getItem('spssAgent_promptText');
    const savedUsePD = localStorage.getItem('spssAgent_usePD');
    const savedVmName = localStorage.getItem('spssAgent_vmName');
    
    const savedLlmFormat = localStorage.getItem('spssAgent_llmFormat');
    const savedLlmBaseUrl = localStorage.getItem('spssAgent_llmBaseUrl');
    const savedLlmModel = localStorage.getItem('spssAgent_llmModel');
    const savedLlmApiKey = localStorage.getItem('spssAgent_llmApiKey');
    const savedLlmTemperature = localStorage.getItem('spssAgent_llmTemperature');

    let config: any = {};
    try {
      config = await GetAppConfig();
    } catch (e) {
      console.warn("Failed to load config", e);
      config = { serverUrl: "ws://localhost:9292", spssPath: "C:\\Program Files\\IBM\\SPSS Statistics\\28\\stats.exe", llmModel: "glm-4.7", apiKey: "" };
    }

    if (savedUrl) serverUrl = savedUrl; else serverUrl = config.serverUrl;
    if (savedPrompt) promptText = savedPrompt;
    if (savedUsePD) usePD = savedUsePD === 'true';
    if (savedVmName) vmName = savedVmName;
    if (savedLlmFormat) llmFormat = savedLlmFormat;
    if (savedLlmBaseUrl) llmBaseUrl = savedLlmBaseUrl;
    if (savedLlmModel) llmModel = savedLlmModel; else llmModel = config.llmModel;
    
    if (!savedLlmApiKey && config.apiKey) {
      llmApiKey = config.apiKey.trim();
    } else if (savedLlmApiKey) {
      llmApiKey = savedLlmApiKey;
    }
    if (savedLlmTemperature && !isNaN(parseFloat(savedLlmTemperature))) llmTemperature = parseFloat(savedLlmTemperature);

    // Detect OS and set defaults only if no saved path exists
    try {
      const env = await GetDevEnvironment();
      osType = env.os;
      if (!savedSpss || !savedData) {
        if (osType === "windows") {
          spssPath = savedSpss || config.spssPath;
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
      if (status !== "Waiting SPSS...") {
        addLog("info", status);
      }
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
        addLog("spss-out", msg.message);
      } else if (msg.type === "thinking") {
        addLog("thinking", msg.message);
      } else if (msg.type === "status") {
        addLog("info", "Agent: " + msg.message);
      } else if (msg.type === "finished") {
        addLog("success", "Agent Task Completed!");
        
        if (msg.analysis_summary) {
          addLog("summary", msg.analysis_summary);
        }
        
        if (msg.final_syntax) {
          addLog("final-syntax", msg.final_syntax);
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

  async function connect() {
    if (isConnected) return;
    
    if (!serverUrl || !promptText || !spssPath || !dataPath) {
      addLog("error", "All fields are required!");
      return;
    }

    // New Session Logic: Fetch dictionary if dataset changed
    if (dataPath !== activeSessionDataPath) {
      isConnected = true;
      statusText = "Syncing Metadata...";
      addLog("system", "Fetching Data Dictionary locally without AI...");
      
      try {
        const fetchResult = await FetchDictionary(spssPath, dataPath, usePD, vmName);
        workingNote = "DATA DICTIONARY / METADATA:\n" + fetchResult;
        activeSessionDataPath = dataPath;
        addLog("success", "Metadata synced successfully. Starting AI Session...");
      } catch (err) {
        addLog("error", "Failed to fetch Dictionary: " + err);
        isConnected = false;
        statusText = "Disconnected";
        return;
      }
    }

    isConnected = true;
    statusText = "Connecting...";
    addLog("system", "Dialing " + serverUrl + " ...");

    const configObj: any = {
      format: llmFormat,
      base_url: llmBaseUrl,
      model: llmModel,
      api_key: llmApiKey
    };
    
    // Explicitly omit temperature if model is kimi / moonshot
    if (!llmModel.toLowerCase().includes("kimi") && !llmModel.toLowerCase().includes("moonshot")) {
      configObj.temperature = llmTemperature;
    }
    
    const llmConfig = JSON.stringify(configObj);

    ConnectServer(serverUrl, promptText, spssPath, dataPath, usePD, vmName, workingNote, llmConfig).catch(err => {
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

  async function handleSelectSPSS() {
    try {
      const selectedPath = await SelectSPSSBinary();
      if (selectedPath && selectedPath.trim() !== "") {
        spssPath = selectedPath;
      }
    } catch (err) {
      addLog("error", "Failed to select SPSS binary: " + err);
    }
  }

  async function handleSelectData() {
    try {
      const selectedPath = await SelectDataFile();
      if (selectedPath && selectedPath.trim() !== "") {
        dataPath = selectedPath;
      }
    } catch (err) {
      addLog("error", "Failed to select Data dataset: " + err);
    }
  }

  async function copyToClipboard(text: string, target: EventTarget | null = null) {
    try {
      await navigator.clipboard.writeText(text);
      if (target instanceof HTMLElement) {
        const originalText = target.innerText;
        target.innerText = "Copied!";
        target.style.backgroundColor = "#a6e3a1";
        target.style.color = "#11111b";
        setTimeout(() => {
          target.innerText = originalText;
          target.style.backgroundColor = "";
          target.style.color = "";
        }, 1500);
      }
    } catch (err) {
      console.error("Failed to copy", err);
    }
  }

  async function syncMetadata() {
    if (!spssPath || !dataPath) {
      addLog("error", "SPSS Path and Data Path are required to sync metadata.");
      return;
    }
    isConnected = true;
    statusText = "Syncing Metadata...";
    addLog("system", "Force Fetching Data Dictionary locally...");
    
    try {
      const fetchResult = await FetchDictionary(spssPath, dataPath, usePD, vmName);
      workingNote = "DATA DICTIONARY / METADATA:\n" + fetchResult;
      activeSessionDataPath = dataPath;
      addLog("success", "Metadata synced successfully.");
    } catch (err) {
      addLog("error", "Failed to fetch Dictionary: " + err);
    } finally {
      isConnected = false;
      statusText = "Disconnected";
    }
  }

  function startNewSession() {
    // If running, kill it
    if (isConnected) {
      CancelExecution();
    }
    isConnected = false;
    activeSessionDataPath = "";
    workingNote = "";
    logs = [];
    expandedLogs.clear();
    expandedLogs = expandedLogs;
    statusText = "Disconnected";
  }
</script>

<main class="app-container">
  <div class="sidebar">
    {#if !showConfig}
      <div class="sidebar-header">
        <h2>SPSS Agent</h2>
        <button class="icon-btn" on:click={() => showConfig = true} title="Settings">⚙️</button>
      </div>

      <div class="form-group main-input-group">
        <label for="data">Dataset Path (.sav)</label>
        <div class="input-with-button">
          <input id="data" type="text" bind:value={dataPath} disabled={isConnected || activeSessionDataPath !== ""} />
          <button class="btn-select" on:click={handleSelectData} disabled={isConnected || activeSessionDataPath !== ""}>Select</button>
          <button class="btn-sync" on:click={syncMetadata} disabled={isConnected} title="Force refresh metadata from file">🔄</button>
        </div>
      </div>

      <div class="form-group main-input-group" style="flex: 1">
        <label for="prompt">Your Goal / Prompt</label>
        <textarea id="prompt" bind:value={promptText} disabled={isConnected} style="height: 100%; resize: none;"></textarea>
      </div>

      <div class="action-buttons">
        <button class="btn-connect" on:click={connect} disabled={isConnected}>
          {isConnected ? 'Running Agent...' : 'Start Execution'}
        </button>

        {#if isConnected}
          <button class="btn-cancel" on:click={cancelTask}>
            Cancel Task
          </button>
        {:else if activeSessionDataPath !== ""}
          <button class="btn-new-session" on:click={startNewSession}>
            Start New Session
          </button>
        {/if}
      </div>



    {:else}
      <div class="sidebar-header">
        <h2>Configurations</h2>
        <button class="btn-done-small" on:click={() => showConfig = false} title="Done">Done</button>
      </div>

      <div class="config-scroll-area">
        <div class="config-section">
          <h3>Connection Setup</h3>
          <div class="form-group">
            <label for="url">Ruby Server URL</label>
            <input id="url" type="text" bind:value={serverUrl} disabled={isConnected} autocapitalize="none" autocorrect="off" spellcheck="false" />
          </div>
          <div class="form-group">
            <label for="spss">
              {usePD ? 'SPSS Path (In VM)' : 'SPSS Binary Path'}
            </label>
            <div class="input-with-button">
              <input id="spss" type="text" bind:value={spssPath} disabled={isConnected} />
              <button class="btn-select" on:click={handleSelectSPSS} disabled={isConnected || usePD}>...Select</button>
            </div>
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
        </div>

        <div class="config-section">
          <h3>LLM Properties</h3>
          <div class="form-group">
            <label for="llmFormat">Format</label>
            <select id="llmFormat" bind:value={llmFormat} disabled={isConnected}>
              <option value="openai">OpenAI Compatible</option>
              <option value="anthropic">Anthropic</option>
              <option value="gemini">Gemini</option>
            </select>
          </div>
          <div class="form-group">
            <label for="llmBaseUrl">Base URL</label>
            <input id="llmBaseUrl" type="text" bind:value={llmBaseUrl} disabled={isConnected} />
          </div>
          <div class="form-group">
            <label for="llmModel">Model Name</label>
            <input id="llmModel" type="text" bind:value={llmModel} disabled={isConnected} />
          </div>
          <div class="form-group">
            <label for="llmApiKey">API Key</label>
            <input id="llmApiKey" type="password" bind:value={llmApiKey} disabled={isConnected} placeholder="Enter API Key" />
          </div>
          <div class="form-group">
            <label for="llmTemperature">Temperature</label>
            <input id="llmTemperature" type="number" min="0" max="2" step="0.1" bind:value={llmTemperature} disabled={isConnected || llmModel.toLowerCase().includes('kimi') || llmModel.toLowerCase().includes('moonshot')} title={(llmModel.toLowerCase().includes('kimi') || llmModel.toLowerCase().includes('moonshot')) ? "Temperature adjustment is locked for Kimi APIs" : ""} />
          </div>
        </div>
      </div>
    {/if}
    <div class="sidebar-footer">
      <a href="https://github.com/catclever/descartes" on:click|preventDefault={() => BrowserOpenURL("https://github.com/catclever/descartes")}>powered by descartes</a>
    </div>
  </div>

  <div class="status-badge">
    Status: <span class={isConnected ? 'active' : ''}>{statusText}</span>
  </div>

  <div class="log-panel" id="log-container">
    {#if logs.length === 0}
      <div class="empty-state">Waiting for execution to start...</div>
    {/if}
    
    {#each logs as log, i}
      <div class="log-entry type-{log.type}">
        <div class="log-header">
          <span class="tag">{log.type.toUpperCase().replace("-", " ")}</span>
          <span class="log-time">[{log.time}]</span>
          {#if log.type === 'syntax' || log.type === 'final-syntax'}
            <button class="btn-copy" on:click={(e) => copyToClipboard(log.text, e.currentTarget)}>Copy</button>
          {/if}
        </div>
        
        {#if log.type === 'spss-out'}
          {@const lines = log.text.trim().split('\n')}
          {@const last3 = lines.slice(Math.max(lines.length - 3, 0)).join('\n')}
          {@const isExpanded = expandedLogs.has(i)}
          
          {#if !isExpanded}
            <div class="log-text spss-box collapsed" on:click={() => toggleExpand(i)} on:keydown={(e) => e.key === 'Enter' && toggleExpand(i)} tabindex="0" role="button">
              <div class="spss-preview">
                {#if lines.length > 3}
                  <div class="spss-trunc-dots">...</div>
                {/if}
                {last3}
              </div>
              <div class="spss-expand-hint">Click to fully expand SPSS Output</div>
            </div>
          {:else}
            <div class="log-text spss-box expanded" on:click={() => toggleExpand(i)} on:keydown={(e) => e.key === 'Enter' && toggleExpand(i)} tabindex="0" role="button">
              {log.text}
              <div class="spss-expand-hint mt-2">Click to collapse</div>
            </div>
          {/if}
        {:else if log.type === 'final-syntax'}
          <div class="log-text syntax-box">{log.text}</div>
        {:else}
          <div class="log-text">{log.text}</div>
        {/if}
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
    position: relative;
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

  .sidebar-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid #313244;
    padding-bottom: 10px;
    margin-bottom: 10px;
  }

  .sidebar-footer {
    margin-top: auto;
    text-align: center;
    padding-top: 15px;
    border-top: 1px solid #313244;
  }
  
  .sidebar-footer a {
    color: #89b4fa;
    text-decoration: none;
    font-size: 0.85em;
    opacity: 0.8;
    transition: opacity 0.2s;
  }
  
  .sidebar-footer a:hover {
    opacity: 1;
    text-decoration: underline;
  }

  .sidebar-header h2 {
    margin: 0;
    font-size: 1.3rem;
    color: #89b4fa;
  }

  .icon-btn {
    background: transparent;
    border: none;
    font-size: 1.2rem;
    cursor: pointer;
    transition: transform 0.2s;
  }

  .icon-btn:hover {
    transform: scale(1.1);
  }

  .btn-done-small {
    background-color: #a6e3a1;
    color: #11111b;
    border: none;
    border-radius: 4px;
    padding: 6px 14px;
    font-size: 0.85rem;
    font-weight: 600;
    cursor: pointer;
    transition: background-color 0.2s, transform 0.1s;
  }

  .btn-done-small:hover {
    background-color: #b3efad;
  }

  .btn-done-small:active {
    transform: scale(0.95);
  }

  .main-input-group {
    margin-bottom: 15px;
  }

  .action-buttons {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .config-scroll-area {
    overflow-y: auto;
    flex: 1;
    padding-right: 5px;
  }
  
  .config-scroll-area::-webkit-scrollbar {
    width: 6px;
  }
  .config-scroll-area::-webkit-scrollbar-thumb {
    background-color: #313244;
    border-radius: 4px;
  }

  .config-section {
    background-color: #1e1e2e;
    padding: 12px;
    border-radius: 8px;
    margin-bottom: 15px;
    border: 1px solid #313244;
  }

  .config-section h3 {
    margin-top: 0;
    margin-bottom: 12px;
    font-size: 0.95rem;
    color: #f38ba8;
    border-bottom: 1px solid #313244;
    padding-bottom: 6px;
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

  input[type="text"], input[type="password"], input[type="number"], textarea, select {
    background-color: #11111b;
    border: 1px solid #313244;
    color: #cdd6f4;
    padding: 10px;
    border-radius: 6px;
    font-size: 0.9rem;
    outline: none;
    transition: border-color 0.2s;
  }

  input:focus, textarea:focus, select:focus {
    border-color: #89b4fa;
  }

  input:disabled, textarea:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .input-with-button {
    display: flex;
    gap: 8px;
  }

  .input-with-button input {
    flex: 1;
    min-width: 0;
  }

  .btn-select {
    background-color: #313244;
    color: #cdd6f4;
    border: 1px solid #45475a;
    border-radius: 6px;
    padding: 0 12px;
    cursor: pointer;
    font-size: 0.85rem;
    transition: all 0.2s;
  }

  .btn-select:hover:not(:disabled) {
    background-color: #45475a;
    border-color: #89b4fa;
  }

  .btn-select:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .btn-sync {
    background-color: #313244;
    color: #cdd6f4;
    border: 1px solid #45475a;
    border-radius: 6px;
    padding: 0 10px;
    cursor: pointer;
    font-size: 0.95rem;
    transition: all 0.2s;
  }

  .btn-sync:hover:not(:disabled) {
    background-color: #45475a;
    border-color: #89b4fa;
  }

  .btn-sync:disabled {
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
  }

  .btn-cancel:hover {
    background-color: #eba0ac;
  }

  .btn-cancel:active {
    transform: scale(0.98);
  }

  .btn-new-session {
    background-color: #a6e3a1;
    color: #11111b;
    border: none;
    padding: 12px;
    border-radius: 6px;
    font-weight: 600;
    cursor: pointer;
    transition: background-color 0.2s, transform 0.1s;
  }

  .btn-new-session:hover {
    background-color: #94e2d5;
  }

  .btn-new-session:active {
    transform: scale(0.98);
  }

  .status-badge {
    position: absolute;
    top: 20px;
    right: 20px;
    background-color: rgba(24, 24, 37, 0.8);
    backdrop-filter: blur(4px);
    padding: 6px 12px;
    border-radius: 6px;
    font-size: 0.85rem;
    color: #a6adc8;
    border: 1px solid #313244;
    z-index: 10;
  }

  .status-badge span.active {
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
    position: relative;
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

  .log-header {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 8px;
  }

  .tag {
    font-size: 0.7rem;
    font-weight: 700;
    padding: 2px 6px;
    border-radius: 4px;
    background-color: #313244;
    color: #cdd6f4;
  }

  .log-time {
    font-size: 0.7rem;
    color: #6c7086;
    user-select: none;
    flex: 1;
  }

  .btn-copy {
    background-color: #45475a;
    color: #cdd6f4;
    border: none;
    border-radius: 4px;
    padding: 4px 8px;
    font-size: 0.7rem;
    cursor: pointer;
    transition: background 0.2s;
  }

  .btn-copy:hover {
    background-color: #585b70;
  }

  .btn-copy:hover {
    background-color: #585b70;
  }
  
  .spss-preview {
    font-family: 'Fira Code', monospace;
    font-size: 0.75rem;
    color: #a6adc8;
    white-space: pre-wrap;
    margin-bottom: 6px;
  }
  
  .spss-trunc-dots {
    color: #585b70;
    margin-bottom: 2px;
  }
  
  .spss-expand-hint {
    font-size: 0.7rem;
    color: #89b4fa;
    font-weight: 600;
  }

  .mt-2 {
    margin-top: 8px;
  }

  .spss-box {
    background-color: #11111b;
    padding: 10px;
    border-radius: 6px;
    border: 1px solid #313244;
    cursor: pointer;
    transition: background-color 0.2s;
  }
  
  .spss-box:hover {
    background-color: #181825;
  }

  .syntax-box {
    background-color: transparent;
    color: inherit;
    padding: 8px 0;
    font-weight: 600;
  }

  .log-entry.type-summary { border-left-color: #a6e3a1; background-color: rgba(166, 227, 161, 0.1); }
  .log-entry.type-summary .tag { background-color: #a6e3a1; color: #11111b; }

  .log-entry.type-info { border-left-color: #89b4fa; }
  .log-entry.type-info .tag { background-color: #89b4fa; color: #11111b; }
  
  .log-entry.type-thinking { border-left-color: #8caaee; background-color: rgba(140, 170, 238, 0.1); }
  .log-entry.type-thinking .tag { background-color: #8caaee; color: #11111b; }

  .log-entry.type-syntax { border-left-color: #cba6f7; background-color: #313244; }
  .log-entry.type-syntax .tag { background-color: #cba6f7; color: #11111b; }

  .log-entry.type-final-syntax { border-left-color: #938aa9; background-color: #938aa9; color: #11111b; }
  .log-entry.type-final-syntax .tag { background-color: #11111b; color: #938aa9; }
  .log-entry.type-final-syntax .log-time { color: #313244; }
  .log-entry.type-final-syntax .btn-copy { background-color: #11111b; color: #938aa9; }
  .log-entry.type-final-syntax .btn-copy:hover { background-color: #313244; }

  .log-entry.type-spss-out { border-left-color: #fab387; background-color: #1e1e2e; color: #bac2de; font-size: 0.8rem; }
  .log-entry.type-spss-out .tag { background-color: #fab387; color: #11111b; }

  .log-entry.type-error { border-left-color: #f38ba8; color: #f38ba8; }
  .log-entry.type-error .tag { background-color: #f38ba8; color: #11111b; }

  .log-entry.type-success { border-left-color: #a6e3a1; }
  .log-entry.type-success .tag { background-color: #a6e3a1; color: #11111b; }

  .log-entry.type-system { border-left-color: #f9e2af; }
  .log-entry.type-system .tag { background-color: #f9e2af; color: #11111b; }
</style>
