let currentTab = localStorage.getItem('tab') || 'overview';
let charts = {};

function setTab(tab){
  currentTab = tab;
  localStorage.setItem('tab', tab);
  document.querySelectorAll('nav#tabs button').forEach(b=>b.classList.toggle('active', b.dataset.tab===tab));
  document.querySelectorAll('.tab').forEach(s=>s.classList.toggle('active', s.id==='tab-'+tab));
  loadTab(tab);
}

document.querySelectorAll('nav#tabs button').forEach(b=>b.addEventListener('click',()=>setTab(b.dataset.tab)));
document.getElementById('window').addEventListener('change',()=>loadTab(currentTab));
document.getElementById('refresh').addEventListener('click',()=>loadTab(currentTab));
document.getElementById('interval').addEventListener('input',e=>{document.getElementById('interval-val').textContent=e.target.value+'s'});

function poll(){
  if(document.getElementById('auto').checked && !document.hidden){
    loadTab(currentTab);
    loadHeader();
  }
}
setInterval(poll, parseInt(document.getElementById('interval').value)*1000);
document.getElementById('interval').addEventListener('change',()=>{
  // interval change handled via next poll
});

async function fetchJSON(url){
  const r = await fetch(url);
  return r.json();
}

async function loadHeader(){
  const sys = await fetchJSON('/api/system');
  const db = await fetchJSON('/api/db_size');
  const el = document.getElementById('header-metrics');
  if(sys && db){
    el.innerHTML = `<div>Host ${sys.hostname}</div><div>Uptime ${Math.floor(sys.uptime/3600)}h</div><div>DB ${db.size}</div><div>RAM ${sys.ram_gb?.toFixed(1)}GB</div>`;
  }
}

function mkTable(rows, cols){
  if(!rows.length) return '<p>Sem dados</p>';
  let h='<table><tr>'+cols.map(c=>`<th>${c}</th>`).join('')+'</tr>';
  rows.forEach(r=>{
    h+='<tr>'+cols.map(c=>`<td>${r[c]!==undefined && r[c]!==null ? r[c] : ''}</td>`).join('')+'</tr>';
  });
  return h+'</table>';
}

function chart(id, data, label, xKey='ts', yKey='value'){
  const ctx = document.getElementById(id);
  if(!ctx) return;
  if(charts[id]) charts[id].destroy();
  charts[id] = new Chart(ctx, {
    type:'line',
    data:{labels:data.map(d=>new Date(d[xKey]).toLocaleTimeString()), datasets:[{label, data:data.map(d=>d[yKey]), borderColor:'#2e7ff1', tension:0.2, pointRadius:0}]},
    options:{responsive:true, animation:false, scales:{y:{beginAtZero:false}}, plugins:{decimation:{enabled:true}}}
  });
}

async function loadTab(tab){
  const win = document.getElementById('window').value;
  const el = document.getElementById('tab-'+tab);
  if(!el) return;
  el.innerHTML = '<p>Carregando...</p>';
  try{
    if(tab==='overview'){
      const [cpu,mem,gpu,disk,net] = await Promise.all([
        fetchJSON('/api/cpu?window='+encodeURIComponent(win)),
        fetchJSON('/api/memory?window='+encodeURIComponent(win)),
        fetchJSON('/api/gpu?window='+encodeURIComponent(win)),
        fetchJSON('/api/disk/usage'),
        fetchJSON('/api/net/latest')
      ]);
      el.innerHTML = `<div class="grid">
        <div class="card"><h3>CPU %</h3><canvas id="ov-cpu"></canvas></div>
        <div class="card"><h3>Memória %</h3><canvas id="ov-mem"></canvas></div>
        <div class="card"><h3>GPU Temp/Util</h3><canvas id="ov-gpu"></canvas></div>
        <div class="card"><h3>Disco uso %</h3><canvas id="ov-disk"></canvas></div>
      </div><div id="ov-net"></div>`;
      if(cpu.length) chart('ov-cpu', cpu, 'CPU %', 'ts', 'cpu');
      if(mem.length) chart('ov-mem', mem, 'Mem %', 'ts', 'used_percent');
      if(gpu.length){
        const ctx=document.getElementById('ov-gpu');
        if(ctx){
          if(charts['ov-gpu']) charts['ov-gpu'].destroy();
          charts['ov-gpu']= new Chart(ctx, {type:'line', data:{labels:gpu.map(d=>new Date(d.ts).toLocaleTimeString()), datasets:[{label:'Temp C', data:gpu.map(d=>d.temp), borderColor:'#ef4444', yAxisID:'y'},{label:'Util %', data:gpu.map(d=>d.util), borderColor:'#22c55e', yAxisID:'y1'}]}, options:{responsive:true, animation:false, scales:{y:{type:'linear',position:'left', title:{display:true,text:'Temp C'}}, y1:{type:'linear',position:'right', grid:{drawOnChartArea:false}, title:{display:true,text:'Util %'}}}}});
        }
      }
      if(disk.length){
        const ctx=document.getElementById('ov-disk');
        if(ctx){
          if(charts['ov-disk']) charts['ov-disk'].destroy();
          charts['ov-disk']= new Chart(ctx, {type:'bar', data:{labels:disk.map(d=>d.device), datasets:[{label:'usado %', data:disk.map(d=>d.used_percent), backgroundColor:'#2e7ff1'}]}, options:{responsive:true, animation:false}});
        }
      }
      if(net.length){
        document.getElementById('ov-net').innerHTML = '<div class="card"><h3>Rede (total MB)</h3>'+mkTable(net.map(r=>({iface:r.iface, recv: (r.recv/1024/1024).toFixed(1), sent:(r.sent/1024/1024).toFixed(1)})), ['iface','recv','sent'])+'</div>';
      }
    } else if(tab==='cpu'){
      const [cpu, temps] = await Promise.all([fetchJSON('/api/cpu?window='+encodeURIComponent(win)), fetchJSON('/api/sensors/cpu_temps?window='+encodeURIComponent(win))]);
      el.innerHTML = `<div class="card"><h3>CPU %</h3><canvas id="cpu-cpu"></canvas></div><div class="card"><h3>Frequência MHz</h3><canvas id="cpu-freq"></canvas></div><div id="cpu-temps"></div>`;
      if(cpu.length){
        chart('cpu-cpu', cpu, 'CPU %', 'ts', 'cpu');
        // freq
        setTimeout(()=>{
          const ctx=document.getElementById('cpu-freq');
          if(ctx){
            if(charts['cpu-freq']) charts['cpu-freq'].destroy();
            charts['cpu-freq']= new Chart(ctx, {type:'line', data:{labels:cpu.map(d=>new Date(d.ts).toLocaleTimeString()), datasets:[{label:'Freq MHz', data:cpu.map(d=>d.freq), borderColor:'#a855f7'}]}, options:{responsive:true, animation:false}});
          }
        },0);
      }
      if(temps.length){
        // group by name
        const groups={};
        temps.forEach(t=>{ (groups[t.name]=groups[t.name]||[]).push(t); });
        let html='';
        Object.keys(groups).slice(0,4).forEach(name=>{
          const id='cpu-temp-'+name.replace(/[^a-z0-9]/gi,'_');
          html+=`<div class="card"><h3>${name.split(':').pop()}</h3><canvas id="${id}"></canvas></div>`;
        });
        document.getElementById('cpu-temps').innerHTML=html;
        Object.keys(groups).slice(0,4).forEach(name=>{
          const id='cpu-temp-'+name.replace(/[^a-z0-9]/gi,'_');
          chart(id, groups[name], '°C', 'ts', 'value');
        });
      }
    } else if(tab==='memory'){
      const mem = await fetchJSON('/api/memory?window='+encodeURIComponent(win));
      el.innerHTML='<div class="card"><h3>Memória %</h3><canvas id="mem-p"></canvas></div><div class="card"><h3>Usada GB</h3><canvas id="mem-gb"></canvas></div>';
      if(mem.length){
        chart('mem-p', mem, 'used %', 'ts', 'used_percent');
        setTimeout(()=>chart('mem-gb', mem, 'GB', 'ts', 'used_gb'),0);
      }
    } else if(tab==='gpu'){
      const gpu = await fetchJSON('/api/gpu?window='+encodeURIComponent(win));
      el.innerHTML='<div class="card"><h3>GPU Temp</h3><canvas id="gpu-temp"></canvas></div><div class="card"><h3>GPU Util</h3><canvas id="gpu-util"></canvas></div><div class="card"><h3>Power W</h3><canvas id="gpu-power"></canvas></div>';
      if(gpu.length){
        chart('gpu-temp', gpu, 'Temp C', 'ts', 'temp');
        setTimeout(()=>{
          chart('gpu-util', gpu, 'Util %', 'ts', 'util');
          chart('gpu-power', gpu, 'Power', 'ts', 'power');
        },0);
      }
    } else if(tab==='sensors'){
      const latest = await fetchJSON('/api/sensors/latest');
      el.innerHTML = '<div class="card"><h3>Sensores (latest 30)</h3>'+mkTable(latest.slice(0,30).map(r=>({name:r.name.slice(0,50), type:r.type, value:r.value, unit:r.unit})), ['name','type','value','unit'])+'</div>';
    } else if(tab==='disk'){
      const [usage, phys, smart] = await Promise.all([fetchJSON('/api/disk/usage'), fetchJSON('/api/disk/physical'), fetchJSON('/api/disk/smart')]);
      el.innerHTML = `<div class="card"><h3>Volumes</h3>${mkTable(usage.map(r=>({device:r.device, mount:r.mount, used:r.used_percent, free:r.free_gb?.toFixed(1)})), ['device','mount','used','free'])}</div>
        <div class="card"><h3>Físicos</h3>${mkTable(phys.map(r=>({id:r.device_id, name:r.friendly_name, type:r.media_type, bus:r.bus_type, health:r.health, size:Math.round(r.size_gb)})), ['id','name','type','bus','health','size'])}</div>
        <div class="card"><h3>SMART</h3>${mkTable(smart.map(r=>({dev:r.device, model:r.model, temp:r.temp, poh:r.poh, wear:r.wear, passed:r.passed})), ['dev','model','temp','poh','wear','passed'])}</div><div class="card"><h3>SMART Temperatura</h3><canvas id="disk-temp"></canvas></div>`;
      if(smart.length){
        // fetch history for chart
        const hist = await fetchJSON('/api/disk/smart/history?window='+encodeURIComponent(win));
        // hist not implemented as endpoint, use smart history via direct? For now skip
      }
    } else if(tab==='net'){
      const [latest, hist] = await Promise.all([fetchJSON('/api/net/latest'), fetchJSON('/api/net?window='+encodeURIComponent(win))]);
      el.innerHTML = '<div class="card"><h3>Rede total MB</h3>'+mkTable(latest.map(r=>({iface:r.iface, recv:(r.recv/1024/1024).toFixed(1), sent:(r.sent/1024/1024).toFixed(1)})), ['iface','recv','sent'])+'</div><div class="card"><h3>Throughput KB/s</h3><canvas id="net-kbs"></canvas></div>';
      if(hist.length){
        // compute delta per iface simple: group
        const byIface={};
        hist.forEach(r=>{ (byIface[r.iface]=byIface[r.iface]||[]).push(r); });
        const iface = Object.keys(byIface).find(k=>k==='Wi-Fi') || Object.keys(byIface)[0];
        if(iface){
          const arr = byIface[iface].sort((a,b)=>new Date(a.ts)-new Date(b.ts));
          const withDelta = arr.map((v,i)=> i===0? null : {...v, recv_kbs:(v.recv - arr[i-1].recv)/1024/10, sent_kbs:(v.sent - arr[i-1].sent)/1024/10}).filter(Boolean);
          chart('net-kbs', withDelta, 'RX KB/s', 'ts', 'recv_kbs');
        }
      }
    } else if(tab==='processes'){
      const procs = await fetchJSON('/api/processes');
      el.innerHTML = '<div class="card"><h3>Top 15 CPU</h3>'+mkTable(procs.map(r=>({name:r.name, pid:r.pid, cpu:r.cpu?.toFixed(1), mem:r.mem?.toFixed(1), rss:r.rss_mb?.toFixed(0)})), ['name','pid','cpu','mem','rss'])+'</div>';
    } else if(tab==='system'){
      const sys = await fetchJSON('/api/system');
      const hb = await fetchJSON('/api/heartbeat');
      el.innerHTML = `<div class="card"><pre>${JSON.stringify(sys,null,2)}</pre></div><div class="card"><h3>Heartbeat</h3>${mkTable(hb.map(r=>({collector:r.collector, success:r.success, ts:new Date(r.ts).toLocaleTimeString()})), ['collector','success','ts'])}</div>`;
    }
  } catch(e){
    el.innerHTML = '<p style="color:#f87171">Erro: '+e+'</p>';
    console.error(e);
  }
}

// init
setTab(currentTab);
loadHeader();
loadTab(currentTab);
