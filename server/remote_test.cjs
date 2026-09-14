const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync(__dirname + '/internal/server/idataweb/remote.js', 'utf8');
(async () => {
 for (const [page, relativeBase, agent, api] of [
  ['http://arbitrary.example/', './', 'ws://arbitrary.example:80/ws/agent', 'http://arbitrary.example/'],
  ['https://another.example:8443/team/tools/?client=other-pc#view', './', 'wss://another.example:8443/team/tools/ws/agent', 'https://another.example:8443/team/tools/'],
  ['http://192.0.2.10:18080/', './', 'ws://192.0.2.10:18080/ws/agent', 'http://192.0.2.10:18080/'],
  ['http://[2001:db8::5]:8080/prefix/', './', 'ws://[2001:db8::5]:8080/prefix/ws/agent', 'http://[2001:db8::5]:8080/prefix/'],
  ['http://arbitrary.example/prefix/idata/', '../', 'ws://arbitrary.example:80/prefix/ws/agent', 'http://arbitrary.example/prefix/'],
 ]) {
  const pageURL = new URL(page), requests = [];
  const location = {href:page, hostname:pageURL.hostname, port:pageURL.port, protocol:pageURL.protocol, pathname:pageURL.pathname, search:pageURL.search, reload(){}};
  let actions;
  const window = {addEventListener(){}};
  const context = {URL, URLSearchParams, location, window, document:{querySelector(){return {content:relativeBase}}}, localStorage:{getItem(){return 'en'}}, history:{replaceState(){}}, ElementPlus:{}, setInterval(){}, setTimeout(){}, Vue:{reactive:x=>x, createApp(config){actions=config.setup(); return {use(){return this}, mount(){}}}},
   fetch:async (url) => {requests.push(url); return {ok:true,status:200,clone(){return this},json:async()=>({clients:[{id:'pc',capabilities:['idata_api_v1']}]})};}
  };
  vm.runInNewContext(source, context);
  actions.openWindowsClient();
  const link = new URL(location.href), q = link.searchParams;
  const host = q.get('server').includes(':') ? `[${q.get('server')}]` : q.get('server');
  const resolved = `${q.get('secure')==='1'?'wss':'ws'}://${host}:${q.get('port')}${q.get('path')||'/ws/agent'}`;
  assert.equal(resolved,agent);
  await window.idataFetch('/api/settings');
  assert(requests.includes(api+'api/v1/ip-login'));
  assert(requests.includes(api+'api/v1/self'));
  assert(requests.includes(api+'api/v1/clients/pc/idata/settings'));
 }
 console.log('Browser launch and API routing: 5 deployment cases passed.');
})().catch(error=>{console.error(error);process.exitCode=1});
