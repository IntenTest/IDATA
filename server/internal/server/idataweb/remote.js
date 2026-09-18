(() => {
  "use strict";

  const apiBase = new URL(document.querySelector('meta[name="idata-api-base"]').content, location.href);
  const endpoint = (path) => new URL(path.replace(/^\//, ""), apiBase).toString();
  const params = new URLSearchParams(location.search);
  const connection = Vue.reactive({
    visible: false,
    downloadVisible: false,
    message: "Looking for the IDATA Client…",
    language: localStorage.getItem("idata-language") === "en" ? "en" : "zh-CN",
    browserIP: "",
    clientIP: "",
    proxyIP: "",
  });
  window.IDATAConnectionInfo = connection;
  const translations = {
  "Multiple Clients use this PC address. Keep one Client running on this PC.": "检测到同一台 PC 地址有多个客户端，请只保留一个客户端运行。",
  "The configured proxy must supply one valid X-Real-IP value.": "服务器代理未正确传递本机 IP，请检查代理配置。",
  "The reverse proxy is not trusted. Device matching was blocked to prevent cross-PC access.": "服务器尚未信任当前反向代理。为防止访问到其他电脑，已停止设备匹配。请将代理 IP 加入 IDATA_TRUSTED_PROXIES。",
  "First-time setup": "首次使用须知",
  "Download every .exe file from the latest release into the same folder. Double-click IDATA-Client.exe once to register the browser launcher, then return to this page and select Open IDATA Client. Keep all downloaded files together.": "首次使用时，请下载最新版本下的所有 .exe 文件，并将它们保存到同一文件夹。下载完成后，请先双击运行一次 IDATA-Client.exe，完成浏览器启动方式的注册，再返回本页点击“已下载，启动客户端”。请勿将这些文件分开放置。",
  "Understood — open the IDATA download page": "已知晓，前往 IDATA 华为内源下载页",
  "Connect IDATA Client": "连接 IDATA 客户端",
  "Open IDATA Client": "已下载，启动客户端",
  "Client started — refresh connection": "已启动客户端，刷新页面",
  "Download IDATA Client": "新用户，下载客户端",
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
  "The server connection was interrupted. Reopen IDATA Client if needed.": "服务器连接已中断，请检查网络，必要时重新启动 IDATA 客户端。",
  "Current web access IP": "当前网页接入 IP",
  "Matched Client connection IP": "匹配的 Client 接入 IP",
  "Reverse proxy IP": "反向代理 IP",
  "Not connected": "尚未连接",
  "IP matching is confirmed.": "网页 IP 与 Client IP 匹配成功。",
  "Waiting for a Client from this IP.": "正在等待同一 IP 的 Client 接入。",
  "IP mismatch; access is blocked.": "IP 不一致，已阻止访问。"
};
  const t = (message) => connection.language === "zh-CN"
    ? translations[message] || translations["The IDATA Client connection is unavailable."]
    : message;
  window.addEventListener("idata-language-change", (event) => {
    connection.language = event.detail === "en" ? "en" : "zh-CN";
  });

  Vue.createApp({
    setup() {
      return { connection, openWindowsClient, openIDATAClientDownload, confirmIDATAClientDownload, refreshConnection, t };
    },
    template: `
      <el-dialog
        v-model="connection.visible"
        :title="t('Connect IDATA Client')"
        width="680px"
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
        <div class="remote-ip-summary">
          <div>
            <span>{{ t('Current web access IP') }}</span>
            <strong>{{ connection.browserIP || '—' }}</strong>
          </div>
          <div>
            <span>{{ t('Matched Client connection IP') }}</span>
            <strong>{{ connection.clientIP || t('Not connected') }}</strong>
          </div>
          <div v-if="connection.proxyIP">
            <span>{{ t('Reverse proxy IP') }}</span>
            <strong>{{ connection.proxyIP }}</strong>
          </div>
          <el-tag
            :type="connection.clientIP && connection.clientIP === connection.browserIP ? 'success' : connection.clientIP ? 'danger' : 'info'"
            effect="light"
          >
            {{ t(connection.clientIP && connection.clientIP === connection.browserIP
              ? 'IP matching is confirmed.'
              : connection.clientIP
                ? 'IP mismatch; access is blocked.'
                : 'Waiting for a Client from this IP.') }}
          </el-tag>
        </div>
        <template #footer>
          <div class="remote-connection-actions">
            <el-button type="primary" @click="openIDATAClientDownload">{{ t('Download IDATA Client') }}</el-button>
            <el-button type="primary" @click="openWindowsClient">{{ t('Open IDATA Client') }}</el-button>
            <el-button type="primary" @click="refreshConnection">{{ t('Client started — refresh connection') }}</el-button>
          </div>
        </template>
      </el-dialog>
      <el-dialog v-model="connection.downloadVisible" :title="t('First-time setup')" width="560px" align-center class="remote-connection-dialog">
        <p class="remote-connection-description">{{ t('Download every .exe file from the latest release into the same folder. Double-click IDATA-Client.exe once to register the browser launcher, then return to this page and select Open IDATA Client. Keep all downloaded files together.') }}</p>
        <template #footer>
          <el-button type="primary" @click="confirmIDATAClientDownload">{{ t('Understood — open the IDATA download page') }}</el-button>
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

  function updateConnectionInfo(result) {
    if (!result || typeof result !== "object") return;
    if (typeof result.browser_ip === "string" && result.browser_ip) {
      connection.browserIP = result.browser_ip;
    }
    connection.clientIP = typeof result.client_ip === "string" ? result.client_ip : "";
    connection.proxyIP = typeof result.proxy_ip === "string" ? result.proxy_ip : "";
  }

  function setConnected(clientID) {
    state.client = clientID;
    params.delete("client");
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
    const agentPath = new URL("ws/agent", apiBase).pathname;
    if (agentPath !== "/ws/agent") target.searchParams.set("path", agentPath);
    return target.toString();
  }

  async function findClient() {
    while (!state.stopped) {
      try {
        const login = await fetch(endpoint("api/v1/ip-login"), { method: "POST", cache: "no-store" });
        const loginResult = await login.clone().json().catch(() => ({}));
        updateConnectionInfo(loginResult);
        if (!login.ok && login.status !== 202) {
          throw new Error(loginResult.error || "The server could not authorize this browser.");
        }
        if (login.status === 202) {
          state.observedDisconnected = true;
          showConnectionDialog("IDATA Client is not connected. Open it to continue.");
          await delay(1000);
          continue;
        }

        const response = await fetch(endpoint("api/v1/self"), { cache: "no-store" });
        if (!response.ok) throw new Error("The IDATA Client connection is not ready.");
        const result = await response.json();
        updateConnectionInfo(result);
        const clients = result.clients || [];
        const selected = clients.length === 1 ? clients[0] : null;
        if (!selected) throw new Error("No IDATA execution client is available.");
        if (!(selected.capabilities || []).includes("server_commands_v1") ||
            !(selected.capabilities || []).includes("command_stdin_v1")) {
          throw new Error("Update and restart IDATA Client 0.7.14 on this computer.");
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
    connection.clientIP = "";
    void connect(message || "The IDATA Client disconnected. Reopen it to continue.");
  }

  function openWindowsClient() {
    showConnectionDialog("Opening IDATA Client and waiting for it to connect…");
    location.href = launchURL();
  }

  function openIDATAClientDownload() {
    connection.downloadVisible = true;
  }

  function confirmIDATAClientDownload() {
    window.open("https://openx.huawei.com/IDATA/download", "_blank", "popup=yes,width=1200,height=850,noopener,noreferrer");
    connection.downloadVisible = false;
  }

  window.idataFetch = async (path, options = {}) => {
    const client = await connect("Connect the IDATA Client to continue.");
    if (!path.startsWith("/api/")) throw new Error("Unsupported IDATA operation.");
    let response;
    try {
      response = await fetch(
        endpoint(`api/v1/clients/${encodeURIComponent(client)}/idata/${path.slice(5)}`),
        { ...options, cache: "no-store" },
      );
    } catch (error) {
      connectionLost("The server connection was interrupted. Reopen IDATA Client if needed.");
      throw error;
    }
    if ([401, 403, 409, 503].includes(response.status)) {
      const data = await response.clone().json().catch(() => ({}));
      connectionLost(data.error || "The IDATA Client connection is unavailable.");
    }
    return response;
  };

  void connect("", false);
  setInterval(async () => {
    if (!state.client || state.connectionPromise) return;
    try {
      const response = await fetch(endpoint("api/v1/self"), { cache: "no-store" });
      if (!response.ok) return connectionLost("The IDATA Client disconnected. Reopen it to continue.");
      const result = await response.json();
      updateConnectionInfo(result);
      if (!(result.clients || []).some((item) => item.id === state.client)) {
        connectionLost("The IDATA Client disconnected. Reopen it to continue.");
      }
    } catch (_) {
      connectionLost("The server connection was interrupted. Reopen IDATA Client if needed.");
    }
  }, 5000);
  window.addEventListener("beforeunload", () => { state.stopped = true; });
})();
