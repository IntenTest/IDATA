# Configurable URL deployment

## Address precedence

The browser launch link uses the page's current scheme, hostname, effective port,
and deployment prefix. It never uses a baked-in public server address. Page query
strings and fragments are excluded. A link from https://server.example:8443/team/
connects to wss://server.example:8443/team/ws/agent. Root HTTP URLs use port 80;
root HTTPS URLs use 443 unless a different port is explicit.

A browser launch overrides Client defaults and previously saved destinations,
including when forwarded to a running Client. Without a launch link, configure a
complete HTTP(S) or WS(S) URL in the Windows connection window, server_url in the
JSON configuration, IDATA_SERVER_URL, or --server. CLI options override environment
values, which override JSON values, which override the fallback. The fallback is
http://idata.test.huawei.com/; it is not an allowlist or a routing rule. There is
no IP-specific port detection. Bare hosts use HTTP port 80 unless they match a
previously configured endpoint. Use a full URL to choose TLS, a port, or a prefix.
The fixed local execution and launcher loopback ports are unrelated to the public
server URL and remain unchanged.

## Upgrade

Upgrade both the Windows Client and Linux server for this release. Keep Client
JSON configuration, tokens, and server device credentials. Exit the old Windows
Client, replace its EXE, and start it once to register the new executable location.
The EXE includes its offline execution runtime.

Copy the Linux archive to Ubuntu, extract it, verify SHA256SUMS, then run:

    sudo bash deploy-ubuntu.sh

The installer preserves IDATA_LISTEN_ADDR on upgrades. First installation defaults
to :12345 on any host. To explicitly choose another listener:

    sudo env IDATA_DEPLOY_LISTEN_ADDR=127.0.0.1:18080 bash deploy-ubuntu.sh

Existing configuration and credentials are backed up; service/health failures
restore the previous deployment. The backend can also be started directly with
--listen or configured using IDATA_LISTEN_ADDR. Public and upstream ports do not
need to match. Restrict the upstream listener to the intended network/proxy.

## Nginx root deployment

nginx-domain.conf.example is a template, not an application requirement. Replace
server.example and the upstream with your actual deployment values. The map goes
in the http context. Replace an existing server block rather than adding a
conflicting one. Keep the browser Host including its external port using $http_host.

The user's existing http://idata.test.huawei.com/ -> 10.90.65.189:12345 deployment
continues to work. Both the browser and Client must use the same proxy entry point.
No port should be added to the browser URL merely because the upstream uses one.

## Path-prefix deployment

For a public URL such as http://server.example/tools/idata/, use a directory URL
ending in /. Within the same Nginx server block, replace the root location with:

    location = /tools/idata {
        return 308 /tools/idata/;
    }
    location /tools/idata/ {
        proxy_pass http://127.0.0.1:12345/;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $idata_connection_upgrade;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_buffering off;
    }

The trailing slash in proxy_pass strips the public prefix before forwarding to
backend routes. The updated page preserves the external prefix for assets, API,
launch, enrollment, and polling. Prefix segments support ASCII letters, numbers,
and -._~; empty, dot, parent, or encoded separator segments are rejected by the
Client. Older root launch links remain accepted. Do not append arbitrary routes
or filenames to the deployment URL.

## HTTPS termination

When Nginx terminates HTTPS and forwards HTTP, set this in the server environment
file and restart the IDATA service (substitute the actual origin):

    IDATA_PUBLIC_ORIGIN=https://server.example:8443

Use the browser origin only, without a deployment path; omit default :443. Configure
Nginx TLS with your organization certificate and preserve Host using $http_host.
The server uses this explicit setting for origin checks and Secure cookies and
rejects other Host values. Client certificate verification remains enabled; the
execution PC must trust the certificate chain. This setting does not change any
launch link. Real PC IP handling is configured separately below. Leave it empty for
normal HTTP deployment. TLS certificates and Nginx installation are managed by
the existing deployment, not installed by the IDATA upgrade script.

## Verify and recover

Back up Nginx configuration before edits. Run sudo nginx -t, then reload Nginx only
if validation succeeds. To revert, restore the backup, validate, and reload again.
Check the public /healthz (under any prefix), launch the updated Client, confirm
an expected device, and exercise a harmless terminal command. /ws/agent must
upgrade with HTTP 101. HTTP and WebSocket requests share the same public origin.

Automated verification covers Client parsing/handoff, enrollment prefixes,
browser URL generation and API routing for DNS/IPv4/IPv6/custom ports/prefixes,
and proxy integration for HTTP, prefixed HTTP, and TLS-terminated HTTPS. The proxy
tests cover native WebSocket registration, cookie login, device discovery, workspace
API exchange, terminal opening, and cross-origin rejection. No live private-server
or Windows desktop test has been performed. Nginx itself is not installed locally;
the integration tests use Go's HTTP reverse proxy.

## PC isolation behind Nginx

Each user PC must have a distinct source IP as seen by Nginx, with one Client
per PC. The browser and Client use the same public gateway. Nginx supplies:

    proxy_set_header X-Real-IP $remote_addr;

Configure the backend with Nginx's own peer address (not the user PC addresses):

    IDATA_TRUSTED_PROXIES=127.0.0.1,::1

The example above applies when Nginx connects over loopback. If Nginx runs on the
same host but proxy_pass uses 10.90.65.189:12345, include 10.90.65.189. For that
layout the offline upgrade command is:

    sudo env IDATA_DEPLOY_TRUSTED_PROXIES=127.0.0.1,::1,10.90.65.189 bash deploy-ubuntu.sh

If Nginx is on another host, substitute the Nginx source address seen by the
backend. The installer preserves this setting on later upgrades unless the
IDATA_DEPLOY_TRUSTED_PROXIES override is supplied. Direct access without a reverse
proxy needs no setting. Address lists and CIDRs are supported; no proxy IP is
hard-coded into the application.

One source-IP adapter applies before all HTTP and WebSocket handlers. Device
lists, test operations, report reads, and terminal authorization use the same
effective PC IP. A browser can only reach the Client whose effective IP matches
its own. It cannot select another PC by changing a URL parameter. An offline
Client does not cause fallback to another PC. Multiple Clients on one effective
IP produce a conflict instead of an arbitrary selection. A duplicate Client ID
from another IP cannot replace an existing active Client; configure distinct
Client IDs if two PCs have identical hostnames.

Verification includes two simulated PCs sharing a reverse proxy: each sees only
its own Client and sends device queries, test requests, and report reads only to
that Client. Requests and terminal access to the other PC are refused. The test
also checks that disconnecting A does not expose B to A's browser. No separate
browser-to-Client binding or confirmation flow is required.
