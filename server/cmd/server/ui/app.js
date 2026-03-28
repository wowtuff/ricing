const state = {
  sessions: [],
  activeSessionId: "",
  activeSnapshot: null,
  provider: null,
  connected: false,
  catalogSocket: null,
  sessionSocket: null,
  activeRunId: "",
  search: ""
};

const providerStatus = document.getElementById("providerStatus");
const providerAction = document.getElementById("providerAction");
const connectButton = document.getElementById("connectButton");
const newSessionButton = document.getElementById("newSessionButton");
const sessionSearch = document.getElementById("sessionSearch");
const sessionList = document.getElementById("sessionList");
const sessionTitle = document.getElementById("sessionTitle");
const sessionModeLabel = document.getElementById("sessionModeLabel");
const sessionState = document.getElementById("sessionState");
const transcript = document.getElementById("transcript");
const connectPanel = document.getElementById("connectPanel");
const emptyPanel = document.getElementById("emptyPanel");
const modeSelect = document.getElementById("modeSelect");
const sendButton = document.getElementById("sendButton");
const stopButton = document.getElementById("stopButton");
const promptInput = document.getElementById("promptInput");
const composerStatus = document.getElementById("composerStatus");
const runStatus = document.getElementById("runStatus");
const nowCard = document.getElementById("nowCard");
const approvalList = document.getElementById("approvalList");
const attachmentList = document.getElementById("attachmentList");
const activityList = document.getElementById("activityList");
const attachButton = document.getElementById("attachButton");
const fileInput = document.getElementById("fileInput");
const attachmentSummary = document.getElementById("attachmentSummary");

function api(path, options = {}) {
  return fetch(path, options);
}

function wsURL() {
  const base = window.location.origin.replace(/^http/, "ws");
  return `${base}/api/v1/ws`;
}

function setProviderState() {
  providerStatus.className = "status-pill";
  if (state.connected) {
    providerStatus.classList.add("connected");
    providerStatus.textContent = "provider linked";
    providerAction.textContent = "relink";
  } else {
    providerStatus.textContent = "provider offline";
    providerAction.textContent = "connect";
  }
}

async function refreshProvider() {
  try {
    const res = await api("/api/v1/providers");
    const data = await res.json();
    const provider = data.providers?.find((item) => item.id === data.default_provider_id) || data.providers?.[0] || null;
    state.provider = provider;
    state.connected = provider?.state === "connected";
  } catch (error) {
    state.provider = null;
    state.connected = false;
  }
  setProviderState();
  renderVisibility();
}

async function connectProvider() {
  providerAction.disabled = true;
  connectButton.disabled = true;
  providerStatus.textContent = "opening auth";
  try {
    const res = await api("/api/v1/providers");
    const data = await res.json();
    const providerId = data.default_provider_id;
    const connectRes = await api(`/api/v1/providers/${providerId}/connect`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ open_browser: "server" })
    });
    const connectData = await connectRes.json();
    if (connectData.auth_url) {
      window.open(connectData.auth_url, "_blank", "width=600,height=760");
    }
    const started = Date.now();
    while (Date.now() - started < 180000) {
      await wait(2000);
      await refreshProvider();
      if (state.connected) {
        break;
      }
    }
  } finally {
    providerAction.disabled = false;
    connectButton.disabled = false;
    await refreshProvider();
  }
}

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function loadSessions() {
  const res = await api("/api/v1/sessions");
  const data = await res.json();
  state.sessions = Array.isArray(data.sessions) ? data.sessions : [];
  renderSessions();
  if (!state.activeSessionId && state.sessions.length > 0) {
    await selectSession(state.sessions[0].id);
  } else if (state.activeSessionId) {
    const exists = state.sessions.find((session) => session.id === state.activeSessionId);
    if (!exists && state.sessions.length > 0) {
      await selectSession(state.sessions[0].id);
    }
  }
  renderVisibility();
}

async function createSession() {
  const res = await api("/api/v1/sessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mode: modeSelect.value || "auto" })
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
  const res = await api(`/api/v1/sessions/${sessionId}`);
  const data = await res.json();
  state.activeSnapshot = data;
  modeSelect.value = data.session?.mode || "auto";
  renderAll();
  openSessionSocket(sessionId);
}

function openCatalogSocket() {
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
  if (state.sessionSocket) {
    state.sessionSocket.close();
  }
  const socket = new WebSocket(wsURL());
  socket.onopen = () => {
    socket.send(JSON.stringify({ type: "subscribe", data: { session_id: sessionId, after_seq: 0 } }));
  };
  socket.onmessage = (event) => {
    const message = JSON.parse(event.data);
    handleSessionMessage(message);
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
    if (state.activeSnapshot?.session) {
      state.activeSnapshot.session = { ...state.activeSnapshot.session, ...message.data.session };
    }
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
  if (!session || !session.id) {
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

function upsertEntry(entry) {
  if (!state.activeSnapshot || !entry?.id) {
    return;
  }
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
    state.activeSnapshot.session.latest_preview = entry.content || state.activeSnapshot.session.latest_preview;
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
    const pending = session.pending_approvals ? `<span class="tag">${session.pending_approvals} approvals</span>` : "";
    const mode = `<span class="tag">${session.mode || "auto"}</span>`;
    const status = `<span class="tag">${session.status || "idle"}</span>`;
    return `<button class="session-item${active}" data-session-id="${escapeHTML(session.id)}"><div class="session-item-head"><span class="session-item-title">${escapeHTML(session.title || "new session")}</span><span class="entry-meta">${escapeHTML(formatRelative(session.updated_at))}</span></div><p class="session-item-preview">${escapeHTML(session.latest_preview || "no activity yet")}</p><div class="session-item-tags">${mode}${status}${pending}</div></button>`;
  }).join("");
  sessionList.querySelectorAll("[data-session-id]").forEach((button) => {
    button.addEventListener("click", () => selectSession(button.dataset.sessionId));
  });
}

function renderVisibility() {
  const hasSession = Boolean(state.activeSnapshot?.session?.id);
  const showConnect = !state.connected;
  connectPanel.classList.toggle("visible", showConnect);
  emptyPanel.classList.toggle("visible", !showConnect && !hasSession);
  transcript.classList.toggle("hidden", !hasSession);
  document.querySelector(".composer").classList.toggle("hidden", !hasSession);
}

function renderBanner() {
  const session = state.activeSnapshot?.session;
  if (!session) {
    sessionTitle.textContent = "select a session";
    sessionModeLabel.textContent = modeSelect.value || "auto";
    sessionState.textContent = "idle";
    sessionState.className = "status-pill subtle";
    runStatus.textContent = "idle";
    runStatus.className = "status-pill subtle";
    return;
  }
  sessionTitle.textContent = session.title || "new session";
  sessionModeLabel.textContent = session.mode || "auto";
  sessionState.textContent = session.status || "idle";
  sessionState.className = "status-pill subtle";
  runStatus.textContent = session.status || "idle";
  runStatus.className = "status-pill subtle";
  if (session.status === "running" || session.status === "queued") {
    sessionState.classList.add("running");
    runStatus.classList.add("running");
  } else if (session.status === "failed") {
    sessionState.classList.add("failed");
    runStatus.classList.add("failed");
  }
}

function renderTranscript() {
  const entries = (state.activeSnapshot?.entries || []).filter((entry) => ["user_message", "assistant_message", "plan", "approval", "system"].includes(entry.kind));
  transcript.innerHTML = entries.map(renderEntryCard).join("");
  transcript.querySelectorAll("[data-approval-action]").forEach((button) => {
    button.addEventListener("click", () => resolveApproval(button.dataset.approvalId, button.dataset.approvalAction));
  });
  transcript.scrollTop = transcript.scrollHeight;
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
  return `<article class="entry-card ${escapeHTML(entry.kind)}${status}"><div class="entry-head"><span class="entry-title">${escapeHTML(entryLabel(entry))}</span><span class="entry-meta">${escapeHTML(formatRelative(entry.updated_at || entry.created_at))}</span></div><div class="entry-content">${escapeHTML(entry.content || "")}</div>${actions}</article>`;
}

function entryLabel(entry) {
  switch (entry.kind) {
    case "user_message":
      return "user";
    case "assistant_message":
      return entry.status === "streaming" ? "assistant streaming" : "assistant";
    case "plan":
      return "plan";
    case "approval":
      return `approval ${entry.status || ""}`.trim();
    default:
      return entry.title || entry.kind;
  }
}

function renderApprovals() {
  const approvals = (state.activeSnapshot?.approvals || []).filter((item) => item.status === "pending");
  approvalList.innerHTML = approvals.length === 0 ? `<div class="now-card">no pending approvals</div>` : approvals.map((approval) => {
    return `<div class="approval-card ${escapeHTML(approval.status)}"><div class="approval-title">${escapeHTML(approval.tool_name)}</div><div class="approval-content">${escapeHTML(approval.summary || "")}</div><div class="approval-actions"><button class="ghost-button" data-approval-action="reject" data-approval-id="${escapeHTML(approval.id)}">reject</button><button class="primary-button" data-approval-action="approve" data-approval-id="${escapeHTML(approval.id)}">approve</button></div></div>`;
  }).join("");
  approvalList.querySelectorAll("[data-approval-action]").forEach((button) => {
    button.addEventListener("click", () => resolveApproval(button.dataset.approvalId, button.dataset.approvalAction));
  });
}

function renderAttachments() {
  const attachments = state.activeSnapshot?.attachments || [];
  attachmentSummary.textContent = attachments.length === 0 ? "no files attached" : `${attachments.length} file${attachments.length === 1 ? "" : "s"} attached`;
  attachmentList.innerHTML = attachments.length === 0 ? `<div class="now-card">drop files into the current session to give the agent more context</div>` : attachments.map((attachment) => {
    return `<div class="attachment-item"><div class="attachment-item-title">${escapeHTML(attachment.name)}</div><div class="attachment-item-meta">${escapeHTML(formatFileSize(attachment.size))}</div></div>`;
  }).join("");
}

function renderActivity() {
  const entries = (state.activeSnapshot?.entries || []).filter((entry) => ["tool_call", "tool_result", "change", "verification"].includes(entry.kind));
  activityList.innerHTML = entries.length === 0 ? `<div class="now-card">tool calls, file changes and verification results will show up here</div>` : entries.map((entry) => {
    return `<div class="activity-card"><div class="activity-card-head"><div class="activity-card-title">${escapeHTML(entry.title || entry.kind)}</div><div class="entry-meta">${escapeHTML(entry.kind)}</div></div><div class="activity-card-content">${escapeHTML(entry.content || "")}</div></div>`;
  }).join("");
}

function renderNowCard() {
  const session = state.activeSnapshot?.session;
  const entries = state.activeSnapshot?.entries || [];
  const lastEntry = [...entries].reverse().find((entry) => ["assistant_message", "tool_call", "change", "verification", "plan", "system"].includes(entry.kind));
  if (!session) {
    nowCard.textContent = "waiting for the next step";
    composerStatus.textContent = "ready";
    return;
  }
  if (session.status === "running" || session.status === "queued") {
    nowCard.textContent = lastEntry?.content || `${session.status}…`;
    composerStatus.textContent = "run in progress";
  } else if (session.status === "failed") {
    nowCard.textContent = lastEntry?.content || "the last run failed";
    composerStatus.textContent = "last run failed";
  } else {
    nowCard.textContent = lastEntry?.content || "idle";
    composerStatus.textContent = "ready";
  }
  stopButton.disabled = !state.activeRunId;
}

function renderAll() {
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
  if (!state.activeSessionId) {
    await createSession();
  }
  if (!state.activeSessionId) {
    return;
  }
  promptInput.value = "";
  sendButton.disabled = true;
  composerStatus.textContent = "sending";
  try {
    const res = await api(`/api/v1/sessions/${state.activeSessionId}/messages`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        prompt,
        mode: modeSelect.value,
        llm: { provider_id: state.provider?.id || "" }
      })
    });
    const data = await res.json();
    if (!res.ok) {
      composerStatus.textContent = data.error?.message || "send failed";
      return;
    }
    state.activeRunId = data.run?.id || "";
    if (data.session) {
      upsertSession(data.session);
    }
    await selectSession(state.activeSessionId);
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
  await selectSession(state.activeSessionId);
}

async function stopRun() {
  if (!state.activeRunId) {
    return;
  }
  await api(`/api/v1/runs/${state.activeRunId}/cancel`, { method: "POST" });
}

function formatRelative(input) {
  if (!input) {
    return "";
  }
  const date = new Date(input);
  const delta = Math.max(0, Date.now() - date.getTime());
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
  const days = Math.floor(hours / 24);
  return `${days}d`;
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

providerAction.addEventListener("click", connectProvider);
connectButton.addEventListener("click", connectProvider);
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

async function init() {
  await refreshProvider();
  await loadSessions();
  openCatalogSocket();
  renderAll();
}

init();
