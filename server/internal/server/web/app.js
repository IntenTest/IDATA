(() => {
  "use strict";

  const adminMode = location.pathname === "/admin" || location.pathname.startsWith("/admin/");
  const state = {
    adminMode,
    token: adminMode ? (sessionStorage.getItem("idata_admin_token") || "") : "",
    authMode: adminMode ? "admin" : "", clients: [], selfID: "", selectedID: "", busy: false,
    loginGeneration: 0,
    history: [], historyIndex: 0,
    terminalSocket: null, terminalReady: false, terminalGeneration: 0,
    terminalReconnectTimer: null, terminalReconnectAttempt: 0,
    decoders: { stdout: new TextDecoder(), stderr: new TextDecoder() },
  };
  const $ = (selector) => document.querySelector(selector);
  const elements = {
    dialog: $("#token-dialog"), tokenForm: $("#token-form"), tokenInput: $("#token-input"), tokenError: $("#token-error"),
    deviceList: $("#device-list"), emptyDevices: $("#empty-devices"), refresh: $("#refresh-clients"), lastRefresh: $("#last-refresh"),
    changeToken: $("#change-token"), serverStatus: $("#server-status"), serverStatusDot: $("#server-status-dot"),
    terminalClient: $("#terminal-client"), terminalClientMeta: $("#terminal-client-meta"), terminalOutput: $("#terminal-output"),
    clearTerminal: $("#clear-terminal"), commandForm: $("#command-form"), commandInput: $("#command-input"),
    runCommand: $("#run-command"), prompt: $("#prompt-label"),
    targetsLabel: $("#targets-label"), devicesTitle: $("#devices-title"), emptyTitle: $("#empty-title"),
    emptyDescription: $("#empty-description"), welcomePrimary: $("#welcome-primary"), welcomeSecondary: $("#welcome-secondary"),
    authEyebrow: $("#auth-eyebrow"), authTitle: $("#auth-title"), authDescription: $("#auth-description"),
    authLabel: $("#auth-label"), authSubmit: $("#auth-submit"), scopeDialog: $("#scope-dialog"),
    scopeDescription: $("#scope-description"), logoutSession: $("#logout-session"),
    loginGate: $("#login-gate"), appShell: $("#app-shell"), loginStatus: $("#login-status"),
    windowsLogin: $("#windows-login"), macosLogin: $("#macos-login"),
    macosCommandPanel: $("#macos-command-panel"), macosCommand: $("#macos-command"),
    enrollmentPanel: $("#enrollment-panel"), enrollmentCount: $("#enrollment-count"),
    enrollmentList: $("#enrollment-list"), credentialList: $("#credential-list"),
  };

  function openTokenDialog(invalid = false) {
    elements.tokenError.hidden = !invalid;
    elements.tokenInput.value = "";
    if (!elements.dialog.open) elements.dialog.showModal();
    setTimeout(() => elements.tokenInput.focus(), 0);
  }

  async function api(path, options = {}) {
    const headers = new Headers(options.headers || {});
    if (state.adminMode) headers.set("Authorization", `Bearer ${state.token}`);
    else if (state.authMode === "device" && state.token) headers.set("Authorization", `Device ${state.token}`);
    if (options.body) headers.set("Content-Type", "application/json");
    const response = await fetch(path, { ...options, headers });
    if (response.status === 401) {
      if (state.adminMode) {
        state.token = "";
        sessionStorage.removeItem("idata_admin_token");
        openTokenDialog(true);
      }
      throw new Error(state.adminMode ? "管理员令牌无效" : "免密登录已失效，请重新登录");
    }
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(body.error || `请求失败（HTTP ${response.status}）`);
    return body;
  }

  function setServerStatus(kind, text) {
    elements.serverStatus.textContent = text;
    elements.serverStatusDot.className = `status-dot ${kind}`;
    elements.serverStatusDot.textContent = kind === "online" ? "[在线]" : kind === "error" ? "[错误]" : "[连接中]";
  }

  function osLabel(os) {
    if (os === "darwin") return "MAC";
    if (os === "windows") return "WIN";
    if (os === "linux") return "LIN";
    return (os || "PC").slice(0, 3).toUpperCase();
  }

  function renderClients() {
    elements.deviceList.replaceChildren();
    elements.emptyDevices.hidden = state.clients.length !== 0;
    for (const client of state.clients) {
      const button = document.createElement("button");
      button.type = "button";
      const isOtherDevice = !state.adminMode && state.authMode !== "ip_session" && client.id !== state.selfID;
      button.className = `device${client.id === state.selectedID ? " selected" : ""}${isOtherDevice ? " locked" : ""}`;
      button.setAttribute("role", "option");
      button.setAttribute("aria-selected", String(client.id === state.selectedID));

      const icon = document.createElement("span");
      icon.className = "device-icon";
      icon.textContent = osLabel(client.os);
      const copy = document.createElement("span");
      copy.className = "device-copy";
      const name = document.createElement("strong");
      name.textContent = client.id;
      const meta = document.createElement("span");
      meta.textContent = `${client.hostname || "unknown"} · ${client.arch || "?"}`;
      copy.append(name, meta);
      const online = document.createElement("span");
      online.className = "device-online";
      online.textContent = "在线";
      button.append(icon, copy, online);
      button.addEventListener("click", () => {
        if (isOtherDevice) showScopeNotice(client);
        else selectClient(client.id);
      });
      elements.deviceList.append(button);
    }
  }

  function identityMeta(identity = {}) {
    return [identity.username, identity.hostname, identity.local_ip, identity.mac_address].filter(Boolean).join(" · ");
  }

  function actionButton(text, className, handler) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = className;
    button.textContent = text;
    button.addEventListener("click", handler);
    return button;
  }

  async function enrollmentAction(path) {
    try {
      await api(path, { method: "POST" });
      await refreshEnrollments();
      await refreshClients(false);
    } catch (error) {
      setServerStatus("error", error.message);
    }
  }

  async function revokeCredential(id, clientID) {
    if (!window.confirm(`确定撤销设备“${clientID}”的连接凭据吗？`)) return;
    try {
      await api(`/api/v1/device-credentials/${encodeURIComponent(id)}`, { method: "DELETE" });
      await refreshEnrollments();
    } catch (error) {
      setServerStatus("error", error.message);
    }
  }

  function renderEnrollments(result) {
    const requests = (result.requests || []).filter((request) => request.status === "pending");
    const credentials = result.credentials || [];
    elements.enrollmentCount.textContent = `${requests.length} 个待处理`;
    elements.enrollmentList.replaceChildren();
    if (!requests.length) {
      const empty = document.createElement("p");
      empty.className = "enrollment-empty";
      empty.textContent = "暂无新设备申请";
      elements.enrollmentList.append(empty);
    }
    for (const request of requests) {
      const card = document.createElement("article");
      card.className = "enrollment-card";
      const name = document.createElement("strong");
      name.textContent = request.identity?.client_id || "未知设备";
      const meta = document.createElement("span");
      meta.textContent = identityMeta(request.identity) || request.remote_ip || "无设备信息";
      const remote = document.createElement("small");
      remote.textContent = `来源 ${request.remote_ip || "未知"}`;
      const actions = document.createElement("div");
      actions.className = "enrollment-actions";
      actions.append(
        actionButton("批准", "approve", () => enrollmentAction(`/api/v1/enrollments/${encodeURIComponent(request.id)}/approve`)),
        actionButton("拒绝", "deny", () => enrollmentAction(`/api/v1/enrollments/${encodeURIComponent(request.id)}/deny`)),
      );
      card.append(name, meta, remote, actions);
      elements.enrollmentList.append(card);
    }
    elements.credentialList.replaceChildren();
    if (!credentials.length) {
      const empty = document.createElement("p");
      empty.className = "enrollment-empty";
      empty.textContent = "暂无已授权设备";
      elements.credentialList.append(empty);
    }
    for (const credential of credentials) {
      const row = document.createElement("div");
      row.className = "credential-row";
      const copy = document.createElement("span");
      const name = document.createElement("strong");
      name.textContent = credential.client_id;
      const meta = document.createElement("small");
      meta.textContent = identityMeta(credential);
      copy.append(name, meta);
      row.append(copy, actionButton("撤销", "revoke", () => revokeCredential(credential.id, credential.client_id)));
      elements.credentialList.append(row);
    }
  }

  async function refreshEnrollments() {
    if (!state.adminMode || !state.token) return;
    try {
      renderEnrollments(await api("/api/v1/enrollments"));
    } catch (error) {
      setServerStatus("error", error.message);
    }
  }

  function selectClient(id) {
    const launch = document.getElementById("open-idata");
    if (launch) launch.remove();
    if (arguments[0]) {
      const link = document.createElement("a");
      link.id = "open-idata";
      link.textContent = "Open IDATA test workspace";
      link.href = `/?client=${encodeURIComponent(arguments[0])}${state.adminMode ? "&mode=admin" : ""}`;
      elements.terminalClient.parentElement.appendChild(link);
    }

    if (!state.adminMode && state.authMode !== "ip_session" && id && id !== state.selfID) {
      const client = state.clients.find((item) => item.id === id);
      showScopeNotice(client);
      return;
    }
    const changed = state.selectedID !== id;
    if (changed) closeTerminal();
    state.selectedID = id;
    const client = state.clients.find((item) => item.id === id);
    if (!client) {
      state.selectedID = "";
      elements.terminalClient.textContent = "未选择设备";
      elements.terminalClientMeta.textContent = "请选择左侧在线客户端";
      elements.prompt.textContent = "$";
    } else {
      elements.terminalClient.textContent = client.id;
      elements.terminalClientMeta.textContent = `${client.os}/${client.arch} · 正在建立交互终端…`;
      elements.prompt.textContent = client.os === "windows" ? ">" : "$";
    }
    elements.commandInput.disabled = !client || !state.terminalReady;
    elements.runCommand.disabled = !client || !state.terminalReady;
    elements.commandInput.placeholder = client ? "正在连接远程 Shell…" : "选择设备后输入命令…";
    renderClients();
    if (client && changed) openTerminal(client);
  }

  function showScopeNotice(client) {
    const target = client?.id ? `“${client.id}”` : "该设备";
    elements.scopeDescription.textContent = state.authMode === "ip_session"
      ? `${target} 已不属于当前来源 IP 的在线列表，请刷新后重试。`
      : `${target} 不是当前电脑。普通设备页面只能操作本机 Client（${state.selfID || "尚未识别"}）。`;
    if (!elements.scopeDialog.open) elements.scopeDialog.showModal();
  }

  function closeTerminal() {
    state.terminalGeneration += 1;
    state.terminalReady = false;
    state.terminalReconnectAttempt = 0;
    if (state.terminalReconnectTimer !== null) {
      clearTimeout(state.terminalReconnectTimer);
      state.terminalReconnectTimer = null;
    }
    const socket = state.terminalSocket;
    state.terminalSocket = null;
    if (socket && socket.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify({ type: "terminal_close" }));
      socket.close(1000, "switching terminal");
    }
  }

  function scheduleTerminalReconnect(client, message) {
    if (!client || state.selectedID !== client.id || state.terminalReconnectTimer !== null) return;
    state.terminalReady = false;
    elements.commandInput.disabled = true;
    elements.runCommand.disabled = true;
    elements.commandInput.placeholder = "远程 Shell 正在重新连接…";
    elements.terminalClientMeta.textContent = `${client.os}/${client.arch} · ${message}`;
    const generation = state.terminalGeneration;
    const delayMs = Math.min(1000 * (2 ** state.terminalReconnectAttempt), 10000);
    state.terminalReconnectAttempt += 1;
    state.terminalReconnectTimer = setTimeout(() => {
      state.terminalReconnectTimer = null;
      if (generation !== state.terminalGeneration || state.selectedID !== client.id) return;
      state.terminalGeneration += 1;
      if (state.terminalSocket && state.terminalSocket.readyState < WebSocket.CLOSING) {
        state.terminalSocket.close(1012, "restarting terminal");
      }
      state.terminalSocket = null;
      openTerminal(client);
    }, delayMs);
  }

  function openTerminal(client) {
    const generation = state.terminalGeneration;
    let reconnectAllowed = true;
    state.decoders = { stdout: new TextDecoder(), stderr: new TextDecoder() };
    const scheme = location.protocol === "https:" ? "wss" : "ws";
    const socket = new WebSocket(`${scheme}://${location.host}/api/v1/clients/${encodeURIComponent(client.id)}/terminal`);
    state.terminalSocket = socket;
    socket.addEventListener("open", () => {
      if (generation !== state.terminalGeneration) return socket.close();
      const auth = state.adminMode
        ? { type: "auth", token: state.token }
        : state.authMode === "ip_session"
          ? { type: "auth", mode: "ip_session" }
          : state.authMode === "session"
            ? { type: "auth", mode: "session" }
          : { type: "auth", mode: "device", token: state.token };
      socket.send(JSON.stringify(auth));
    });
    socket.addEventListener("message", (event) => {
      if (generation !== state.terminalGeneration) return;
      let message;
      try { message = JSON.parse(event.data); } catch (_) { return; }
      if (message.type === "terminal_opened") {
        state.terminalReady = true;
        state.terminalReconnectAttempt = 0;
        elements.terminalClientMeta.textContent = `${client.os}/${client.arch} · ${client.hostname || "unknown"} · 交互终端已连接`;
        elements.commandInput.disabled = false;
        elements.runCommand.disabled = false;
        elements.commandInput.placeholder = "输入命令后按 Enter…";
        elements.commandInput.focus();
      } else if (message.type === "terminal_output") {
        appendTerminalOutput(message.stream || "stdout", decodeTerminalData(message.stream || "stdout", message.data || ""));
      } else if (message.type === "terminal_closed") {
        state.terminalReady = false;
        appendTerminalOutput("stderr", `\n[远程 Shell 已退出，exit ${message.exit_code ?? 0}${message.error ? `：${message.error}` : ""}]\n`);
        scheduleTerminalReconnect(client, "远程 Shell 已退出，正在自动恢复…");
      } else if (message.type === "terminal_error") {
        state.terminalReady = false;
        appendTerminalOutput("stderr", `\n[终端连接失败：${message.error || "未知错误"}]\n`);
        if (message.error === "device_scope_violation") {
          reconnectAllowed = false;
          showScopeNotice(client);
        } else if (message.error === "unauthorized" && state.authMode === "device") {
          reconnectAllowed = false;
          openTokenDialog(true);
        } else if (message.error === "unauthorized" && (state.authMode === "session" || state.authMode === "ip_session")) {
          reconnectAllowed = false;
          showLoginGate();
        } else {
          scheduleTerminalReconnect(client, "终端暂时不可用，正在自动重试…");
        }
      }
    });
    socket.addEventListener("close", () => {
      if (generation !== state.terminalGeneration) return;
      state.terminalReady = false;
      if (reconnectAllowed && state.selectedID === client.id) scheduleTerminalReconnect(client, "终端连接中断，正在自动重试…");
    });
  }

  function decodeTerminalData(stream, encoded) {
    if (!encoded) return "";
    const raw = atob(encoded);
    const bytes = Uint8Array.from(raw, (character) => character.charCodeAt(0));
    return state.decoders[stream].decode(bytes, { stream: true });
  }

  function encodeTerminalData(value) {
    const bytes = new TextEncoder().encode(value);
    let raw = "";
    for (const byte of bytes) raw += String.fromCharCode(byte);
    return btoa(raw);
  }

  function appendTerminalOutput(kind, value) {
    if (!value) return;
    const previous = elements.terminalOutput.lastElementChild;
    if (previous && previous.classList.contains("live-stream") && previous.dataset.kind === kind) {
      previous.append(document.createTextNode(value));
    } else {
      const stream = document.createElement("pre");
      stream.className = `stream ${kind} live-stream`;
      stream.dataset.kind = kind;
      stream.textContent = value;
      elements.terminalOutput.append(stream);
    }
    elements.terminalOutput.scrollTop = elements.terminalOutput.scrollHeight;
  }

  async function refreshClients(showLoginOnFailure = true) {
    state.busy = true;
    elements.refresh.classList.add("loading");
    try {
      const result = await api(state.adminMode ? "/api/v1/clients" : "/api/v1/self");
      state.clients = result.clients || [];
      state.selfID = state.adminMode ? "" : (result.self_client_id || "");
      if (!state.adminMode) state.authMode = result.auth_mode || "ip_session";
      setServerStatus("online", state.adminMode ? `${state.clients.length} 台设备在线` : `${state.clients.length} 台同 IP 设备在线`);
      elements.lastRefresh.textContent = `更新于 ${new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })}`;
      if (!state.adminMode) {
        elements.devicesTitle.textContent = "在线设备";
        elements.changeToken.hidden = true;
        elements.logoutSession.hidden = false;
        elements.loginGate.hidden = true;
        elements.appShell.hidden = false;
        if (state.selectedID && !state.clients.some((client) => client.id === state.selectedID)) selectClient("");
        else renderClients();
      } else if (state.selectedID && !state.clients.some((client) => client.id === state.selectedID)) {
        selectClient("");
      } else {
        renderClients();
      }
      if (state.adminMode) await refreshEnrollments();
      return true;
    } catch (error) {
      setServerStatus("error", error.message);
      if (!state.adminMode) {
        state.authMode = "";
        state.clients = [];
        state.selfID = "";
        if (state.selectedID) selectClient("");
        else renderClients();
        elements.logoutSession.hidden = true;
        if (showLoginOnFailure) showLoginGate();
      }
      return false;
    } finally {
      state.busy = false;
      elements.refresh.classList.remove("loading");
    }
  }

  function delay(milliseconds) {
    return new Promise((resolve) => setTimeout(resolve, milliseconds));
  }

  function showLoginGate(message = "") {
    closeTerminal();
    state.authMode = "";
    state.clients = [];
    state.selfID = "";
    state.selectedID = "";
    elements.appShell.hidden = true;
    elements.loginGate.hidden = false;
    elements.windowsLogin.disabled = false;
    elements.macosLogin.disabled = false;
    elements.loginStatus.hidden = message === "";
    elements.loginStatus.textContent = message;
  }

  function setLoginLaunchBusy(busy) {
    state.busy = busy;
    elements.windowsLogin.disabled = busy;
    elements.macosLogin.disabled = busy;
  }

  function currentAgentURL() {
    const hostname = location.hostname.replace(/^\[|\]$/g, "");
    const host = hostname.includes(":") ? `[${hostname}]` : hostname;
    const port = location.port || (location.protocol === "https:" ? "443" : "80");
    const scheme = location.protocol === "https:" ? "wss" : "ws";
    return `${scheme}://${host}:${port}/ws/agent`;
  }

  function shellQuote(value) {
    return `'${value.replaceAll("'", `'\\''`)}'`;
  }

  async function waitForPasswordlessLogin(generation, waitingMessage) {
    try {
      const deadline = Date.now() + 30000;
      while (generation === state.loginGeneration && Date.now() < deadline) {
        const response = await fetch("/api/v1/ip-login", { method: "POST" });
        const result = await response.json().catch(() => ({}));
        if (response.ok && result.status === "approved") {
          elements.loginStatus.textContent = `免密登录成功，发现 ${result.client_count || 0} 台同 IP 设备。`;
          if (!await refreshClients(false)) throw new Error("登录会话未能生效，请重试");
          const client = state.clients.find((item) => item.id === state.selfID) || state.clients[0];
          if (!client) throw new Error("Client 已连接，但没有可用的执行设备");
          location.assign(`/idata/?client=${encodeURIComponent(client.id)}`);
          return;
        }
        if (response.status !== 202 || result.status !== "waiting") {
          throw new Error(result.error || `登录失败（HTTP ${response.status}）`);
        }
        elements.loginStatus.textContent = waitingMessage;
        await delay(1000);
      }
      throw new Error("30 秒内未发现同 IP Client，请启动 Client 后重试");
    } catch (error) {
      elements.loginStatus.textContent = error.message;
    } finally {
      setLoginLaunchBusy(false);
    }
  }

  async function startWindowsLogin() {
    if (state.busy) return;
    const generation = ++state.loginGeneration;
    setLoginLaunchBusy(true);
    elements.loginStatus.hidden = false;
    elements.loginStatus.textContent = "正在唤起本机 iData Client…";
    const launchURL = new URL("idata://connect");
    launchURL.searchParams.set("server", location.hostname.replace(/^\[|\]$/g, ""));
    launchURL.searchParams.set("port", location.port || (location.protocol === "https:" ? "443" : "80"));
    launchURL.searchParams.set("secure", location.protocol === "https:" ? "1" : "0");
    location.href = launchURL.toString();
    await waitForPasswordlessLogin(generation, "正在等待本机 Client 接入 Server…");
  }

  async function copyMacOSCommand() {
    if (state.busy) return;
    const generation = ++state.loginGeneration;
    setLoginLaunchBusy(true);
    const command = `~/.local/share/idata-connection/client/idata-client --server ${shellQuote(currentAgentURL())}`;
    elements.macosCommand.value = command;
    elements.macosCommandPanel.hidden = false;
    let copied = false;
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(command);
        copied = true;
      }
    } catch (_) {
      copied = false;
    }
    if (!copied) {
      elements.macosCommand.focus();
      elements.macosCommand.select();
      copied = document.execCommand("copy");
    }
    elements.loginStatus.hidden = false;
    const message = copied
      ? "启动命令已复制；请粘贴到 Terminal 并按 Enter，正在等待 Client…"
      : "请手动复制上方命令，粘贴到 Terminal 并按 Enter；正在等待 Client…";
    elements.loginStatus.textContent = message;
    await waitForPasswordlessLogin(generation, message);
  }

  async function logoutDeviceSession() {
    closeTerminal();
    ++state.loginGeneration;
    try {
      await fetch("/api/v1/self/logout", { method: "POST" });
    } finally {
      state.authMode = "";
      state.clients = [];
      state.selfID = "";
      state.selectedID = "";
      showLoginGate("已退出免密登录。再次进入时请重新唤起本机 Client。");
    }
  }

  function appendCommand(command) {
    const entry = document.createElement("section");
    entry.className = "terminal-entry";
    const line = document.createElement("div");
    line.className = "command-line";
    const prompt = document.createElement("span");
    prompt.className = "entry-prompt";
    prompt.textContent = `${state.selectedID}${elements.prompt.textContent}`;
    const text = document.createElement("span");
    text.textContent = command;
    line.append(prompt, text);
    const running = document.createElement("div");
    running.className = "running-line";
    running.textContent = "正在远程执行…";
    entry.append(line, running);
    elements.terminalOutput.append(entry);
    elements.terminalOutput.scrollTop = elements.terminalOutput.scrollHeight;
    return { entry, running };
  }

  function appendStream(entry, kind, value) {
    if (!value) return;
    const stream = document.createElement("pre");
    stream.className = `stream ${kind}`;
    stream.textContent = value;
    entry.append(stream);
  }

  function finishCommand(view, result, error) {
    view.running.remove();
    if (error) {
      appendStream(view.entry, "stderr", error.message);
    } else {
      appendStream(view.entry, "stdout", result.stdout);
      appendStream(view.entry, "stderr", result.stderr);
      if (!result.stdout && !result.stderr) {
        const empty = document.createElement("p");
        empty.className = "stream empty";
        empty.textContent = "（命令没有产生输出）";
        view.entry.append(empty);
      }
      const exitCode = result.exit_code ?? 0;
      const meta = document.createElement("div");
      meta.className = "command-meta";
      const exit = document.createElement("span");
      exit.className = exitCode === 0 ? "success" : "failed";
      exit.textContent = `exit ${exitCode}`;
      const duration = document.createElement("span");
      duration.textContent = `${result.duration_ms || 0} ms`;
      meta.append(exit, duration);
      if (result.timed_out) {
        const timedOut = document.createElement("span");
        timedOut.className = "failed";
        timedOut.textContent = "已超时";
        meta.append(timedOut);
      }
      if (result.stdout_truncated || result.stderr_truncated) {
        const truncated = document.createElement("span");
        truncated.textContent = "输出已截断";
        meta.append(truncated);
      }
      if (result.error) appendStream(view.entry, "stderr", result.error);
      view.entry.append(meta);
    }
    elements.terminalOutput.scrollTop = elements.terminalOutput.scrollHeight;
  }

  async function executeCommand(command) {
    if (!state.selectedID || !state.terminalReady || !state.terminalSocket) return;
    const view = appendCommand(command);
    view.running.remove();
    const client = state.clients.find((item) => item.id === state.selectedID);
    const newline = client && client.os === "windows" ? "\r\n" : "\n";
    state.terminalSocket.send(JSON.stringify({
      type: "terminal_input",
      data: encodeTerminalData(command + newline),
    }));
  }

  elements.tokenForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    state.token = elements.tokenInput.value.trim();
    sessionStorage.setItem("idata_admin_token", state.token);
    if (await refreshClients(false)) elements.dialog.close();
    else openTokenDialog(true);
  });
  elements.commandForm.addEventListener("submit", (event) => {
    event.preventDefault();
    const command = elements.commandInput.value.trim();
    if (!command) return;
    state.history.push(command);
    state.historyIndex = state.history.length;
    elements.commandInput.value = "";
    executeCommand(command);
  });
  elements.commandInput.addEventListener("keydown", (event) => {
    if (event.key === "ArrowUp" && state.history.length) {
      event.preventDefault();
      state.historyIndex = Math.max(0, state.historyIndex - 1);
      elements.commandInput.value = state.history[state.historyIndex];
    } else if (event.key === "ArrowDown" && state.history.length) {
      event.preventDefault();
      state.historyIndex = Math.min(state.history.length, state.historyIndex + 1);
      elements.commandInput.value = state.history[state.historyIndex] || "";
    } else if (event.key.toLowerCase() === "l" && event.ctrlKey) {
      event.preventDefault();
      elements.terminalOutput.replaceChildren();
    }
  });
  elements.refresh.addEventListener("click", () => {
    if (state.adminMode || state.authMode) refreshClients();
    else initializeDeviceMode();
  });
  elements.clearTerminal.addEventListener("click", () => elements.terminalOutput.replaceChildren());
  elements.changeToken.addEventListener("click", () => {
    if (state.adminMode) openTokenDialog(false);
  });
  elements.logoutSession.addEventListener("click", logoutDeviceSession);
  document.getElementById("running-login").addEventListener("click", async () => {
    if (state.busy) return;
    const generation = ++state.loginGeneration;
    setLoginLaunchBusy(true);
    elements.loginStatus.hidden = false;
    await waitForPasswordlessLogin(generation, "Waiting for the combined IDATA PC launcher...");
  });
  elements.windowsLogin.addEventListener("click", startWindowsLogin);
  elements.macosLogin.addEventListener("click", copyMacOSCommand);
  elements.dialog.addEventListener("cancel", (event) => { if (state.adminMode && !state.token) event.preventDefault(); });

  if (state.adminMode) {
    elements.loginGate.hidden = true;
    elements.appShell.hidden = false;
    elements.targetsLabel.textContent = "TARGETS";
    elements.devicesTitle.textContent = "在线设备";
    elements.emptyTitle.textContent = "没有在线设备";
    elements.emptyDescription.textContent = "启动客户端后，它会自动出现在这里。";
    elements.welcomePrimary.textContent = "选择一台在线设备，然后在下方输入命令。";
    elements.welcomeSecondary.textContent = "命令将在目标设备的本地 shell 中执行。";
    elements.changeToken.hidden = false;
    elements.enrollmentPanel.hidden = false;
    if (state.token) refreshClients();
    else openTokenDialog(false);
  } else {
    elements.targetsLabel.textContent = "SAME IP CLIENTS";
    elements.devicesTitle.textContent = "同 IP 在线设备";
    elements.emptyTitle.textContent = "暂无同 IP 设备";
    elements.emptyDescription.textContent = "请确认 Client 已连接当前 Server。";
    elements.welcomePrimary.textContent = "免密登录后，可以选择并控制当前来源 IP 下的任意在线 Client。";
    elements.welcomeSecondary.textContent = "共享代理或 NAT 下的 Client 会出现在同一个列表中。";
    initializeDeviceMode();
  }
  setInterval(() => {
    if (!state.busy && (state.adminMode ? state.token : state.authMode)) refreshClients();
  }, 10000);
  window.addEventListener("beforeunload", closeTerminal);

  async function initializeDeviceMode() {
    state.authMode = "";
    if (!await refreshClients(false)) {
      showLoginGate();
      return;
    }
    const client = state.clients.find((item) => item.id === state.selfID) || state.clients[0];
    if (client) location.assign(`/idata/?client=${encodeURIComponent(client.id)}`);
  }
})();
