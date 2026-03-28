const providerDefaults = {
  chatgpt: { label: "chatgpt", model: "gpt-5.2-codex", oauth: true },
  openai: { label: "openai", model: "gpt-4o", apiKey: true },
  openrouter: { label: "openrouter", model: "openai/gpt-4o", apiKey: true },
  anthropic: { label: "anthropic", model: "claude-sonnet-4-5", apiKey: true },
  gemini: { label: "gemini", model: "gemini-2.0-flash", apiKey: true },
  ollama: { label: "ollama", model: "", url: "http://localhost:11434/v1", local: true },
  lmstudio: { label: "lm studio", model: "", url: "http://localhost:1234/v1", local: true }
};

const state = {
  sessions: [],
  activeSessionId: "",
  activeSnapshot: null,
  catalogSocket: null,
  sessionSocket: null,
  activeRunId: "",
  search: "",
  oauthProviderId: "",
  oauthConnected: false,
  backendOnline: false,
  config: loadStoredConfig(),
  localEntries: []
};

const backendStatus = document.getElementById("backendStatus");
const providerAction = document.getElementById("providerAction");
const providerStatus = document.getElementById("providerStatus");
const newSessionButton = document.getElementById("newSessionButton");
const sessionSearch = document.getElementById("sessionSearch");
const sessionList = document.getElementById("sessionList");
const sessionTitle = document.getElementById("sessionTitle");
const sessionModeLabel = document.getElementById("sessionModeLabel");
const sessionState = document.getElementById("sessionState");
const modeSelect = document.getElementById("modeSelect");
const connectPanel = document.getElementById("connectPanel");
const connectButton = document.getElementById("connectButton");
const emptyPanel = document.getElementById("emptyPanel");
const transcript = document.getElementById("transcript");
const composer = document.getElementById("composer");
const promptInput = document.getElementById("promptInput");
const sendButton = document.getElementById("sendButton");
const stopButton = document.getElementById("stopButton");
const composerStatus = document.getElementById("composerStatus");
const connectionSummary = document.getElementById("connectionSummary");
const attachButton = document.getElementById("attachButton");
const fileInput = document.getElementById("fileInput");
const attachmentSummary = document.getElementById("attachmentSummary");
const runStatus = document.getElementById("runStatus");
const nowCard = document.getElementById("nowCard");
const approvalList = document.getElementById("approvalList");
const attachmentList = document.getElementById("attachmentList");
const activityList = document.getElementById("activityList");
const providerDialog = document.getElementById("providerDialog");
const providerClose = document.getElementById("providerClose");
const providerSelect = document.getElementById("providerSelect");
const modelInput = document.getElementById("modelInput");
const apiKeyField = document.getElementById("apiKeyField");
const apiKeyInput = document.getElementById("apiKeyInput");
const urlField = document.getElementById("urlField");
const urlInput = document.getElementById("urlInput");
const providerMessage = document.getElementById("providerMessage");
const providerOAuth = document.getElementById("providerOAuth");
const providerTest = document.getElementById("providerTest");
const providerSave = document.getElementById("providerSave");

function loadStoredConfig() {
  try {
    const raw = localStorage.getItem("ricing-config");
    if (!raw) {
      return { provider: "", model: "", apiKey: "", url: "" };
    }
    return normalizeConfig(JSON.parse(raw));
  } catch {
    return { provider: "", model: "", apiKey: "", url: "" };
  }
}

function saveStoredConfig(config) {
  state.config = normalizeConfig(config);
  localStorage.setItem("ricing-config", JSON.stringify(state.config));
}

function normalizeConfig(config) {
  const provider = `${config?.provider || ""}`.trim();
  const defaults = providerDefaults[provider] || {};
  return {
    provider,
    model: `${config?.model || defaults.model || ""}`.trim(),
    apiKey: `${config?.apiKey || ""}`.trim(),
    url: `${config?.url || defaults.url || ""}`.trim()
  };
}

function currentConfig() {
  if (state.config.provider) {
    return normalizeConfig(state.config);
  }
  if (state.oauthConnected) {
    return normalizeConfig({ provider: "chatgpt" });
  }
  return normalizeConfig({});
}

function providerLabel(provider) {
  return providerDefaults[provider]?.label || provider || "provider";
}

function providerSummary(config = currentConfig()) {
  if (!config.provider) {
    return "choose provider";
  }
  return config.model ? `${providerLabel(config.provider)} / ${config.model}` : providerLabel(config.provider);
}

function providerStateText(config = currentConfig()) {
  if (!config.provider) {
    return "provider offline";
  }
  if (config.provider === "chatgpt") {
    return state.oauthConnected ? "provider linked" : "needs chatgpt login";
  }
  if (config.provider === "ollama" || config.provider === "lmstudio") {
    return config.url ? "local provider ready" : "needs base url";
  }
  return config.apiKey ? "credentials loaded" : "needs api key";
}

function hasReadyProvider(config = currentConfig()) {
  if (!config.provider) {
    return false;
  }
  if (config.provider === "chatgpt") {
    return state.oauthConnected;
  }
  if (config.provider === "ollama" || config.provider === "lmstudio") {
    return Boolean(config.url);
  }
  return Boolean(config.apiKey);
}

function activeProviderPayload() {
  const config = currentConfig();
  if (config.provider === "chatgpt") {
    return {
      provider_id: state.oauthProviderId || "",
      model: config.model || providerDefaults.chatgpt.model,
      api_key: "",
      url: ""
    };
  }
  return {
    provider_id: config.provider,
    model: config.model,
    api_key: config.apiKey,
    url: config.url
  };
}

function api(path, options = {}) {
  return fetch(path, options);
}

function wsURL() {
  return `${window.location.origin.replace(/^http/, "ws")}/api/v1/ws`;
}

async function refreshProviders() {
  try {
    const res = await api("/api/v1/providers");
    const data = await res.json();
    state.backendOnline = res.ok;
    state.oauthProviderId = data.default_provider_id || "";
    const provider = (data.providers || []).find((item) => item.id === state.oauthProviderId);
    state.oauthConnected = provider?.state === "connected";
  } catch {
    state.backendOnline = false;
    state.oauthConnected = false;
    state.oauthProviderId = "";
  }
  renderProviderState();
  renderVisibility();
}

function renderProviderState() {
  const config = currentConfig();
  backendStatus.className = "status-badge subtle";
  backendStatus.textContent = state.backendOnline ? "localhost online" : "backend offline";
  backendStatus.classList.add(state.backendOnline ? "connected" : "failed");
  providerAction.textContent = providerSummary(config);
  providerStatus.className = "status-badge";
  providerStatus.textContent = providerStateText(config);
  connectionSummary.textContent = providerSummary(config);
  if (hasReadyProvider(config)) {
    providerStatus.classList.add("connected");
  } else if (config.provider === "chatgpt") {
    providerStatus.classList.add("warn");
  } else if (config.provider) {
    providerStatus.classList.add("failed");
  }
}

function openProviderDialog() {
  const config = currentConfig();
  providerSelect.value = config.provider || "chatgpt";
  modelInput.value = config.model || providerDefaults[providerSelect.value]?.model || "";
  apiKeyInput.value = config.apiKey || "";
  urlInput.value = config.url || providerDefaults[providerSelect.value]?.url || "";
  updateProviderFields(false);
  providerDialog.classList.remove("is-hidden");
}

function closeProviderDialog() {
  providerDialog.classList.add("is-hidden");
}

function providerDraft() {
  return normalizeConfig({
    provider: providerSelect.value,
    model: modelInput.value,
    apiKey: apiKeyInput.value,
    url: urlInput.value
  });
}

function setProviderMessage(text) {
  providerMessage.textContent = text;
}

function updateProviderFields(fillDefaults = true) {
  const provider = providerSelect.value;
  const defaults = providerDefaults[provider] || {};
  if (fillDefaults) {
    modelInput.value = defaults.model || "";
    apiKeyInput.value = "";
    urlInput.value = defaults.url || "";
  }
  apiKeyField.classList.toggle("is-hidden", !defaults.apiKey);
  urlField.classList.toggle("is-hidden", !defaults.local);
  providerOAuth.classList.toggle("is-hidden", !defaults.oauth);
  providerTest.classList.toggle("is-hidden", Boolean(defaults.oauth));
  if (defaults.oauth) {
    setProviderMessage(state.oauthConnected ? "chatgpt is already linked on this machine." : "use the button below to complete chatgpt oauth in a new browser window.");
  } else if (defaults.local) {
    setProviderMessage("local providers use the base url below and do not need an api key.");
  } else {
    setProviderMessage("credentials stay in this browser and are sent to the shared backend per run.");
  }
}

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function connectOAuth() {
  if (!state.oauthProviderId) {
    setProviderMessage("the backend is offline or the oauth provider is unavailable.");
    return;
  }
  saveStoredConfig({ provider: "chatgpt", model: modelInput.value || providerDefaults.chatgpt.model });
  renderProviderState();
  providerOAuth.disabled = true;
  setProviderMessage("opening chatgpt oauth...");
  try {
    const connectRes = await api(`/api/v1/providers/${state.oauthProviderId}/connect`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ open_browser: "none" })
    });
    const data = await connectRes.json();
    if (!connectRes.ok) {
      setProviderMessage(data.error?.message || "could not start chatgpt oauth");
      return;
    }
    if (data.auth_url) {
      window.open(data.auth_url, "_blank", "width=620,height=760");
    }
    const started = Date.now();
    while (Date.now() - started < 180000) {
      await wait(2000);
      await refreshProviders();
      if (state.oauthConnected) {
        setProviderMessage("chatgpt linked");
        closeProviderDialog();
        renderAll();
        return;
      }
    }
    setProviderMessage("oauth timed out");
  } finally {
    providerOAuth.disabled = false;
    renderProviderState();
  }
}

async function testProvider() {
  const config = providerDraft();
  providerTest.disabled = true;
  setProviderMessage("testing provider...");
  try {
    const res = await api(`/api/v1/providers/${config.provider}/ping`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        model: config.model,
        api_key: config.apiKey,
        url: config.url
      })
    });
    const data = await res.json();
    if (!res.ok) {
      setProviderMessage(data.error?.message || "provider test failed");
      return;
    }
    setProviderMessage(data.reply || "provider responded");
  } catch (error) {
    setProviderMessage(error.message || "provider test failed");
  } finally {
    providerTest.disabled = false;
  }
}

async function saveProviderConfig() {
  const config = providerDraft();
  saveStoredConfig(config);
  renderProviderState();
  renderVisibility();
  if (config.provider === "chatgpt" && !state.oauthConnected) {
    setProviderMessage("model saved. complete oauth to use chatgpt.");
    return;
  }
  closeProviderDialog();
}

async function loadSessions() {
  if (!state.backendOnline) {
    state.sessions = [];
    renderSessions();
    return;
  }
  const res = await api("/api/v1/sessions");
  const data = await res.json();
  state.sessions = Array.isArray(data.sessions) ? data.sessions : [];
  renderSessions();
  if (!state.activeSessionId && state.sessions.length > 0) {
    await selectSession(state.sessions[0].id);
  } else if (state.activeSessionId) {
    const exists = state.sessions.find((session) => session.id === state.activeSessionId);
    if (!exists) {
      state.activeSessionId = "";
      state.activeSnapshot = null;
      state.localEntries = [];
      if (state.sessions.length > 0) {
        await selectSession(state.sessions[0].id);
      }
    }
  }
  renderVisibility();
}

async function createSession() {
  const payload = activeProviderPayload();
  const res = await api("/api/v1/sessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      mode: modeSelect.value || "auto",
      provider_id: payload.provider_id,
      model: payload.model
    })
  });
  const data = await res.json();
  if (!res.ok) {
    composerStatus.textContent = data.error?.message || "could not create session";
    return;
  }
  upsertSession(data.session);
  await selectSession(data.session.id);
}

async function selectSession(sessionId) {
  state.activeSessionId = sessionId;
  state.activeRunId = "";
  state.localEntries = [];
  const res = await api(`/api/v1/sessions/${sessionId}`);
  const data = await res.json();
  state.activeSnapshot = data;
  modeSelect.value = data.session?.mode || "auto";
  renderAll();
  openSessionSocket(sessionId);
}

function openCatalogSocket() {
  if (!state.backendOnline) {
    return;
  }
  if (state.catalogSocket) {
    state.catalogSocket.close();
  }
  const socket = new WebSocket(wsURL());
  socket.onopen = () => {
    socket.send(JSON.stringify({ type: "subscribe", data: { catalog: true, after_seq: 0 } }));
  };
  socket.onmessage = (event) => {
    const message = JSON.parse(event.data);
    if (message.type === "session.snapshot") {
      state.sessions = Array.isArray(message.data?.sessions) ? message.data.sessions : [];
      renderSessions();
      renderVisibility();
      return;
    }
    if (message.type === "session.updated") {
      upsertSession(message.data?.session);
      renderSessions();
    }
  };
  state.catalogSocket = socket;
}

function openSessionSocket(sessionId) {
  if (!state.backendOnline) {
    return;
  }
  if (state.sessionSocket) {
    state.sessionSocket.close();
  }
  const socket = new WebSocket(wsURL());
  socket.onopen = () => {
    socket.send(JSON.stringify({ type: "subscribe", data: { session_id: sessionId, after_seq: 0 } }));
  };
  socket.onmessage = (event) => {
    handleSessionMessage(JSON.parse(event.data));
  };
  state.sessionSocket = socket;
}

function handleSessionMessage(message) {
  if (!state.activeSnapshot) {
    return;
  }
  if (message.type === "session.snapshot") {
    state.activeSnapshot = message.data;
    modeSelect.value = state.activeSnapshot.session?.mode || "auto";
    renderAll();
    return;
  }
  if (message.type === "entry.created") {
    const entry = message.data?.entry;
    if (entry) {
      upsertEntry(entry);
      if (entry.run_id) {
        state.activeRunId = entry.run_id;
      }
    }
    renderAll();
    return;
  }
  if (message.type === "entry.updated") {
    const entry = message.data?.entry;
    if (entry) {
      upsertEntry(entry);
    }
    renderAll();
    return;
  }
  if (message.type === "entry.delta") {
    const entry = message.data?.entry;
    if (entry) {
      upsertEntry(entry);
    } else {
      appendEntryDelta(message.data?.entry_id, message.data?.text || "");
    }
    renderTranscript();
    renderNowCard();
    return;
  }
  if (message.type === "approval.updated") {
    const approval = message.data?.approval;
    if (approval) {
      upsertApproval(approval);
    }
    renderAll();
    return;
  }
  if (message.type === "session.updated" && message.data?.session) {
    state.activeSnapshot.session = { ...state.activeSnapshot.session, ...message.data.session };
    if (!["running", "queued"].includes(message.data.session.status || "")) {
      state.activeRunId = "";
    }
    upsertSession(message.data.session);
    renderAll();
    return;
  }
  if (message.type === "session.updated" && message.data?.attachment) {
    upsertAttachment(message.data.attachment);
    renderAttachments();
  }
}

function upsertSession(session) {
  if (!session?.id) {
    return;
  }
  const index = state.sessions.findIndex((item) => item.id === session.id);
  if (index === -1) {
    state.sessions.unshift(session);
  } else {
    state.sessions[index] = { ...state.sessions[index], ...session };
  }
  state.sessions.sort((left, right) => `${right.updated_at || ""}`.localeCompare(`${left.updated_at || ""}`));
}

function removeMatchedLocalEntry(entry) {
  if (entry.kind !== "user_message") {
    return;
  }
  const index = state.localEntries.findIndex((item) => item.kind === "user_message" && item.content === entry.content);
  if (index !== -1) {
    state.localEntries.splice(index, 1);
  }
}

function upsertEntry(entry) {
  if (!state.activeSnapshot || !entry?.id) {
    return;
  }
  removeMatchedLocalEntry(entry);
  const entries = state.activeSnapshot.entries || [];
  const index = entries.findIndex((item) => item.id === entry.id);
  if (index === -1) {
    entries.push(entry);
  } else {
    entries[index] = { ...entries[index], ...entry };
  }
  entries.sort((left, right) => (left.seq || 0) - (right.seq || 0));
  state.activeSnapshot.entries = entries;
  if (state.activeSnapshot.session) {
    state.activeSnapshot.session.latest_entry_id = entry.id;
    if (entry.content) {
      state.activeSnapshot.session.latest_preview = entry.content;
    }
  }
}

function appendEntryDelta(entryId, text) {
  if (!state.activeSnapshot || !entryId) {
    return;
  }
  const entry = (state.activeSnapshot.entries || []).find((item) => item.id === entryId);
  if (entry) {
    entry.content = `${entry.content || ""}${text}`;
  }
}

function upsertApproval(approval) {
  if (!state.activeSnapshot || !approval?.id) {
    return;
  }
  const approvals = state.activeSnapshot.approvals || [];
  const index = approvals.findIndex((item) => item.id === approval.id);
  if (index === -1) {
    approvals.push(approval);
  } else {
    approvals[index] = { ...approvals[index], ...approval };
  }
  state.activeSnapshot.approvals = approvals;
}

function upsertAttachment(attachment) {
  if (!state.activeSnapshot || !attachment?.id) {
    return;
  }
  const attachments = state.activeSnapshot.attachments || [];
  const index = attachments.findIndex((item) => item.id === attachment.id);
  if (index === -1) {
    attachments.push(attachment);
  } else {
    attachments[index] = { ...attachments[index], ...attachment };
  }
  state.activeSnapshot.attachments = attachments;
}

function addLocalEntry(entry) {
  state.localEntries.push(entry);
}

function updateLocalEntry(entryId, patch) {
  const entry = state.localEntries.find((item) => item.id === entryId);
  if (!entry) {
    return;
  }
  Object.assign(entry, patch, { updated_at: new Date().toISOString() });
}

function renderSessions() {
  const search = state.search.trim().toLowerCase();
  const filtered = state.sessions.filter((session) => {
    if (!search) {
      return true;
    }
    const haystack = `${session.title || ""} ${session.latest_preview || ""}`.toLowerCase();
    return haystack.includes(search);
  });
  sessionList.innerHTML = filtered.map((session) => {
    const active = session.id === state.activeSessionId ? " active" : "";
    const tags = [
      `<span class="tag">${escapeHTML(session.mode || "auto")}</span>`,
      `<span class="tag">${escapeHTML(session.status || "idle")}</span>`
    ];
    if (session.pending_approvals) {
      tags.push(`<span class="tag">${session.pending_approvals} approvals</span>`);
    }
    return `<button class="session-item${active}" data-session-id="${escapeHTML(session.id)}"><div class="session-item-head"><span class="session-item-title">${escapeHTML(session.title || "new session")}</span><span class="entry-meta">${escapeHTML(formatRelative(session.updated_at))}</span></div><p class="session-item-preview">${escapeHTML(session.latest_preview || "no activity yet")}</p><div class="session-item-tags">${tags.join("")}</div></button>`;
  }).join("");
  sessionList.querySelectorAll("[data-session-id]").forEach((button) => {
    button.addEventListener("click", () => selectSession(button.dataset.sessionId));
  });
}

function renderVisibility() {
  const hasSession = Boolean(state.activeSnapshot?.session?.id);
  const ready = hasReadyProvider();
  connectPanel.classList.toggle("visible", !ready && !hasSession);
  emptyPanel.classList.toggle("visible", ready && !hasSession);
  transcript.classList.toggle("is-hidden", !hasSession);
  composer.classList.toggle("is-hidden", !hasSession);
}

function renderBanner() {
  const session = state.activeSnapshot?.session;
  if (!session) {
    sessionTitle.textContent = "select a session";
    sessionModeLabel.textContent = modeSelect.value || "auto";
    sessionState.textContent = "idle";
    sessionState.className = "status-badge subtle";
    runStatus.textContent = "idle";
    runStatus.className = "status-badge subtle";
    return;
  }
  sessionTitle.textContent = session.title || "new session";
  sessionModeLabel.textContent = session.mode || "auto";
  sessionState.textContent = session.status || "idle";
  sessionState.className = "status-badge subtle";
  runStatus.textContent = session.status || "idle";
  runStatus.className = "status-badge subtle";
  if (session.status === "running" || session.status === "queued") {
    sessionState.classList.add("running");
    runStatus.classList.add("running");
  } else if (session.status === "failed") {
    sessionState.classList.add("failed");
    runStatus.classList.add("failed");
  } else if (session.status === "cancelled") {
    sessionState.classList.add("warn");
    runStatus.classList.add("warn");
  } else {
    sessionState.classList.add("connected");
    runStatus.classList.add("connected");
  }
}

function transcriptEntries() {
  const sessionEntries = (state.activeSnapshot?.entries || []).filter((entry) => ["user_message", "assistant_message", "plan", "approval", "system"].includes(entry.kind));
  return sessionEntries.concat(state.localEntries).sort((left, right) => {
    if (typeof left.seq === "number" && typeof right.seq === "number") {
      return left.seq - right.seq;
    }
    if (typeof left.seq === "number") {
      return -1;
    }
    if (typeof right.seq === "number") {
      return 1;
    }
    return new Date(left.created_at || 0).getTime() - new Date(right.created_at || 0).getTime();
  });
}

function renderTranscript() {
  const entries = transcriptEntries();
  transcript.innerHTML = entries.map(renderEntryCard).join("");
  transcript.querySelectorAll("[data-approval-action]").forEach((button) => {
    button.addEventListener("click", () => resolveApproval(button.dataset.approvalId, button.dataset.approvalAction));
  });
  transcript.scrollTop = transcript.scrollHeight;
}

function entryLabel(entry) {
  switch (entry.kind) {
    case "user_message":
      return "you";
    case "assistant_message":
      return entry.status === "streaming" ? "assistant streaming" : "assistant";
    case "plan":
      return "plan";
    case "approval":
      return `approval ${entry.status || ""}`.trim();
    case "system":
      return entry.title || "system";
    default:
      return entry.title || entry.kind;
  }
}

function entryMeta(entry) {
  if (["sending", "queued", "failed"].includes(entry.status || "")) {
    return entry.status;
  }
  return formatRelative(entry.updated_at || entry.created_at);
}

function renderEntryCard(entry) {
  const status = entry.status ? ` ${escapeHTML(entry.status)}` : "";
  let actions = "";
  if (entry.kind === "approval" && entry.status === "pending") {
    const approvalId = entry.meta?.approval_id;
    if (approvalId) {
      actions = `<div class="approval-actions"><button class="ghost-button" data-approval-action="reject" data-approval-id="${escapeHTML(approvalId)}">reject</button><button class="primary-button" data-approval-action="approve" data-approval-id="${escapeHTML(approvalId)}">approve</button></div>`;
    }
  }
  return `<article class="entry-card ${escapeHTML(entry.kind)}${status}"><div class="entry-head"><span class="entry-title">${escapeHTML(entryLabel(entry))}</span><span class="entry-meta">${escapeHTML(entryMeta(entry))}</span></div><div class="entry-content">${escapeHTML(entry.content || "")}</div>${actions}</article>`;
}

function renderApprovals() {
  const approvals = (state.activeSnapshot?.approvals || []).filter((item) => item.status === "pending");
  approvalList.innerHTML = approvals.length === 0 ? `<div class="info-card">no pending approvals</div>` : approvals.map((approval) => {
    return `<div class="approval-card ${escapeHTML(approval.status)}"><div class="approval-title">${escapeHTML(approval.tool_name)}</div><div class="approval-content">${escapeHTML(approval.summary || "")}</div><div class="approval-actions"><button class="ghost-button" data-approval-action="reject" data-approval-id="${escapeHTML(approval.id)}">reject</button><button class="primary-button" data-approval-action="approve" data-approval-id="${escapeHTML(approval.id)}">approve</button></div></div>`;
  }).join("");
  approvalList.querySelectorAll("[data-approval-action]").forEach((button) => {
    button.addEventListener("click", () => resolveApproval(button.dataset.approvalId, button.dataset.approvalAction));
  });
}

function renderAttachments() {
  const attachments = state.activeSnapshot?.attachments || [];
  attachmentSummary.textContent = attachments.length === 0 ? "no files attached" : `${attachments.length} file${attachments.length === 1 ? "" : "s"} attached`;
  attachmentList.innerHTML = attachments.length === 0 ? `<div class="info-card">drop files into the current session to give the agent more context</div>` : attachments.map((attachment) => {
    return `<div class="attachment-item"><div class="attachment-item-title">${escapeHTML(attachment.name)}</div><div class="attachment-item-meta">${escapeHTML(formatFileSize(attachment.size))}</div></div>`;
  }).join("");
}

function renderActivity() {
  const entries = (state.activeSnapshot?.entries || []).filter((entry) => ["tool_call", "tool_result", "change", "verification"].includes(entry.kind));
  activityList.innerHTML = entries.length === 0 ? `<div class="info-card">tool calls, changes, and verification results will show up here</div>` : entries.map((entry) => {
    return `<div class="activity-card"><div class="activity-card-head"><div class="activity-card-title">${escapeHTML(entry.title || entry.kind)}</div><div class="entry-meta">${escapeHTML(entry.kind)}</div></div><div class="activity-card-content">${escapeHTML(entry.content || "")}</div></div>`;
  }).join("");
}

function renderNowCard() {
  const session = state.activeSnapshot?.session;
  const entries = transcriptEntries().concat((state.activeSnapshot?.entries || []).filter((entry) => ["tool_call", "tool_result", "change", "verification"].includes(entry.kind)));
  const lastEntry = [...entries].reverse().find((entry) => ["assistant_message", "tool_call", "change", "verification", "plan", "system", "user_message"].includes(entry.kind));
  const pendingLocal = [...state.localEntries].reverse().find((entry) => entry.kind === "user_message" && ["sending", "queued"].includes(entry.status || ""));
  stopButton.classList.toggle("is-hidden", !state.activeRunId);
  if (!session) {
    nowCard.textContent = "waiting for the next step";
    composerStatus.textContent = hasReadyProvider() ? "ready" : "choose a provider";
    return;
  }
  if (pendingLocal) {
    nowCard.textContent = pendingLocal.content || "waiting for the backend";
    composerStatus.textContent = pendingLocal.status;
    return;
  }
  if (session.status === "running" || session.status === "queued") {
    nowCard.textContent = lastEntry?.content || `${session.status}...`;
    composerStatus.textContent = session.status === "queued" ? "queued" : "run in progress";
    return;
  }
  if (session.status === "failed") {
    nowCard.textContent = lastEntry?.content || "the last run failed";
    composerStatus.textContent = "last run failed";
    return;
  }
  nowCard.textContent = lastEntry?.content || "idle";
  if (!hasReadyProvider()) {
    composerStatus.textContent = "choose a provider";
  } else if (!sendButton.disabled) {
    composerStatus.textContent = "ready";
  }
}

function renderAll() {
  renderProviderState();
  renderVisibility();
  renderBanner();
  renderSessions();
  renderTranscript();
  renderApprovals();
  renderAttachments();
  renderActivity();
  renderNowCard();
}

async function sendPrompt() {
  const prompt = promptInput.value.trim();
  if (!prompt) {
    return;
  }
  if (!hasReadyProvider()) {
    composerStatus.textContent = "choose a provider first";
    openProviderDialog();
    return;
  }
  if (!state.activeSessionId) {
    await createSession();
  }
  if (!state.activeSessionId) {
    return;
  }
  const payload = activeProviderPayload();
  const now = new Date().toISOString();
  const localId = `local_${Date.now()}`;
  addLocalEntry({
    id: localId,
    kind: "user_message",
    status: "sending",
    content: prompt,
    created_at: now,
    updated_at: now
  });
  if (state.activeSnapshot?.session) {
    state.activeSnapshot.session.provider_id = payload.provider_id;
    state.activeSnapshot.session.model = payload.model;
  }
  promptInput.value = "";
  sendButton.disabled = true;
  composerStatus.textContent = "sending";
  renderTranscript();
  renderNowCard();
  try {
    const res = await api(`/api/v1/sessions/${state.activeSessionId}/messages`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        prompt,
        mode: modeSelect.value,
        llm: payload
      })
    });
    const data = await res.json();
    if (!res.ok) {
      updateLocalEntry(localId, { status: "failed" });
      addLocalEntry({
        id: `local_error_${Date.now()}`,
        kind: "system",
        status: "failed",
        title: "send failed",
        content: data.error?.message || "send failed",
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString()
      });
      composerStatus.textContent = data.error?.message || "send failed";
      if (data.error?.code === "message_create_failed") {
        openProviderDialog();
      }
      renderAll();
      return;
    }
    updateLocalEntry(localId, { status: "queued" });
    state.activeRunId = data.run?.id || "";
    if (data.session) {
      upsertSession(data.session);
      if (state.activeSnapshot?.session) {
        state.activeSnapshot.session = { ...state.activeSnapshot.session, ...data.session };
      }
    }
    composerStatus.textContent = "queued";
    renderAll();
  } catch (error) {
    updateLocalEntry(localId, { status: "failed" });
    addLocalEntry({
      id: `local_error_${Date.now()}`,
      kind: "system",
      status: "failed",
      title: "send failed",
      content: error.message || "send failed",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString()
    });
    composerStatus.textContent = error.message || "send failed";
    renderAll();
  } finally {
    sendButton.disabled = false;
  }
}

async function setMode() {
  if (!state.activeSessionId) {
    return;
  }
  const res = await api(`/api/v1/sessions/${state.activeSessionId}/mode`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mode: modeSelect.value })
  });
  const data = await res.json();
  if (res.ok) {
    upsertSession(data.session);
    if (state.activeSnapshot?.session) {
      state.activeSnapshot.session.mode = data.session.mode;
    }
    renderBanner();
    renderSessions();
  }
}

async function resolveApproval(approvalId, decision) {
  if (!approvalId) {
    return;
  }
  await api(`/api/v1/approvals/${approvalId}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ decision })
  });
}

async function uploadFiles(files) {
  if (!state.activeSessionId || files.length === 0) {
    return;
  }
  for (const file of files) {
    const form = new FormData();
    form.append("file", file);
    await api(`/api/v1/sessions/${state.activeSessionId}/attachments`, {
      method: "POST",
      body: form
    });
  }
}

async function stopRun() {
  if (!state.activeRunId) {
    return;
  }
  composerStatus.textContent = "stopping";
  await api(`/api/v1/runs/${state.activeRunId}/cancel`, { method: "POST" });
}

function formatRelative(input) {
  if (!input) {
    return "";
  }
  const delta = Math.max(0, Date.now() - new Date(input).getTime());
  const minutes = Math.floor(delta / 60000);
  if (minutes < 1) {
    return "now";
  }
  if (minutes < 60) {
    return `${minutes}m`;
  }
  const hours = Math.floor(minutes / 60);
  if (hours < 24) {
    return `${hours}h`;
  }
  return `${Math.floor(hours / 24)}d`;
}

function formatFileSize(size) {
  if (size < 1024) {
    return `${size} B`;
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function escapeHTML(value) {
  return `${value ?? ""}`
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

providerAction.addEventListener("click", openProviderDialog);
connectButton.addEventListener("click", openProviderDialog);
providerClose.addEventListener("click", closeProviderDialog);
providerSelect.addEventListener("change", () => updateProviderFields(true));
providerSave.addEventListener("click", saveProviderConfig);
providerTest.addEventListener("click", testProvider);
providerOAuth.addEventListener("click", connectOAuth);
newSessionButton.addEventListener("click", createSession);
modeSelect.addEventListener("change", setMode);
sendButton.addEventListener("click", sendPrompt);
stopButton.addEventListener("click", stopRun);
attachButton.addEventListener("click", () => fileInput.click());
fileInput.addEventListener("change", async () => {
  await uploadFiles(Array.from(fileInput.files || []));
  fileInput.value = "";
});
sessionSearch.addEventListener("input", () => {
  state.search = sessionSearch.value || "";
  renderSessions();
});
promptInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();
    sendPrompt();
  }
});
providerDialog.addEventListener("click", (event) => {
  if (event.target === providerDialog) {
    closeProviderDialog();
  }
});

async function init() {
  renderProviderState();
  renderVisibility();
  renderAll();
  await refreshProviders();
  await loadSessions();
  openCatalogSocket();
  renderAll();
}

init();
