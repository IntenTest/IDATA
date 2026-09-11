(() => {
  "use strict";

  const params = new URLSearchParams(location.search);
  const preferredClient = params.get("client") || "";
  const connection = Vue.reactive({
    visible: false,
    message: "Looking for the IDATA Client…",
    language: localStorage.getItem("idata-language") === "en" ? "en" : "zh-CN",
  });
  const translations = {
  "Connect IDATA Client": "连接 IDATA 客户端",
  "Open IDATA Client": "启动 IDATA 客户端",
  "Client started — refresh connection": "已启动客户端，刷新连接",
  "Open IDATA Client on this computer to access devices and run tests. This page will refresh automatically when the client connects.": "请启动本机的 IDATA 客户端，以访问设备并运行测试。检测到接入后，页面将自动刷新。",
  "Looking for the IDATA Client…": "正在检测 IDATA 客户端接入状态…",
  "IDATA Client is not connected. Open it to continue.": "尚未检测到 IDATA 客户端接入，请点击下方按钮启动客户端。",
  "The server could not authorize this browser.": "服务器无法授权此浏览器，请稍后重试。",
  "The IDATA Client connection is not ready.": "IDATA 客户端尚未完成连接，请稍候。",
  "No IDATA execution client is available.": "未找到可用的 IDATA 执行客户端。",
  "Update and restart the IDATA Client on this computer.": "请更新并重启本机的 IDATA 客户端。",
  "The IDATA Client connection is unavailable.": "IDATA 客户端连接不可用，请重新启动客户端。",
  "The IDATA Client disconnected. Reopen it to continue.": "IDATA 客户端已断开，请重新启动客户端以继续。",
  "Opening IDATA Client and waiting for it to connect…": "正在启动 IDATA 客户端，等待接入…",
  "Connect the IDATA Client to continue.": "请连接 IDATA 客户端以继续。",
  "The server connection was interrupted. Reopen IDATA Client if needed.": "服务器连接已中断，请检查网络，必要时重新启动 IDATA 客户端。"
};
  const t = (message) => connection.language === "zh-CN"
    ? translations[message] || translations["The IDATA Client connection is unavailable."]
    : message;
  window.addEventListener("idata-language-change", (event) => {
    connection.language = event.detail === "en" ? "en" : "zh-CN";
  });

  Vue.createApp({
    setup() {
      return { connection, openWindowsClient, refreshConnection, t };
    },
    template: `
      <el-dialog
        v-model="connection.visible"
        :title="t('Connect IDATA Client')"
        width="520px"
        :show-close="false"
        :close-on-click-modal="false"
        :close-on-press-escape="false"
        :before-close="() => {}"
        align-center
        class="remote-connection-dialog"
      >
        <p class="remote-connection-description">
          {{ t('Open IDATA Client on this computer to access devices and run tests. This page will refresh automatically when the client connects.') }}
        </p>
        <el-alert :title="t(connection.message)" type="info" :closable="false" show-icon />
        <template #footer>
          <el-button @click="refreshConnection">{{ t('Client started — refresh connection') }}</el-button>
          <el-button type="primary" @click="openWindowsClient">{{ t('Open IDATA Client') }}</el-button>
        </template>
      </el-dialog>
    `,
  }).use(ElementPlus).mount("#remote-connection");
  const state = { client: "", connectionPromise: null, stopped: false, observedDisconnected: false };

  const delay = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds));

  function showConnectionDialog(message) {
    connection.message = message || "Looking for the IDATA Client…";
    connection.visible = true;
  }

  function setConnected(clientID) {
    state.client = clientID;
    params.set("client", clientID);
    history.replaceState({}, "", `${location.pathname}?${params.toString()}`);
    connection.visible = false;
    // Refresh only after an observed disconnection, never on every page load.
    if (state.observedDisconnected) {
      state.stopped = true;
      refreshConnection();
    }
  }

  function refreshConnection() {
    location.reload();
  }

  function launchURL() {
    const target = new URL("idata://connect");
    target.searchParams.set("server", location.hostname.replace(/^\[|\]$/g, ""));
    target.searchParams.set("port", location.port || (location.protocol === "https:" ? "443" : "80"));
    target.searchParams.set("secure", location.protocol === "https:" ? "1" : "0");
    return target.toString();
  }

  async function findClient() {
    while (!state.stopped) {
      try {
        const login = await fetch("/api/v1/ip-login", { method: "POST", cache: "no-store" });
        const loginResult = await login.clone().json().catch(() => ({}));
        if (!login.ok && login.status !== 202) {
          throw new Error(loginResult.error || "The server could not authorize this browser.");
        }
        if (login.status === 202) {
          state.observedDisconnected = true;
          showConnectionDialog("IDATA Client is not connected. Open it to continue.");
          await delay(1000);
          continue;
        }

        const response = await fetch("/api/v1/self", { cache: "no-store" });
        if (!response.ok) throw new Error("The IDATA Client connection is not ready.");
        const result = await response.json();
        const clients = result.clients || [];
        const selected = clients.find((item) => item.id === preferredClient)
          || clients.find((item) => item.id === result.self_client_id)
          || clients[0];
        if (!selected) throw new Error("No IDATA execution client is available.");
        if (!(selected.capabilities || []).includes("idata_api_v1")) {
          throw new Error("Update and restart the IDATA Client on this computer.");
        }
        setConnected(selected.id);
        return selected.id;
      } catch (error) {
        state.observedDisconnected = true;
        showConnectionDialog(error.message || "The IDATA Client connection is unavailable.");
        await delay(1500);
      }
    }
    throw new Error("IDATA Client connection stopped.");
  }

  function connect(message, showImmediately = true) {
    if (state.client) return Promise.resolve(state.client);
    if (!state.connectionPromise) {
      if (showImmediately) showConnectionDialog(message);
      state.connectionPromise = findClient().finally(() => { state.connectionPromise = null; });
    } else if (showImmediately && connection.visible && message) {
      connection.message = message;
    }
    return state.connectionPromise;
  }

  function connectionLost(message) {
    state.observedDisconnected = true;
    state.client = "";
    void connect(message || "The IDATA Client disconnected. Reopen it to continue.");
  }

  function openWindowsClient() {
    showConnectionDialog("Opening IDATA Client and waiting for it to connect…");
    location.href = launchURL();
  }

  window.idataFetch = async (path, options = {}) => {
    const client = await connect("Connect the IDATA Client to continue.");
    if (!path.startsWith("/api/")) throw new Error("Unsupported IDATA operation.");
    let response;
    try {
      response = await fetch(
        `/api/v1/clients/${encodeURIComponent(client)}/idata/${path.slice(5)}`,
        { ...options, cache: "no-store" },
      );
    } catch (error) {
      connectionLost("The server connection was interrupted. Reopen IDATA Client if needed.");
      throw error;
    }
    if ([401, 403, 409, 502, 503].includes(response.status)) {
      const data = await response.clone().json().catch(() => ({}));
      connectionLost(data.error || "The IDATA Client connection is unavailable.");
    }
    return response;
  };

  void connect("", false);
  setInterval(async () => {
    if (!state.client || state.connectionPromise) return;
    try {
      const response = await fetch("/api/v1/self", { cache: "no-store" });
      if (!response.ok) return connectionLost("The IDATA Client disconnected. Reopen it to continue.");
      const result = await response.json();
      if (!(result.clients || []).some((item) => item.id === state.client)) {
        connectionLost("The IDATA Client disconnected. Reopen it to continue.");
      }
    } catch (_) {
      connectionLost("The server connection was interrupted. Reopen IDATA Client if needed.");
    }
  }, 5000);
  window.addEventListener("beforeunload", () => { state.stopped = true; });
})();
