(() => {
  "use strict";
  const params = new URLSearchParams(location.search);
  const client = params.get("client") || "";
  const admin = params.get("mode") === "admin";
  const bar = document.getElementById("remote-controls");
  bar.style.cssText = "display:flex;gap:20px;align-items:center;padding:12px 24px;background:#14263d;color:white;font:14px system-ui";
  const label = document.createElement("strong");
  label.textContent = client ? `Execution PC: ${client}` : "Select an execution PC to use IDATA.";
  const link = document.createElement("a");
  link.href = admin ? "/admin/" : "/";
  link.textContent = "Change PC / Connection console";
  link.style.color = "#8edaff";
  bar.append(label, link);
  window.idataFetch = async (path, options = {}) => {
    if (!client) throw new Error("Select an execution PC in the connection console.");
    if (!path.startsWith("/api/")) throw new Error("Unsupported IDATA operation.");
    const headers = new Headers(options.headers || {});
    if (admin) headers.set("Authorization", `Bearer ${sessionStorage.getItem("idata_admin_token") || ""}`);
    const response = await fetch(`/api/v1/clients/${encodeURIComponent(client)}/idata/${path.slice(5)}`, {...options, headers, cache: "no-store"});
    if ([401, 403, 409, 502, 503].includes(response.status)) {
      const data = await response.clone().json().catch(() => ({}));
      label.textContent = `${client}: ${data.error || "Connection unavailable"}`;
    } else if (response.ok) label.textContent = `Execution PC: ${client}`;
    return response;
  };
})();
