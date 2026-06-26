package main

import "net/http"

// uiHTML is a single-page engineering console: it polls the read-only status,
// group, job-list and history APIs. Not an end-user product — a probe + API
// validation surface.
const uiHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>gua console</title>
<style>
 body{font:13px/1.4 monospace;margin:16px;background:#0b0e14;color:#c9d1d9}
 h1{font-size:16px} h2{font-size:13px;color:#7ee787;margin:16px 0 6px}
 table{border-collapse:collapse;width:100%;margin-bottom:8px}
 td,th{border:1px solid #2d333b;padding:3px 6px;text-align:left;font-size:12px}
 th{background:#161b22;color:#8b949e}
 .ok{color:#3fb950} .bad{color:#f85149} .muted{color:#8b949e}
 input,button{font:12px monospace;background:#161b22;color:#c9d1d9;border:1px solid #2d333b;padding:3px 6px}
 #bar{margin-bottom:10px}
</style></head><body>
<h1>gua console <span class="muted" id="ver"></span></h1>
<div id="bar">group: <input id="grp" placeholder="group name" size="16">
 <button onclick="load()">load</button>
 <button onclick="toggle()">auto-refresh: <span id="ar">off</span></button></div>
<h2>cluster / queue</h2><div id="status"></div>
<h2>jobs</h2><div id="jobs"></div>
<h2>recent executions</h2><div id="hist"></div>
<script>
let timer=null;
const esc=s=>String(s==null?'':s).replace(/[&<>]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]));
const ts=u=>u?new Date(u*1000).toISOString().replace('T',' ').replace('.000Z',''):'-';
async function j(u){const r=await fetch(u);if(!r.ok)throw new Error(u+' '+r.status);return r.json();}
function table(rows,cols){if(!rows||!rows.length)return '<div class="muted">(none)</div>';
 let h='<table><tr>'+cols.map(c=>'<th>'+c[0]+'</th>').join('')+'</tr>';
 for(const x of rows)h+='<tr>'+cols.map(c=>'<td>'+c[1](x)+'</td>').join('')+'</tr>';return h+'</table>';}
async function load(){
 const g=document.getElementById('grp').value.trim();
 try{document.getElementById('ver').textContent=await j('/version');}catch(e){}
 try{const s=await j('/v1/status');
  document.getElementById('status').innerHTML=
   '<div>ready_queue_depth: <b>'+s.ready_queue_depth+'</b> &nbsp; down_server_backlog: <b>'+s.down_server_backlog+'</b></div>'+
   table(s.servers,[['slot',x=>esc(x.name)],['last_heartbeat',x=>ts(x.last_heartbeat)],
    ['alive',x=>x.alive?'<span class=ok>yes</span>':'<span class=bad>no</span>']]);
 }catch(e){document.getElementById('status').innerHTML='<span class=bad>'+esc(e.message)+'</span>';}
 if(!g){document.getElementById('jobs').innerHTML='<div class=muted>enter a group</div>';document.getElementById('hist').innerHTML='';return;}
 try{const jobs=await j('/v1/groups/'+encodeURIComponent(g)+'/jobs');
  document.getElementById('jobs').innerHTML=table(jobs,[['id',x=>esc(x.id)],['name',x=>esc(x.name)],
   ['next_exec',x=>ts(x.exec_time)],['interval',x=>esc(x.interval_pattern)],['url',x=>esc(x.request_url)],
   ['active',x=>x.active?'<span class=ok>on</span>':'<span class=muted>paused</span>']]);
 }catch(e){document.getElementById('jobs').innerHTML='<span class=bad>'+esc(e.message)+'</span>';}
 try{const h=await j('/v1/groups/'+encodeURIComponent(g)+'/history?limit=100');
  document.getElementById('hist').innerHTML=table(h,[['job_id',x=>esc(x.job_id)],['type',x=>esc(x.type)],
   ['exec',x=>ts(x.exec_time)],['finish',x=>ts(x.finish_time)],
   ['result',x=>x.success?'<span class=ok>ok</span>':'<span class=bad>fail</span>'],
   ['detail',x=>esc(x.error||x.message)],['host',x=>esc(x.exec_machine_host)]]);
 }catch(e){document.getElementById('hist').innerHTML='<span class=bad>'+esc(e.message)+'</span>';}
}
function toggle(){if(timer){clearInterval(timer);timer=null;document.getElementById('ar').textContent='off';}
 else{timer=setInterval(load,3000);document.getElementById('ar').textContent='on';load();}}
load();
</script></body></html>`

func UI() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(uiHTML))
	}
}
