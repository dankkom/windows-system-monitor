const S = {
  tab: localStorage.getItem('tab') || 'overview',
  win: localStorage.getItem('win') || '1 hour',
  intervalSec: parseInt(localStorage.getItem('interval') || '10', 10),
  timer: null,
  charts: {},
  mounted: new Set(),
  abort: null,
};

const windowTabs = new Set(['overview','cpu','memory','gpu','sensors','disk','net']);
const els = {};

function init(){
  els.tabs = document.querySelectorAll('nav#tabs button');
  els.panels = document.querySelectorAll('.tab');
  els.win = document.getElementById('window');
  els.winHint = document.getElementById('window-hint');
  els.auto = document.getElementById('auto');
  els.interval = document.getElementById('interval');
  els.intervalVal = document.getElementById('interval-val');
  els.refresh = document.getElementById('refresh');
  els.lastUpdated = document.getElementById('last-updated');
  els.header = document.getElementById('header-metrics');
  els.footerDb = document.getElementById('footer-db');

  els.win.value = S.win;
  els.interval.value = String(S.intervalSec);
  els.intervalVal.textContent = S.intervalSec + 's';

  els.tabs.forEach(b=> b.addEventListener('click', ()=> setTab(b.dataset.tab)));
  els.win.addEventListener('change', ()=>{
    S.win = els.win.value;
    localStorage.setItem('win', S.win);
    updateCurrent({force:true});
  });
  els.refresh.addEventListener('click', ()=> updateCurrent({force:true}));
  els.auto.addEventListener('change', ()=> { if(els.auto.checked) schedule(); else unschedule(); });
  els.interval.addEventListener('input', e=>{
    S.intervalSec = parseInt(e.target.value,10);
    els.intervalVal.textContent = S.intervalSec + 's';
    localStorage.setItem('interval', String(S.intervalSec));
  });
  els.interval.addEventListener('change', ()=> { if(els.auto.checked) schedule(); });

  document.addEventListener('visibilitychange', ()=>{
    if(document.hidden) unschedule();
    else if(els.auto.checked) schedule();
  });

  setTab(S.tab, {forceMount:true});
  loadHeader();
  schedule();
}

function setTab(tab, opts={}){
  S.tab = tab;
  localStorage.setItem('tab', tab);
  const isWindowTab = windowTabs.has(tab);
  els.win.disabled = !isWindowTab;
  els.winHint.hidden = isWindowTab;
  els.tabs.forEach(b=>{
    const active = b.dataset.tab===tab;
    b.classList.toggle('active', active);
    b.setAttribute('aria-selected', String(active));
  });
  els.panels.forEach(p=>{
    const active = p.id==='tab-'+tab;
    p.classList.toggle('active', active);
    p.hidden = !active;
  });
  ensureMounted(tab);
  updateCurrent(opts);
}

function ensureMounted(tab){
  if(S.mounted.has(tab)) return;
  const el = document.getElementById('tab-'+tab);
  if(!el) return;
  // skeleton flat — sem caixas, só divisores
  if(tab==='overview'){
    el.innerHTML = `
      <div class="section"><h2>Histórico</h2><div class="grid">
        <div><h3>CPU %</h3><div class="chart-wrap"><canvas id="ov-cpu"></canvas></div></div>
        <div><h3>Memória %</h3><div class="chart-wrap"><canvas id="ov-mem"></canvas></div></div>
        <div><h3>GPU</h3><div class="chart-wrap"><canvas id="ov-gpu"></canvas></div></div>
        <div><h3>Disco usado %</h3><div class="chart-wrap"><canvas id="ov-disk"></canvas></div></div>
      </div></div>
      <div class="section"><h2>Rede</h2><div id="ov-net"><div class="skeleton" style="width:60%"></div></div></div>`;
  } else if(tab==='cpu'){
    el.innerHTML = `
      <div class="section"><h2>CPU</h2>
        <div class="grid">
          <div><h3>Uso %</h3><div class="chart-wrap"><canvas id="cpu-cpu"></canvas></div></div>
          <div><h3>Frequência MHz</h3><div class="chart-wrap"><canvas id="cpu-freq"></canvas></div></div>
        </div>
      </div>
      <div class="section"><h2>Temperaturas</h2><div id="cpu-temps"><div class="skeleton" style="width:40%"></div></div></div>`;
  } else if(tab==='memory'){
    el.innerHTML = `
      <div class="section"><h2>Memória</h2><div class="grid">
        <div><h3>Uso %</h3><div class="chart-wrap"><canvas id="mem-p"></canvas></div></div>
        <div><h3>Usada GB</h3><div class="chart-wrap"><canvas id="mem-gb"></canvas></div></div>
      </div></div>`;
  } else if(tab==='gpu'){
    el.innerHTML = `
      <div class="section"><h2>GPU</h2><div class="grid">
        <div><h3>Temperatura °C</h3><div class="chart-wrap"><canvas id="gpu-temp"></canvas></div></div>
        <div><h3>Utilização %</h3><div class="chart-wrap"><canvas id="gpu-util"></canvas></div></div>
        <div><h3>Power W</h3><div class="chart-wrap"><canvas id="gpu-power"></canvas></div></div>
      </div></div>`;
  } else if(tab==='sensors'){
    el.innerHTML = `<div class="section"><h2>Sensores</h2><div id="sensors-table"><div class="skeleton"></div></div><p class="subtle">Mostra últimos valores por sensor (sem janela)</p></div>`;
  } else if(tab==='disk'){
    el.innerHTML = `
      <div class="section"><h2>Volumes</h2><div id="disk-usage" class="table-wrap"><div class="skeleton"></div></div></div>
      <div class="section"><h2>Físicos</h2><div id="disk-phys" class="table-wrap"><div class="skeleton"></div></div></div>
      <div class="section"><h2>SMART</h2><div id="disk-smart" class="table-wrap"><div class="skeleton"></div></div></div>
      <div class="section"><h2>SMART temperatura</h2><div class="chart-wrap tall"><canvas id="disk-temp"></canvas></div></div>`;
  } else if(tab==='net'){
    el.innerHTML = `
      <div class="section"><h2>Totais por interface</h2><div id="net-latest" class="table-wrap"><div class="skeleton"></div></div></div>
      <div class="section"><h2>Throughput</h2><div class="chart-wrap"><canvas id="net-kbs"></canvas></div><p class="subtle">Delta a cada ~10s na janela selecionada</p></div>`;
  } else if(tab==='processes'){
    el.innerHTML = `<div class="section"><h2>Top 15 por CPU</h2><div id="proc-table" class="table-wrap"><div class="skeleton"></div></div><p class="subtle">Snapshot mais recente</p></div>`;
  } else if(tab==='system'){
    el.innerHTML = `<div class="section"><h2>Sistema</h2><pre id="sys-pre" class="subtle" style="white-space:pre-wrap;word-break:break-all;margin:0"></pre></div>
      <div class="section"><h2>Heartbeat</h2><div id="hb-table" class="table-wrap"><div class="skeleton"></div></div></div>`;
  }
  S.mounted.add(tab);
}

function schedule(){
  unschedule();
  if(!els.auto.checked || document.hidden) return;
  S.timer = setTimeout(async ()=>{
    await updateCurrent();
    await loadHeader();
    schedule();
  }, S.intervalSec * 1000);
}
function unschedule(){ if(S.timer){ clearTimeout(S.timer); S.timer=null; } }

async function updateCurrent(opts={}){
  const tab = S.tab;
  // não apaga DOM no poll — só atualiza dados
  try{
    if(tab==='overview') await updateOverview(opts);
    else if(tab==='cpu') await updateCpu(opts);
    else if(tab==='memory') await updateMemory(opts);
    else if(tab==='gpu') await updateGpu(opts);
    else if(tab==='sensors') await updateSensors(opts);
    else if(tab==='disk') await updateDisk(opts);
    else if(tab==='net') await updateNet(opts);
    else if(tab==='processes') await updateProcesses(opts);
    else if(tab==='system') await updateSystem(opts);
    els.lastUpdated.textContent = 'atualizado ' + new Date().toLocaleTimeString();
  }catch(e){
    console.error(e);
    // mostra erro discreto sem apagar charts
    const el = document.getElementById('tab-'+tab);
    if(el && !el.querySelector('.err')){
      const d=document.createElement('p'); d.className='err subtle'; d.style.color='#f87171'; d.textContent='erro ao atualizar';
      el.prepend(d); setTimeout(()=>d.remove(), 4000);
    }
  }
}

async function fetchJSON(url){
  if(S.abort) S.abort.abort();
  S.abort = new AbortController();
  const r = await fetch(url, {signal: S.abort.signal, headers:{'Cache-Control':'no-store'}});
  if(!r.ok) throw new Error(url+' '+r.status);
  return r.json();
}

async function loadHeader(){
  try{
    const [sys, db] = await Promise.all([fetchJSON('/api/system'), fetchJSON('/api/db_size')]);
    if(sys && db){
      els.header.innerHTML = `<span><strong>${sys.hostname||''}</strong></span><span class="sep">·</span><span>up ${Math.floor((sys.uptime||0)/3600)}h</span><span class="sep">·</span><span>${sys.ram_gb? sys.ram_gb.toFixed(1)+'GB RAM':''}</span><span class="sep">·</span><span>DB ${db.size}</span>`;
      if(els.footerDb) els.footerDb.textContent = `${db.size} · ${db.cpu_rows} cpu rows · ${db.sensor_rows} sensores`;
    }
  }catch(e){ /* silencioso */ }
}

// ---- chart helpers (update diferencial, sem destroy) ----
function upsertChart(id, labels, datasets, extra={}){
  const canvas = document.getElementById(id);
  if(!canvas) return;
  const existing = S.charts[id];
  if(existing){
    existing.data.labels = labels;
    existing.data.datasets = datasets;
    Object.assign(existing.options, extra);
    existing.update('none');
    return existing;
  }
  const ctx = canvas.getContext('2d');
  const c = new Chart(ctx, {
    type: datasets[0]?.type || 'line',
    data: {labels, datasets},
    options: {
      responsive:true, maintainAspectRatio:false, animation:false,
      interaction:{mode:'index', intersect:false},
      plugins:{legend:{labels:{color:'#8a94a6', boxWidth:12, font:{size:11}}}, decimation:{enabled:true}},
      scales:{
        x:{ticks:{color:'#6b7585', maxTicksLimit:6}, grid:{color:'rgba(255,255,255,.06)'}},
        y:{ticks:{color:'#6b7585'}, grid:{color:'rgba(255,255,255,.06)'}}
      },
      ...extra
    }
  });
  S.charts[id]=c;
  return c;
}
function fmtTime(ts){ return new Date(ts).toLocaleTimeString([], {hour:'2-digit', minute:'2-digit', second:'2-digit'}); }

function tableHTML(rows, cols){
  if(!rows.length) return '<p class="subtle">sem dados</p>';
  let h='<table><thead><tr>'+cols.map(c=>`<th>${c}</th>`).join('')+'</tr></thead><tbody>';
  rows.forEach(r=>{
    h+='<tr>'+cols.map(c=>{
      const v=r[c];
      return `<td>${v!==undefined && v!==null ? String(v) : '<span class="muted">—</span>'}</td>`;
    }).join('')+'</tr>';
  });
  return h+'</tbody></table>';
}

// ---- tab updaters ----
async function updateOverview(){
  const win = S.win;
  const [cpu, mem, gpu, disk, net] = await Promise.all([
    fetchJSON('/api/cpu?window='+encodeURIComponent(win)),
    fetchJSON('/api/memory?window='+encodeURIComponent(win)),
    fetchJSON('/api/gpu?window='+encodeURIComponent(win)),
    fetchJSON('/api/disk/usage'),
    fetchJSON('/api/net/latest'),
  ]);
  if(cpu.length){
    const labels=cpu.map(d=>fmtTime(d.ts));
    upsertChart('ov-cpu', labels, [{label:'CPU %', data:cpu.map(d=>d.cpu), borderColor:'#2e7ff1', borderWidth:1.4, pointRadius:0, tension:.25}]);
  }
  if(mem.length){
    const labels=mem.map(d=>fmtTime(d.ts));
    upsertChart('ov-mem', labels, [{label:'Mem %', data:mem.map(d=>d.used_percent), borderColor:'#22c55e', borderWidth:1.4, pointRadius:0, tension:.25}]);
  }
  if(gpu.length){
    const labels=gpu.map(d=>fmtTime(d.ts));
    const existing=S.charts['ov-gpu'];
    if(existing){
      existing.data.labels=labels;
      existing.data.datasets[0].data=gpu.map(d=>d.temp);
      existing.data.datasets[1].data=gpu.map(d=>d.util);
      existing.update('none');
    } else {
      upsertChart('ov-gpu', labels, [
        {label:'Temp °C', data:gpu.map(d=>d.temp), borderColor:'#ef4444', borderWidth:1.3, pointRadius:0, tension:.2, yAxisID:'y'},
        {label:'Util %', data:gpu.map(d=>d.util), borderColor:'#22c55e', borderWidth:1.3, pointRadius:0, tension:.2, yAxisID:'y1'}
      ], {scales:{y:{type:'linear',position:'left', title:{display:true,text:'°C',color:'#8a94a6'}}, y1:{type:'linear',position:'right', grid:{drawOnChartArea:false}, title:{display:true,text:'%',color:'#8a94a6'}}, x:{ticks:{color:'#6b7585'}, grid:{color:'rgba(255,255,255,.06)'}}}});
    }
  }
  if(disk.length){
    const labels=disk.map(d=>d.device);
    const vals=disk.map(d=>d.used_percent);
    // bar chart diferencial
    const id='ov-disk';
    const canvas=document.getElementById(id);
    if(canvas){
      if(S.charts[id]){
        S.charts[id].data.labels=labels;
        S.charts[id].data.datasets[0].data=vals;
        S.charts[id].update('none');
      } else {
        upsertChart(id, labels, [{type:'bar', label:'usado %', data:vals, backgroundColor:'rgba(46,127,241,.85)', borderWidth:0}], {scales:{y:{max:100}}});
      }
    }
  }
  const netEl=document.getElementById('ov-net');
  if(netEl) netEl.innerHTML = tableHTML(net.map(r=>({iface:r.iface, recv:(r.recv/1024/1024).toFixed(1)+' MB', sent:(r.sent/1024/1024).toFixed(1)+' MB'})), ['iface','recv','sent']);
}

async function updateCpu(){
  const win=S.win;
  const [cpu, temps] = await Promise.all([fetchJSON('/api/cpu?window='+encodeURIComponent(win)), fetchJSON('/api/sensors/cpu_temps?window='+encodeURIComponent(win))]);
  if(cpu.length){
    const labels=cpu.map(d=>fmtTime(d.ts));
    upsertChart('cpu-cpu', labels, [{label:'CPU %', data:cpu.map(d=>d.cpu), borderColor:'#2e7ff1', pointRadius:0, tension:.2}]);
    upsertChart('cpu-freq', labels, [{label:'MHz', data:cpu.map(d=>d.freq), borderColor:'#a855f7', pointRadius:0, tension:.2}]);
  }
  if(temps.length){
    const groups={};
    temps.forEach(t=>{ (groups[t.name]=groups[t.name]||[]).push(t); });
    const container=document.getElementById('cpu-temps');
    // monta estrutura de temps apenas uma vez (sem recriar canvas que já tem chart)
    const wanted = Object.keys(groups).slice(0,4);
    const existingIds = new Set([...container.querySelectorAll('canvas')].map(c=>c.id));
    const wantedIds = new Set(wanted.map(n=>'cpu-temp-'+n.replace(/[^a-z0-9]/gi,'_')));
    // remove excedentes
    [...container.children].forEach(ch=>{
      const cv=ch.querySelector('canvas');
      if(cv && !wantedIds.has(cv.id)) ch.remove();
    });
    wanted.forEach(name=>{
      const id='cpu-temp-'+name.replace(/[^a-z0-9]/gi,'_');
      if(!existingIds.has(id)){
        const div=document.createElement('div');
        div.innerHTML=`<h3>${name.split(':').pop()}</h3><div class="chart-wrap"><canvas id="${id}"></canvas></div>`;
        // inserir como section flat
        const sec=document.createElement('div'); sec.className='section'; sec.style.borderTop='1px solid var(--hairline)'; sec.style.paddingTop='14px'; sec.appendChild(div);
        container.appendChild(sec);
      }
    });
    wanted.forEach(name=>{
      const id='cpu-temp-'+name.replace(/[^a-z0-9]/gi,'_');
      const arr=groups[name];
      const labels=arr.map(d=>fmtTime(d.ts));
      upsertChart(id, labels, [{label:'°C', data:arr.map(d=>d.value), borderColor:'#ef4444', pointRadius:0, tension:.15}]);
    });
  }
}

async function updateMemory(){
  const mem = await fetchJSON('/api/memory?window='+encodeURIComponent(S.win));
  if(mem.length){
    const labels=mem.map(d=>fmtTime(d.ts));
    upsertChart('mem-p', labels, [{label:'%', data:mem.map(d=>d.used_percent), borderColor:'#22c55e', pointRadius:0, tension:.2}]);
    upsertChart('mem-gb', labels, [{label:'GB', data:mem.map(d=>d.used_gb), borderColor:'#2e7ff1', pointRadius:0, tension:.2}]);
  }
}

async function updateGpu(){
  const gpu = await fetchJSON('/api/gpu?window='+encodeURIComponent(S.win));
  if(gpu.length){
    const labels=gpu.map(d=>fmtTime(d.ts));
    upsertChart('gpu-temp', labels, [{label:'°C', data:gpu.map(d=>d.temp), borderColor:'#ef4444', pointRadius:0, tension:.2}]);
    upsertChart('gpu-util', labels, [{label:'%', data:gpu.map(d=>d.util), borderColor:'#22c55e', pointRadius:0, tension:.2}]);
    upsertChart('gpu-power', labels, [{label:'W', data:gpu.map(d=>d.power), borderColor:'#f59e0b', pointRadius:0, tension:.2}]);
  }
}

async function updateSensors(){
  const latest = await fetchJSON('/api/sensors/latest');
  const el=document.getElementById('sensors-table');
  if(el) el.innerHTML = tableHTML(latest.slice(0,50).map(r=>({name:r.name.slice(0,64), type:r.type, value:r.value, unit:r.unit||''})), ['name','type','value','unit']);
}

async function updateDisk(){
  const [usage, phys, smart] = await Promise.all([fetchJSON('/api/disk/usage'), fetchJSON('/api/disk/physical'), fetchJSON('/api/disk/smart')]);
  document.getElementById('disk-usage').innerHTML = tableHTML(usage.map(r=>({device:r.device, mount:r.mount, used:(r.used_percent!=null?r.used_percent+'%':''), free:r.free_gb!=null? r.free_gb.toFixed(1)+' GB':''})), ['device','mount','used','free']);
  document.getElementById('disk-phys').innerHTML = tableHTML(phys.map(r=>({id:r.device_id, name:(r.friendly_name||r.model||'').slice(0,28), type:r.media_type||'', bus:r.bus_type||'', health:r.health||'', size:r.size_gb!=null? Math.round(r.size_gb)+' GB':''})), ['id','name','type','bus','health','size']);
  document.getElementById('disk-smart').innerHTML = tableHTML(smart.map(r=>({dev:r.device, model:(r.model||'').slice(0,20), temp:r.temp!=null? r.temp+'°C':'', poh:r.poh||'', wear:r.wear!=null? r.wear+'%':'', passed:String(r.passed)})), ['dev','model','temp','poh','wear','passed']);
  // histórico temperatura SMART — respeita janela
  try{
    const hist = await fetchJSON('/api/disk/smart/history?window='+encodeURIComponent(S.win));
    if(hist.length){
      const byDev={};
      hist.forEach(r=>{ (byDev[r.device]=byDev[r.device]||[]).push(r); });
      const labelsByDev = {};
      const datasets=[];
      const colors=['#2e7ff1','#22c55e','#ef4444','#a855f7'];
      let ci=0;
      for(const dev in byDev){
        const arr=byDev[dev].sort((a,b)=> new Date(a.ts)-new Date(b.ts));
        if(!datasets.length) labelsByDev.labels = arr.map(d=>fmtTime(d.ts));
        datasets.push({label:dev, data:arr.map(d=>d.temp), borderColor:colors[ci%colors.length], pointRadius:0, tension:.2});
        ci++;
        if(ci>=4) break;
      }
      const id='disk-temp';
      if(datasets.length){
        if(S.charts[id]){
          S.charts[id].data.labels=labelsByDev.labels;
          S.charts[id].data.datasets=datasets;
          S.charts[id].update('none');
        } else {
          upsertChart(id, labelsByDev.labels, datasets);
        }
      }
    }
  }catch(e){ /* sem histórico ainda */ }
}

async function updateNet(){
  const [latest, hist] = await Promise.all([fetchJSON('/api/net/latest'), fetchJSON('/api/net?window='+encodeURIComponent(S.win))]);
  document.getElementById('net-latest').innerHTML = tableHTML(latest.map(r=>({iface:r.iface, recv:(r.recv/1024/1024).toFixed(1)+' MB', sent:(r.sent/1024/1024).toFixed(1)+' MB'})), ['iface','recv','sent']);
  if(hist.length){
    const byIface={};
    hist.forEach(r=>{ (byIface[r.iface]=byIface[r.iface]||[]).push(r); });
    const iface = Object.keys(byIface).find(k=>k==='Wi-Fi') || Object.keys(byIface)[0];
    if(iface){
      const arr=byIface[iface].sort((a,b)=> new Date(a.ts)-new Date(b.ts));
      const withDelta = arr.slice(1).map((v,i)=>{
        const prev=arr[i];
        const dt=(new Date(v.ts)-new Date(prev.ts))/1000;
        const recv_kbs= dt>0 ? (v.recv - prev.recv)/1024/dt : 0;
        const sent_kbs= dt>0 ? (v.sent - prev.sent)/1024/dt : 0;
        return {...v, recv_kbs: Math.max(0,recv_kbs), sent_kbs: Math.max(0,sent_kbs)};
      });
      const labels=withDelta.map(d=>fmtTime(d.ts));
      const id='net-kbs';
      if(S.charts[id]){
        S.charts[id].data.labels=labels;
        S.charts[id].data.datasets[0].data=withDelta.map(d=>d.recv_kbs);
        S.charts[id].data.datasets[1].data=withDelta.map(d=>d.sent_kbs);
        S.charts[id].update('none');
      } else {
        upsertChart(id, labels, [
          {label:'RX KB/s', data:withDelta.map(d=>d.recv_kbs), borderColor:'#2e7ff1', pointRadius:0, tension:.2},
          {label:'TX KB/s', data:withDelta.map(d=>d.sent_kbs), borderColor:'#22c55e', pointRadius:0, tension:.2}
        ]);
      }
    }
  }
}

async function updateProcesses(){
  const procs = await fetchJSON('/api/processes');
  document.getElementById('proc-table').innerHTML = tableHTML(procs.map(r=>({name:(r.name||'').slice(0,28), pid:r.pid, cpu:r.cpu!=null? r.cpu.toFixed(1):'', mem:r.mem!=null? r.mem.toFixed(1):'', rss:r.rss_mb!=null? r.rss_mb.toFixed(0):''})), ['name','pid','cpu','mem','rss']);
}

async function updateSystem(){
  const [sys, hb] = await Promise.all([fetchJSON('/api/system'), fetchJSON('/api/heartbeat')]);
  const pre=document.getElementById('sys-pre');
  if(pre) pre.textContent = sys ? JSON.stringify(sys,null,2) : '—';
  document.getElementById('hb-table').innerHTML = tableHTML(hb.map(r=>({collector:r.collector, success:String(r.success), ts:new Date(r.ts).toLocaleTimeString()})), ['collector','success','ts']);
}

init();
