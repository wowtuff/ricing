const providerDefaults = {
  chatgpt: { label: "chatgpt", model: "gpt-5.2-codex", oauth: true },
  openai: { label: "openai", model: "gpt-4o", apiKey: true },
  openrouter: { label: "openrouter", model: "openai/gpt-4o", apiKey: true },
  anthropic: { label: "anthropic", model: "claude-sonnet-4-5", apiKey: true },
  gemini: { label: "gemini", model: "gemini-2.0-flash", apiKey: true },
  ollama: { label: "ollama", model: "", url: "http://localhost:11434/v1", local: true },
  lmstudio: { label: "lm studio", model: "", url: "http://localhost:1234/v1", local: true }
};

const defaultBackendOrigin = "http://127.0.0.1:1777";
const pageOrigin = window.location.origin && window.location.origin !== "null" ? window.location.origin : "";

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
  backendOrigin: pageOrigin || defaultBackendOrigin,
  config: loadStoredConfig(),
  localEntries: [],
  editingEntryId: "",
  expandedActivityGroups: {}
};

const backendStatus = document.getElementById("backendStatus");
const providerAction = document.getElementById("providerAction");
const providerStatus = document.getElementById("providerStatus");
const newSessionButton = document.getElementById("newSessionButton");
const sessionRailToggle = document.getElementById("sessionRailToggle");
const sessionRailBackdrop = document.getElementById("sessionRailBackdrop");
const sessionSearch = document.getElementById("sessionSearch");
const sessionList = document.getElementById("sessionList");
const sessionTitle = document.getElementById("sessionTitle");
const sessionModeLabel = document.getElementById("sessionModeLabel");
const sessionState = document.getElementById("sessionState");
const deleteSessionButton = document.getElementById("deleteSessionButton");
const thinkingSelect = document.getElementById("thinkingSelect");
const modeSelect = document.getElementById("modeSelect");
const connectPanel = document.getElementById("connectPanel");
const connectButton = document.getElementById("connectButton");
const emptyPanel = document.getElementById("emptyPanel");
const transcript = document.getElementById("transcript");
const composer = document.getElementById("composer");
const promptInput = document.getElementById("promptInput");
const sendButton = document.getElementById("sendButton");
const cancelEditButton = document.getElementById("cancelEditButton");
const stopButton = document.getElementById("stopButton");
const composerStatus = document.getElementById("composerStatus");
const connectionSummary = document.getElementById("connectionSummary");
const attachButton = document.getElementById("attachButton");
const fileInput = document.getElementById("fileInput");
const attachmentSummary = document.getElementById("attachmentSummary");
const composerAttachments = document.getElementById("composerAttachments");
const attachmentPreviewDialog = document.getElementById("attachmentPreviewDialog");
const attachmentPreviewImage = document.getElementById("attachmentPreviewImage");
const attachmentPreviewName = document.getElementById("attachmentPreviewName");
const attachmentPreviewClose = document.getElementById("attachmentPreviewClose");
const attachmentPreviewDownload = document.getElementById("attachmentPreviewDownload");
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

thinkingSelect.value = normalizeThinkingValue(state.config.thinking);

function loadStoredConfig() {
  try {
    const raw = localStorage.getItem("ricing-config");
    if (!raw) {
      return { provider: "", model: "", thinking: "", apiKey: "", url: "" };
    }
    return normalizeConfig(JSON.parse(raw));
  } catch {
    return { provider: "", model: "", thinking: "", apiKey: "", url: "" };
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
    thinking: normalizeThinkingValue(config?.thinking),
    apiKey: `${config?.apiKey || ""}`.trim(),
    url: `${config?.url || defaults.url || ""}`.trim()
  };
}

function currentConfig() {
  const stored = normalizeConfig(state.config);
  if (stored.provider) {
    return stored;
  }
  if (state.oauthConnected) {
    return normalizeConfig({ ...stored, provider: "chatgpt" });
  }
  return stored;
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

function normalizeThinkingValue(value) {
  const next = `${value || ""}`.trim().toLowerCase();
  if (["", "auto", "default"].includes(next)) {
    return "";
  }
  if (["low", "medium", "high"].includes(next)) {
    return next;
  }
  return "";
}

function thinkingLabel(value) {
  const next = normalizeThinkingValue(value);
  return next ? `think ${next}` : "";
}

function modeLabel(value) {
  switch (`${value || ""}`.trim().toLowerCase()) {
    case "full":
      return "full permission";
    case "build":
      return "build";
    case "plan":
      return "plan";
    default:
      return "auto";
  }
}

function currentThinkingValue() {
  if (thinkingSelect) {
    return normalizeThinkingValue(thinkingSelect.value);
  }
  if (state.activeSnapshot?.session) {
    return normalizeThinkingValue(state.activeSnapshot.session.thinking);
  }
  return normalizeThinkingValue(currentConfig().thinking);
}

function providerRunSummary(config = currentConfig(), thinking = currentThinkingValue()) {
  const summary = providerSummary(config);
  if (!config.provider) {
    return summary;
  }
  const label = thinkingLabel(thinking);
  return label ? `${summary} / ${label}` : summary;
}

function openSessionRail() {
  document.body.classList.add("session-rail-open");
  sessionRailBackdrop?.classList.remove("is-hidden");
}

function closeSessionRail() {
  document.body.classList.remove("session-rail-open");
  sessionRailBackdrop?.classList.add("is-hidden");
}

function toggleSessionRail() {
  if (document.body.classList.contains("session-rail-open")) {
    closeSessionRail();
    return;
  }
  openSessionRail();
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
  const reasoningEffort = currentThinkingValue();
  if (config.provider === "chatgpt") {
    return {
      provider_id: state.oauthProviderId || "",
      model: config.model || providerDefaults.chatgpt.model,
      reasoning_effort: reasoningEffort,
      api_key: "",
      url: ""
    };
  }
  return {
    provider_id: config.provider,
    model: config.model,
    reasoning_effort: reasoningEffort,
    api_key: config.apiKey,
    url: config.url
  };
}

function backendOrigin() {
  return state.backendOrigin || defaultBackendOrigin;
}

function apiURL(path) {
  return new URL(path, backendOrigin()).toString();
}

async function detectBackendOrigin() {
  const candidates = [];
  if (pageOrigin) {
    candidates.push(pageOrigin);
  }
  if (!candidates.includes(defaultBackendOrigin)) {
    candidates.push(defaultBackendOrigin);
  }

  for (const origin of candidates) {
    try {
      const res = await fetch(new URL("/api/v1/health", origin).toString());
      if (res.ok) {
        state.backendOrigin = origin;
        return;
      }
    } catch {
      // Try the next likely backend origin.
    }
  }
}

function api(path, options = {}) {
  return fetch(apiURL(path), options);
}

function wsURL() {
  return `${backendOrigin().replace(/^http/, "ws")}/api/v1/ws`;
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
  providerAction.textContent = providerRunSummary(config);
  providerStatus.className = "status-badge";
  providerStatus.textContent = providerStateText(config);
  connectionSummary.textContent = providerRunSummary(config);
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
    thinking: currentThinkingValue(),
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
  saveStoredConfig({
    ...currentConfig(),
    provider: "chatgpt",
    model: modelInput.value || providerDefaults.chatgpt.model,
    thinking: currentThinkingValue()
  });
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
        reasoning_effort: config.thinking,
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
      thinking: currentThinkingValue(),
      provider_id: payload.provider_id,
      model: payload.model
    })
  });
  const data = await res.json();
  if (!res.ok) {
    composerStatus.textContent = data.error?.message || "could not create session";
    return null;
  }
  upsertSession(data.session);
  await selectSession(data.session.id);
  closeSessionRail();
  return data.session;
}

async function selectSession(sessionId) {
  if (
    sessionId &&
    state.activeSessionId === sessionId &&
    state.activeSnapshot?.session?.id === sessionId &&
    state.sessionSocket &&
    [WebSocket.OPEN, WebSocket.CONNECTING].includes(state.sessionSocket.readyState)
  ) {
    return;
  }
  state.activeSessionId = sessionId;
  state.activeRunId = "";
  state.localEntries = [];
  state.editingEntryId = "";
  state.expandedActivityGroups = {};
  const res = await api(`/api/v1/sessions/${sessionId}`);
  const data = await res.json();
  state.activeSnapshot = data;
  thinkingSelect.value = normalizeThinkingValue(data.session?.thinking || currentConfig().thinking);
  modeSelect.value = data.session?.mode || "auto";
  renderAll();
  closeSessionRail();
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
      return;
    }
    if (message.type === "session.deleted") {
      removeSessionLocal(message.data?.session_id);
      renderAll();
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
    thinkingSelect.value = normalizeThinkingValue(state.activeSnapshot.session?.thinking || currentConfig().thinking);
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
    renderComposerState();
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
    thinkingSelect.value = normalizeThinkingValue(state.activeSnapshot.session?.thinking || currentConfig().thinking);
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
    return;
  }
  if (message.type === "session.updated" && message.data?.attachment_removed_id) {
    removeAttachmentLocal(message.data.attachment_removed_id);
    renderAttachments();
    return;
  }
  if (message.type === "session.deleted") {
    removeSessionLocal(message.data?.session_id);
    renderAll();
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

function removeSessionLocal(sessionId) {
  if (!sessionId) {
    return;
  }
  state.sessions = state.sessions.filter((session) => session.id !== sessionId);
  if (state.activeSessionId !== sessionId) {
    return;
  }
  if (state.sessionSocket) {
    state.sessionSocket.close();
    state.sessionSocket = null;
  }
  state.activeSessionId = "";
  state.activeSnapshot = null;
  state.activeRunId = "";
  state.localEntries = [];
  state.editingEntryId = "";
  state.expandedActivityGroups = {};
  thinkingSelect.value = normalizeThinkingValue(currentConfig().thinking);
  closeSessionRail();
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
  if (state.activeSnapshot.session) {
    state.activeSnapshot.session.attachment_count = attachments.length;
  }
}

function removeAttachmentLocal(attachmentId) {
  if (!state.activeSnapshot || !attachmentId) {
    return;
  }
  const attachments = state.activeSnapshot.attachments || [];
  const next = attachments.filter((item) => item.id !== attachmentId);
  state.activeSnapshot.attachments = next;
  if (state.activeSnapshot.session) {
    state.activeSnapshot.session.attachment_count = next.length;
  }
  const previewSrc = attachmentPreviewImage?.dataset.attachmentId || "";
  if (previewSrc === attachmentId) {
    closeAttachmentPreview();
  }
}

function isImageAttachment(attachment) {
  return /\.(png|jpe?g|gif|webp|bmp|ico|svg)$/i.test(attachment?.name || "");
}

function attachmentTypeLabel(attachment) {
  return (attachment?.name?.split(".").pop() || "file").toUpperCase();
}

function attachmentContentURL(attachment, download = false) {
  const url = new URL(apiURL(`/api/v1/sessions/${attachment.session_id}/attachments/${attachment.id}/content`));
  if (download) {
    url.searchParams.set("download", "1");
  }
  return url.toString();
}

function openAttachmentPreview(attachmentId) {
  const attachment = (state.activeSnapshot?.attachments || []).find((item) => item.id === attachmentId);
  if (!attachment || !isImageAttachment(attachment) || !attachmentPreviewDialog) {
    return;
  }
  attachmentPreviewName.textContent = attachment.name;
  attachmentPreviewImage.src = attachmentContentURL(attachment);
  attachmentPreviewImage.alt = attachment.name;
  attachmentPreviewImage.dataset.attachmentId = attachment.id;
  attachmentPreviewDownload.href = attachmentContentURL(attachment, true);
  attachmentPreviewDownload.download = attachment.name;
  attachmentPreviewDialog.classList.remove("is-hidden");
}

function closeAttachmentPreview() {
  if (!attachmentPreviewDialog) {
    return;
  }
  attachmentPreviewDialog.classList.add("is-hidden");
  attachmentPreviewImage.removeAttribute("src");
  attachmentPreviewImage.removeAttribute("alt");
  delete attachmentPreviewImage.dataset.attachmentId;
  attachmentPreviewDownload.removeAttribute("href");
}

async function deleteAttachment(attachmentId) {
  const attachment = (state.activeSnapshot?.attachments || []).find((item) => item.id === attachmentId);
  if (!attachment) {
    return;
  }
  composerStatus.textContent = `removing ${attachment.name}...`;
  try {
    const res = await api(`/api/v1/sessions/${attachment.session_id}/attachments/${attachment.id}`, {
      method: "DELETE"
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error?.message || `could not remove ${attachment.name}`);
    }
    removeAttachmentLocal(attachment.id);
    renderAttachments();
    composerStatus.textContent = "attachment removed";
  } catch (error) {
    composerStatus.textContent = error.message || `could not remove ${attachment.name}`;
  }
}

function currentEditingEntry() {
  if (!state.editingEntryId) {
    return null;
  }
  return (state.activeSnapshot?.entries || []).find((entry) => entry.id === state.editingEntryId) || null;
}

function beginEditMessage(entryId) {
  const entry = (state.activeSnapshot?.entries || []).find((item) => item.id === entryId && item.kind === "user_message");
  if (!entry) {
    return;
  }
  state.editingEntryId = entry.id;
  promptInput.value = entry.content || "";
  promptInput.focus();
  promptInput.setSelectionRange(promptInput.value.length, promptInput.value.length);
  renderAll();
}

function cancelEditMessage() {
  state.editingEntryId = "";
  promptInput.value = "";
  renderAll();
}

async function answerQuestion(questionId, optionId) {
  if (!state.activeSessionId || !questionId || !optionId) {
    return;
  }
  composerStatus.textContent = "sending answer";
  try {
    const res = await api(`/api/v1/sessions/${state.activeSessionId}/questions/${questionId}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ option_id: optionId })
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error?.message || "could not answer question");
    }
    if (data.entry) {
      upsertEntry(data.entry);
    }
    composerStatus.textContent = "answer recorded";
    renderAll();
  } catch (error) {
    composerStatus.textContent = error.message || "could not answer question";
  }
}

async function deleteSession(sessionId = state.activeSessionId) {
  if (!sessionId) {
    return;
  }
  const session = state.sessions.find((item) => item.id === sessionId) || state.activeSnapshot?.session;
  if (!window.confirm(`Delete "${session?.title || "this session"}"?`)) {
    return;
  }
  composerStatus.textContent = "deleting session";
  try {
    const res = await api(`/api/v1/sessions/${sessionId}`, {
      method: "DELETE"
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error?.message || "could not delete session");
    }
    removeSessionLocal(sessionId);
    renderAll();
    if (!state.activeSessionId && state.sessions.length > 0) {
      await selectSession(state.sessions[0].id);
    }
    composerStatus.textContent = "session deleted";
  } catch (error) {
    composerStatus.textContent = error.message || "could not delete session";
  }
}

function fileExtensionForType(type) {
  switch ((type || "").toLowerCase()) {
    case "image/jpeg":
      return "jpg";
    case "image/gif":
      return "gif";
    case "image/webp":
      return "webp";
    case "image/bmp":
      return "bmp";
    case "image/svg+xml":
      return "svg";
    default:
      return "png";
  }
}

function clipboardImageFiles(event) {
  const items = Array.from(event.clipboardData?.items || []).filter((item) => item.type?.startsWith("image/"));
  return items.map((item, index) => {
    const file = item.getAsFile();
    if (!file) {
      return null;
    }
    if (file.name) {
      return file;
    }
    return new File([file], `clipboard-${Date.now()}-${index + 1}.${fileExtensionForType(file.type)}`, {
      type: file.type || "image/png"
    });
  }).filter(Boolean);
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

function applySessionStatusBadge(element, status) {
  const value = `${status || ""}`.trim();
  element.className = "status-badge subtle";
  if (!value || value === "idle") {
    element.textContent = "";
    element.classList.add("is-hidden");
    return;
  }
  element.classList.remove("is-hidden");
  element.textContent = value;
  if (value === "running" || value === "queued") {
    element.classList.add("running");
  } else if (value === "failed") {
    element.classList.add("failed");
  } else if (value === "cancelled") {
    element.classList.add("warn");
  } else {
    element.classList.add("connected");
  }
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
    const tags = [`<span class="tag">${escapeHTML(modeLabel(session.mode))}</span>`];
    if (session.thinking) {
      tags.push(`<span class="tag">${escapeHTML(thinkingLabel(session.thinking))}</span>`);
    }
    if (session.status && session.status !== "idle") {
      tags.push(`<span class="tag">${escapeHTML(session.status)}</span>`);
    }
    if (session.pending_approvals) {
      tags.push(`<span class="tag">${session.pending_approvals} approvals</span>`);
    }
    return `<div class="session-item-shell${active}"><button class="session-item${active}" data-session-id="${escapeHTML(session.id)}"><div class="session-item-head"><span class="session-item-title">${escapeHTML(session.title || "new session")}</span><span class="entry-meta">${escapeHTML(formatRelative(session.updated_at))}</span></div><p class="session-item-preview">${escapeHTML(session.latest_preview || "no activity yet")}</p><div class="session-item-tags">${tags.join("")}</div></button><button class="session-delete-button" data-delete-session="${escapeHTML(session.id)}" aria-label="delete ${escapeHTML(session.title || "session")}">delete</button></div>`;
  }).join("");
  sessionList.querySelectorAll("[data-session-id]").forEach((button) => {
    button.addEventListener("click", () => selectSession(button.dataset.sessionId));
  });
  sessionList.querySelectorAll("[data-delete-session]").forEach((button) => {
    button.addEventListener("click", (event) => {
      event.stopPropagation();
      deleteSession(button.dataset.deleteSession);
    });
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

function bannerModeText(mode, thinking) {
  const label = thinkingLabel(thinking);
  const base = modeLabel(mode);
  return label ? `${base} / ${label}` : base;
}

function renderBanner() {
  const session = state.activeSnapshot?.session;
  if (!session) {
    sessionTitle.textContent = "select a session";
    sessionModeLabel.textContent = bannerModeText(modeSelect.value, currentThinkingValue());
    applySessionStatusBadge(sessionState, "");
    deleteSessionButton.classList.add("is-hidden");
    return;
  }
  sessionTitle.textContent = session.title || "new session";
  sessionModeLabel.textContent = bannerModeText(session.mode, session.thinking);
  applySessionStatusBadge(sessionState, session.status);
  deleteSessionButton.classList.remove("is-hidden");
  deleteSessionButton.disabled = ["running", "queued"].includes(session.status || "");
}

function currentApprovalStatus(entry) {
  const approvalId = `${entry.meta?.approval_id || ""}`.trim();
  if (!approvalId) {
    return entry.status || "";
  }
  const approval = (state.activeSnapshot?.approvals || []).find((item) => item.id === approvalId);
  return approval?.status || entry.status || "";
}

function isEntryVisible(entry) {
  if (entry.kind === "approval") {
    return currentApprovalStatus(entry) === "pending";
  }
  if (entry.kind === "question") {
    return entry.status === "pending";
  }
  return true;
}

function isInlineActivity(entry) {
  return ["tool_call", "change", "verification", "system"].includes(entry.kind);
}

function latestOpenToolCallEntryId() {
  const entries = state.activeSnapshot?.entries || [];
  const activeRunId = state.activeRunId;
  if (!activeRunId) {
    return "";
  }
  const toolCalls = entries.filter((entry) => entry.kind === "tool_call" && entry.run_id === activeRunId);
  for (let index = toolCalls.length - 1; index >= 0; index -= 1) {
    const toolCall = toolCalls[index];
    const hasResult = entries.some((entry) => entry.kind === "tool_result" && entry.meta?.tool_call_entry_id === toolCall.id);
    if (!hasResult) {
      return toolCall.id;
    }
  }
  return "";
}

function lastEntryForKinds(kinds, runId = "") {
  const entries = state.activeSnapshot?.entries || [];
  for (let index = entries.length - 1; index >= 0; index -= 1) {
    const entry = entries[index];
    if (!kinds.includes(entry.kind)) {
      continue;
    }
    if (runId && entry.run_id !== runId) {
      continue;
    }
    return entry;
  }
  return null;
}

function activePlanEntry() {
  return lastEntryForKinds(["plan"], state.activeRunId) || lastEntryForKinds(["plan"]);
}

function thinkingStatusSummary() {
  const session = state.activeSnapshot?.session;
  if (!session) {
    return "";
  }
  if (session.status === "queued") {
    return "Queued and waiting to start";
  }
  const openToolCallId = latestOpenToolCallEntryId();
  if (openToolCallId) {
    const toolEntry = (state.activeSnapshot?.entries || []).find((entry) => entry.id === openToolCallId);
    if (toolEntry) {
      return activityText(toolEntry) || "Running a tool";
    }
  }
  const latestActivity = lastEntryForKinds(["tool_call", "change", "verification", "system"], state.activeRunId);
  if (latestActivity) {
    return activityText(latestActivity);
  }
  return "Working on your request";
}

function shouldShowThinkingEntry() {
  const session = state.activeSnapshot?.session;
  if (!session || !["running", "queued"].includes(session.status || "")) {
    return false;
  }
  if ((state.activeSnapshot?.entries || []).some((entry) => entry.kind === "question" && entry.status === "pending")) {
    return false;
  }
  if ((state.activeSnapshot?.approvals || []).some((approval) => approval.status === "pending")) {
    return false;
  }
  return true;
}

function pendingThinkingEntry() {
  if (!shouldShowThinkingEntry()) {
    return null;
  }
  return {
    id: "__thinking__",
    kind: "thinking_status",
    status: state.activeSnapshot?.session?.status || "running",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString()
  };
}

function transcriptEntries() {
  const sessionEntries = (state.activeSnapshot?.entries || []).filter((entry) => {
    if (!["user_message", "assistant_message", "plan", "approval", "question", "tool_call", "change", "verification", "system"].includes(entry.kind)) {
      return false;
    }
    return isEntryVisible(entry);
  });
  const entries = sessionEntries.concat(state.localEntries).sort((left, right) => {
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
  const thinkingEntry = pendingThinkingEntry();
  if (thinkingEntry) {
    entries.push(thinkingEntry);
  }
  return collapseFinishedActivity(entries);
}

function collapseFinishedActivity(entries) {
  const collapsed = [];
  for (let index = 0; index < entries.length; index += 1) {
    const entry = entries[index];
    if (
      !isInlineActivity(entry) ||
      !entry.run_id ||
      entry.run_id === state.activeRunId
    ) {
      collapsed.push(entry);
      continue;
    }
    const group = [entry];
    let cursor = index + 1;
    while (cursor < entries.length) {
      const candidate = entries[cursor];
      if (!isInlineActivity(candidate) || candidate.run_id !== entry.run_id) {
        break;
      }
      group.push(candidate);
      cursor += 1;
    }
    index = cursor - 1;
    const groupId = `activity_group_${entry.run_id}_${entry.id}`;
    collapsed.push({
      id: groupId,
      kind: "activity_group",
      group_id: groupId,
      run_id: entry.run_id,
      entries: group,
      expanded: Boolean(state.expandedActivityGroups[groupId]),
      created_at: group[0].created_at,
      updated_at: group[group.length - 1].updated_at
    });
  }
  return collapsed;
}

function renderTranscript() {
  const entries = transcriptEntries();
  transcript.innerHTML = entries.map(renderEntryCard).join("");
  transcript.querySelectorAll("[data-activity-toggle]").forEach((button) => {
    button.addEventListener("click", () => toggleActivityGroup(button.dataset.activityToggle));
  });
  transcript.querySelectorAll("[data-approval-action]").forEach((button) => {
    button.addEventListener("click", () => resolveApproval(button.dataset.approvalId, button.dataset.approvalAction));
  });
  transcript.querySelectorAll("[data-question-option]").forEach((button) => {
    button.addEventListener("click", () => answerQuestion(button.dataset.questionId, button.dataset.questionOption));
  });
  transcript.querySelectorAll("[data-edit-entry]").forEach((button) => {
    button.addEventListener("click", () => beginEditMessage(button.dataset.editEntry));
  });
  transcript.scrollTop = transcript.scrollHeight;
}

function entryLabel(entry) {
  switch (entry.kind) {
    case "user_message":
      return "you";
    case "assistant_message":
      return "assistant";
    case "tool_call":
      return "activity";
    case "change":
      return "change";
    case "verification":
      return "verification";
    case "plan":
      return "thinking";
    case "question":
      return entry.title || "question";
    case "approval":
      return "approval";
    case "system":
      return entry.title || "system";
    default:
      return entry.title || entry.kind;
  }
}

function entryMeta(entry) {
  if (["sending", "queued", "failed", "answered"].includes(entry.status || "")) {
    return entry.status;
  }
  if (entry.kind === "approval") {
    return "waiting";
  }
  if (entry.kind === "question" && entry.status === "pending") {
    return "waiting";
  }
  if (entry.kind === "tool_call" && latestOpenToolCallEntryId() === entry.id) {
    return "running";
  }
  return formatRelative(entry.updated_at || entry.created_at);
}

function planSteps(entry) {
  const steps = Array.isArray(entry.meta?.steps) ? entry.meta.steps : [];
  return steps.map((step) => ({
    title: `${step?.title || ""}`.trim(),
    status: `${step?.status || ""}`.trim().toLowerCase()
  })).filter((step) => step.title);
}

function planSummary(entry) {
  const summary = `${entry.meta?.summary || ""}`.trim();
  if (summary) {
    return summary;
  }
  const firstLine = `${entry.content || ""}`.split("\n").map((line) => line.trim()).find(Boolean);
  return firstLine || "";
}

function planStepStatusLabel(status) {
  switch (status) {
    case "completed":
      return "completed";
    case "in_progress":
      return "in progress";
    case "pending":
      return "pending";
    default:
      return status || "pending";
  }
}

function planStepIcon(status) {
  switch (status) {
    case "completed":
      return "●";
    case "in_progress":
      return "◔";
    default:
      return "○";
  }
}

function renderPlanEntry(entry) {
  const steps = planSteps(entry);
  const completed = steps.filter((step) => step.status === "completed").length;
  const progress = steps.length > 0
    ? `${completed} out of ${steps.length} tasks completed`
    : "thinking through the task";
  const summary = planSummary(entry);
  const summaryMarkup = summary ? `<div class="plan-card-summary">${escapeHTML(summary)}</div>` : "";
  const stepMarkup = steps.map((step) => {
    const status = step.status || "pending";
    return `<div class="plan-step ${escapeHTML(status)}"><span class="plan-step-icon">${escapeHTML(planStepIcon(status))}</span><div><div class="plan-step-text">${escapeHTML(step.title)}</div><div class="plan-step-status">${escapeHTML(planStepStatusLabel(status))}</div></div></div>`;
  }).join("");
  const listMarkup = stepMarkup ? `<div class="plan-step-list">${stepMarkup}</div>` : "";
  return `<p class="plan-caption">Thinking</p><div class="plan-card-shell"><div class="plan-progress">${escapeHTML(progress)}</div>${summaryMarkup}${listMarkup}</div>`;
}

function renderThinkingStatusEntry() {
  const planEntry = activePlanEntry();
  const summary = thinkingStatusSummary();
  const updatedAt = lastEntryForKinds(["tool_call", "change", "verification", "system", "assistant_message"], state.activeRunId)?.updated_at
    || state.activeSnapshot?.session?.updated_at
    || "";
  const statusLine = updatedAt ? `Last update ${escapeHTML(formatRelative(updatedAt))}` : "Waiting for the next update";
  const summaryMarkup = summary ? `<div class="thinking-status-summary">${escapeHTML(summary)}</div>` : "";
  const detailMarkup = `<div class="thinking-status-detail">${statusLine}</div>`;
  if (!planEntry) {
    return `<p class="plan-caption">Thinking</p><div class="thinking-status-shell"><div class="thinking-status-pill">live</div>${summaryMarkup}${detailMarkup}</div>`;
  }
  const steps = planSteps(planEntry);
  const completed = steps.filter((step) => step.status === "completed").length;
  const progress = steps.length > 0
    ? `${completed} out of ${steps.length} tasks completed`
    : "thinking through the task";
  const stepMarkup = steps.map((step) => {
    const status = step.status || "pending";
    return `<div class="plan-step ${escapeHTML(status)}"><span class="plan-step-icon">${escapeHTML(planStepIcon(status))}</span><div><div class="plan-step-text">${escapeHTML(step.title)}</div><div class="plan-step-status">${escapeHTML(planStepStatusLabel(status))}</div></div></div>`;
  }).join("");
  const listMarkup = stepMarkup ? `<div class="plan-step-list">${stepMarkup}</div>` : "";
  return `<p class="plan-caption">Thinking</p><div class="thinking-status-shell"><div class="thinking-status-pill">live</div><div class="plan-progress">${escapeHTML(progress)}</div>${summaryMarkup}${detailMarkup}${listMarkup}</div>`;
}

function compactToolArgs(value) {
  if (!Array.isArray(value)) {
    return "";
  }
  return value.map((item) => `${item ?? ""}`.trim()).filter(Boolean).join(" ");
}

function activityText(entry) {
  if (entry.kind === "tool_call") {
    const tool = `${entry.title || ""}`.trim();
    if (tool === "cmd") {
      const command = `${entry.meta?.command || ""}`.trim();
      const args = compactToolArgs(entry.meta?.args);
      const payload = [command, args].filter(Boolean).join(" ");
      return payload ? `Ran ${payload}` : "Ran command";
    }
    if (tool === "apply_patch") {
      return "Applied a patch";
    }
    if (tool === "update_plan") {
      return "Updated the plan";
    }
    if (tool === "request_user_input") {
      return "Asked for a quick choice";
    }
    return tool ? `Ran ${tool}` : "Ran tool";
  }
  if (entry.kind === "change") {
    return entry.content || "Recorded a change";
  }
  if (entry.kind === "verification") {
    return entry.content || "Verification finished";
  }
  return entry.content || entry.title || entry.kind;
}

function toggleActivityGroup(groupId) {
  if (!groupId) {
    return;
  }
  state.expandedActivityGroups[groupId] = !state.expandedActivityGroups[groupId];
  renderTranscript();
}

function renderActivityGroupEntry(entry) {
  const logs = Array.isArray(entry.entries) ? entry.entries : [];
  const count = logs.length;
  const latest = logs[logs.length - 1];
  const summary = latest ? activityText(latest) : "Activity finished";
  const details = entry.expanded
    ? `<div class="activity-group-lines">${logs.map((log) => `<div class="activity-line">${escapeHTML(activityText(log))}</div>`).join("")}</div>`
    : "";
  const toggleLabel = entry.expanded ? "hide logs" : "show logs";
  return `
    <article class="entry-card activity-group">
      <div class="entry-head">
        <span class="entry-title">activity</span>
        <div class="entry-head-meta">
          <span class="entry-meta">${escapeHTML(formatRelative(entry.updated_at || entry.created_at))}</span>
        </div>
      </div>
      <div class="activity-group-summary">${escapeHTML(count === 1 ? "1 activity update" : `${count} activity updates`)}</div>
      <div class="activity-group-latest">${escapeHTML(summary)}</div>
      ${details}
      <div class="entry-card-actions">
        <button class="entry-inline-action" data-activity-toggle="${escapeHTML(entry.group_id || entry.id)}">${toggleLabel}</button>
      </div>
    </article>
  `;
}

function renderQuestionEntry(entry) {
  const questionId = `${entry.meta?.question_id || ""}`.trim();
  const options = Array.isArray(entry.meta?.options) ? entry.meta.options : [];
  const answer = questionAnswer(entry);
  const buttons = options.map((option) => {
    const optionId = `${option.id || ""}`.trim();
    const selected = answer?.option_id === optionId;
    const selectedClass = selected ? " selected" : "";
    const disabled = entry.status !== "pending" ? " disabled" : "";
    const description = option.description ? `<div class="question-option-description">${escapeHTML(option.description)}</div>` : "";
    return `<button class="question-option${selectedClass}" data-question-id="${escapeHTML(questionId)}" data-question-option="${escapeHTML(optionId)}"${disabled}><div class="question-option-label">${escapeHTML(option.label || optionId)}</div>${description}</button>`;
  }).join("");
  const answerSummary = answer ? `<div class="question-answer-summary">selected: ${escapeHTML(answer.answer || answer.label)}</div>` : "";
  return `<div class="entry-content">${escapeHTML(entry.content || "")}</div><div class="question-options">${buttons}</div>${answerSummary}`;
}

function questionAnswer(entry) {
  const answer = entry.meta?.answer;
  if (!answer) {
    return null;
  }
  return {
    option_id: `${answer.option_id || ""}`.trim(),
    label: `${answer.label || answer.answer || ""}`.trim(),
    description: `${answer.description || ""}`.trim(),
    answer: `${answer.answer || answer.label || ""}`.trim()
  };
}

function renderEntryCard(entry) {
  if (entry.kind === "activity_group") {
    return renderActivityGroupEntry(entry);
  }
  if (isInlineActivity(entry)) {
    return `<article class="entry-card activity-entry ${escapeHTML(entry.kind)}"><div class="activity-line">${escapeHTML(activityText(entry))}</div></article>`;
  }
  if (entry.kind === "plan") {
    if (shouldShowThinkingEntry() && activePlanEntry()?.id === entry.id) {
      return "";
    }
    return `<article class="entry-card plan ${escapeHTML(entry.status || "")}">${renderPlanEntry(entry)}</article>`;
  }
  if (entry.kind === "thinking_status") {
    return `<article class="entry-card thinking-status ${escapeHTML(entry.status || "")}">${renderThinkingStatusEntry()}</article>`;
  }
  const status = entry.status ? ` ${escapeHTML(entry.status)}` : "";
  let actions = "";
  let body = `<div class="entry-content">${escapeHTML(entry.content || "")}</div>`;
  if (entry.kind === "approval" && currentApprovalStatus(entry) === "pending") {
    const approvalId = entry.meta?.approval_id;
    if (approvalId) {
      actions = `<div class="approval-actions"><button class="ghost-button" data-approval-action="reject" data-approval-id="${escapeHTML(approvalId)}">reject</button><button class="primary-button" data-approval-action="approve" data-approval-id="${escapeHTML(approvalId)}">approve</button></div>`;
    }
  }
  if (entry.kind === "question") {
    body = renderQuestionEntry(entry);
  }
  if (
    entry.kind === "user_message" &&
    !`${entry.id || ""}`.startsWith("local_") &&
    !["sending", "queued"].includes(entry.status || "") &&
    !["running", "queued"].includes(state.activeSnapshot?.session?.status || "")
  ) {
    actions = `${actions}<div class="entry-card-actions"><button class="entry-inline-action" data-edit-entry="${escapeHTML(entry.id)}">edit</button></div>`;
  }
  return `<article class="entry-card ${escapeHTML(entry.kind)}${status}"><div class="entry-head"><span class="entry-title">${escapeHTML(entryLabel(entry))}</span><div class="entry-head-meta"><span class="entry-meta">${escapeHTML(entryMeta(entry))}</span></div></div>${body}${actions}</article>`;
}

function renderAttachments() {
  const attachments = state.activeSnapshot?.attachments || [];
  attachmentSummary.textContent = attachments.length === 0 ? "no files attached" : `${attachments.length} file${attachments.length === 1 ? "" : "s"} attached`;
  if (!composerAttachments) {
    return;
  }
  composerAttachments.classList.toggle("is-hidden", attachments.length === 0);
  composerAttachments.innerHTML = attachments.map((attachment) => {
    const primary = isImageAttachment(attachment)
      ? `<button class="composer-attachment-open" data-attachment-preview="${escapeHTML(attachment.id)}" aria-label="preview ${escapeHTML(attachment.name)}"><img class="composer-attachment-thumb" src="${escapeHTML(attachmentContentURL(attachment))}" alt="${escapeHTML(attachment.name)}"></button>`
      : `<a class="composer-attachment-open" href="${escapeHTML(attachmentContentURL(attachment))}" target="_blank" rel="noreferrer" aria-label="open ${escapeHTML(attachment.name)}"><span class="composer-attachment-fallback">${escapeHTML(attachmentTypeLabel(attachment))}</span></a>`;
    const title = isImageAttachment(attachment)
      ? `<button class="composer-attachment-name composer-chip-button" data-attachment-preview="${escapeHTML(attachment.id)}">${escapeHTML(attachment.name)}</button>`
      : `<a class="composer-attachment-name composer-chip-button" href="${escapeHTML(attachmentContentURL(attachment))}" target="_blank" rel="noreferrer">${escapeHTML(attachment.name)}</a>`;
    return `<div class="composer-attachment">${primary}<div class="composer-attachment-body">${title}<div class="composer-attachment-meta">${escapeHTML(formatFileSize(attachment.size))}</div></div><div class="composer-attachment-actions"><button class="composer-chip-button danger" data-attachment-remove="${escapeHTML(attachment.id)}" aria-label="remove ${escapeHTML(attachment.name)}">x</button></div></div>`;
  }).join("");
  composerAttachments.querySelectorAll("[data-attachment-preview]").forEach((button) => {
    button.addEventListener("click", () => openAttachmentPreview(button.dataset.attachmentPreview));
  });
  composerAttachments.querySelectorAll("[data-attachment-remove]").forEach((button) => {
    button.addEventListener("click", () => deleteAttachment(button.dataset.attachmentRemove));
  });
}

function renderComposerState() {
  const session = state.activeSnapshot?.session;
  const pendingLocal = [...state.localEntries].reverse().find((entry) => entry.kind === "user_message" && ["sending", "queued"].includes(entry.status || ""));
  const editing = currentEditingEntry();
  const pendingQuestion = (state.activeSnapshot?.entries || []).find((entry) => entry.kind === "question" && entry.status === "pending");
  const pendingApproval = (state.activeSnapshot?.approvals || []).find((approval) => approval.status === "pending");
  stopButton.classList.toggle("is-hidden", !state.activeRunId);
  cancelEditButton.classList.toggle("is-hidden", !editing);
  sendButton.textContent = editing ? "save" : "send";
  promptInput.placeholder = editing ? "edit your message" : "ask for follow-up changes";
  if (!session) {
    composerStatus.textContent = hasReadyProvider() ? "ready" : "choose a provider";
    return;
  }
  if (editing) {
    composerStatus.textContent = "editing message";
    return;
  }
  if (pendingLocal) {
    composerStatus.textContent = pendingLocal.status;
    return;
  }
  if (pendingQuestion) {
    composerStatus.textContent = "answer the question to continue";
    return;
  }
  if (pendingApproval) {
    composerStatus.textContent = "approve or reject to continue";
    return;
  }
  if (session.status === "running" || session.status === "queued") {
    composerStatus.textContent = session.status === "queued" ? "queued" : "run in progress";
    return;
  }
  if (session.status === "failed") {
    composerStatus.textContent = "last run failed";
    return;
  }
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
  renderAttachments();
  renderComposerState();
}

async function sendPrompt() {
  const prompt = promptInput.value.trim();
  if (!prompt) {
    return;
  }
  if (state.editingEntryId) {
    try {
      sendButton.disabled = true;
      composerStatus.textContent = "saving edit";
      const res = await api(`/api/v1/sessions/${state.activeSessionId}/entries/${state.editingEntryId}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content: prompt })
      });
      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error?.message || "could not save edit");
      }
      if (data.entry) {
        upsertEntry(data.entry);
      }
      state.editingEntryId = "";
      promptInput.value = "";
      composerStatus.textContent = "message updated";
      renderAll();
    } catch (error) {
      composerStatus.textContent = error.message || "could not save edit";
    } finally {
      sendButton.disabled = false;
    }
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
    state.activeSnapshot.session.thinking = payload.reasoning_effort;
  }
  promptInput.value = "";
  sendButton.disabled = true;
  composerStatus.textContent = "sending";
  renderTranscript();
  renderComposerState();
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

async function setThinking() {
  const thinking = currentThinkingValue();
  saveStoredConfig({ ...currentConfig(), thinking });
  renderProviderState();
  if (!state.activeSessionId) {
    renderBanner();
    return;
  }
  try {
    const res = await api(`/api/v1/sessions/${state.activeSessionId}/thinking`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ thinking })
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error?.message || "could not save thinking mode");
    }
    upsertSession(data.session);
    if (state.activeSnapshot?.session) {
      state.activeSnapshot.session.thinking = data.session.thinking;
    }
    renderBanner();
    renderSessions();
    renderComposerState();
  } catch (error) {
    thinkingSelect.value = normalizeThinkingValue(state.activeSnapshot?.session?.thinking || currentConfig().thinking);
    composerStatus.textContent = error.message || "could not save thinking mode";
    renderProviderState();
  }
}

async function resolveApproval(approvalId, decision) {
  if (!approvalId) {
    return;
  }
  try {
    const res = await api(`/api/v1/approvals/${approvalId}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ decision })
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error?.message || "could not resolve approval");
    }
    if (data.approval) {
      upsertApproval(data.approval);
    }
    composerStatus.textContent = decision === "approve" ? "approved" : "rejected";
    renderAll();
  } catch (error) {
    composerStatus.textContent = error.message || "could not resolve approval";
  }
}

async function uploadFiles(files) {
  if (files.length === 0) {
    return;
  }
  if (!state.activeSessionId) {
    const session = await createSession();
    if (!session?.id) {
      composerStatus.textContent = "could not create a session for uploads";
      return;
    }
  }
  attachmentSummary.textContent = `uploading ${files.length} file${files.length === 1 ? "" : "s"}...`;
  for (const file of files) {
    try {
      const form = new FormData();
      form.append("file", file);
      const res = await api(`/api/v1/sessions/${state.activeSessionId}/attachments`, {
        method: "POST",
        body: form
      });
      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error?.message || `could not upload ${file.name}`);
      }
      if (data.attachment) {
        upsertAttachment(data.attachment);
      }
    } catch (error) {
      composerStatus.textContent = error.message || `could not upload ${file.name}`;
      return;
    }
  }
  renderAttachments();
  composerStatus.textContent = "attachments ready";
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
sessionRailToggle?.addEventListener("click", toggleSessionRail);
sessionRailBackdrop?.addEventListener("click", closeSessionRail);
newSessionButton.addEventListener("click", createSession);
deleteSessionButton.addEventListener("click", () => deleteSession());
thinkingSelect.addEventListener("change", setThinking);
modeSelect.addEventListener("change", setMode);
sendButton.addEventListener("click", sendPrompt);
cancelEditButton.addEventListener("click", cancelEditMessage);
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
promptInput.addEventListener("paste", async (event) => {
  const files = clipboardImageFiles(event);
  if (files.length === 0) {
    return;
  }
  event.preventDefault();
  await uploadFiles(files);
});
providerDialog.addEventListener("click", (event) => {
  if (event.target === providerDialog) {
    closeProviderDialog();
  }
});
attachmentPreviewClose?.addEventListener("click", closeAttachmentPreview);
attachmentPreviewDialog?.addEventListener("click", (event) => {
  if (event.target === attachmentPreviewDialog) {
    closeAttachmentPreview();
  }
});
window.addEventListener("keydown", (event) => {
  if (event.key === "Escape") {
    closeSessionRail();
  }
});

async function init() {
  renderProviderState();
  renderVisibility();
  renderAll();
  await detectBackendOrigin();
  await refreshProviders();
  await loadSessions();
  openCatalogSocket();
  renderAll();
}

init();
