const TAB_NAMES = new Set(['overview','cpu','memory','gpu','power','sensors','disk','net','processes','system']);
const WINDOW_TABS = new Set(['overview','cpu','memory','gpu','power','sensors','disk','net']);
const COLORS = ['#2e7ff1','#22c55e','#ef4444','#a855f7','#f59e0b','#06b6d4','#ec4899','#84cc16'];
const GIB = 1073741824;
const MIB = 1048576;
const KBPS_8 = 8 / 1e6;
const S = {
  tab: TAB_NAMES.has(localStorage.getItem('tab')) ? localStorage.getItem('tab') : 'overview',
  win: localStorage.getItem('win') || '1 hour',
  intervalSec: Math.min(60, Math.max(5, parseInt(localStorage.getItem('interval') || '10', 10) || 10)),
  timer: null, charts: {}, mounted: new Set(), netIface: null, diskVolume: null, diskIoDevice: null,
};
const els = {};

const INDICATORS = {
  cpu: ['Uso da CPU', 'Percentual de tempo em que os processadores lógicos não ficaram ociosos.', 'Fonte: psutil. Valores são médias no bucket da janela; picos curtos podem ser suavizados. Uso sustentado próximo de 100% indica saturação, mas deve ser avaliado junto de frequência e temperatura.'],
  cpuFreq: ['Frequência da CPU', 'Clock médio atual informado pelo sistema, em MHz.', 'Fonte: psutil. Boost, economia de energia e limites térmicos alteram a frequência. Não representa desempenho por ciclo nem deve ser comparado isoladamente entre arquiteturas.'],
  temperature: ['Temperatura', 'Temperatura reportada pelos sensores do hardware, em °C.', 'Fonte: LibreHardwareMonitor ou NVML. O significado depende do sensor: package, core, hotspot e ambiente não são equivalentes. Limites aceitáveis dependem do fabricante.'],
  memory: ['Uso de memória', 'RAM ocupada em percentual ou GiB.', 'Fonte: psutil. Cache recuperável pode ser contabilizado de forma diferente pelo Windows; pressão real deve ser analisada junto de paginação e desempenho.'],
  gpu: ['Carga da GPU', 'Utilização e temperatura reportadas pelo driver da GPU.', 'Fonte: NVML/nvidia-smi. Ausência de potência significa que a placa/driver não expõe esse contador; não é tratada como zero.'],
  capacity: ['Capacidade dos volumes', 'Espaço usado e livre por volume ao longo do tempo.', 'Fonte: monitor.disk_usage. GiB usa base 1024. Mudanças de montagem ou expansão do volume podem alterar a capacidade total; percentual e bytes devem ser analisados juntos.'],
  diskThroughput: ['Throughput de disco', 'Bytes efetivamente lidos e escritos por segundo.', 'Derivado da diferença de contadores cumulativos dividida pelo tempo. Intervalos com reboot, contador reduzido ou gap superior a três coletas são descartados.'],
  diskCumulative: ['I/O acumulado', 'Total lido e escrito desde o início da janela selecionada.', 'Soma apenas deltas válidos dos contadores do Windows. Reinicia visualmente em cada janela e não inclui intervalos sem cobertura.'],
  diskIops: ['IOPS e ocupação', 'Operações por segundo e percentual de tempo em que o disco esteve ocupado.', 'IOPS é delta de operações por segundo. Ocupação usa busy_time/tempo e é limitada a 100%. Alta ocupação com baixo throughput pode indicar acessos pequenos ou latência.'],
  diskLatency: ['Latência de disco', 'Tempo médio por operação de leitura e escrita, em ms.', 'Calculada por delta de tempo de I/O dividido pelo delta de operações. Fica indisponível quando não há operações; é uma média do intervalo, não percentil de cauda.'],
  smart: ['SMART', 'Indicadores de saúde, temperatura e desgaste expostos pelo firmware do dispositivo.', 'Fonte: smartctl. Campos variam por SATA/NVMe e fabricante. SMART aprovado não garante ausência de falha; setores, media errors e temperatura devem ser acompanhados.'],
  netThroughput: ['Throughput de rede', 'Taxa recebida e enviada pela interface, em bit/s.', 'Derivado dos contadores de bytes. Resets e gaps são descartados. É tráfego da interface, não necessariamente internet, e pode incluir VPN, virtualização e tráfego local.'],
  netCumulative: ['Tráfego acumulado', 'Bytes recebidos e enviados durante a janela selecionada.', 'Soma deltas válidos da interface selecionada. O total reinicia com a janela e evita somar interfaces automaticamente para não duplicar tráfego virtual.'],
  netPackets: ['Pacotes, erros e descartes', 'Taxa de pacotes e eventos de erro/drop observados na interface.', 'Pacotes/s são derivados de contadores. Erros e descartes são deltas por bucket; valores persistentes podem indicar driver, sinal, congestionamento ou buffer insuficiente.'],
  netUtil: ['Utilização do link', 'Maior taxa entre RX e TX como percentual da velocidade nominal.', 'Calculada somente quando speed_mbps é informado. Em links sem fio, a velocidade negociada não equivale à capacidade útil; 100% não é um limite preciso de aplicação.'],
  powerInstant: ['Potência instantânea', 'Potência média por bucket, em watts, separando fontes medidas e estimadas.', 'CPU Package é preferido para evitar somar subcomponentes. GPU usa leitura nativa quando disponível; modelo por utilização só é ativado com limites configurados. A estimativa na tomada inclui baseline auxiliar e eficiência da fonte.'],
  powerEnergy: ['Energia acumulada', 'Energia consumida na janela, em Wh ou kWh.', 'Integração trapezoidal entre pontos consecutivos. Gaps superiores ao limite não são preenchidos. Medido é subtotal dos componentes instrumentados; estimado representa o modelo calibrável do sistema.'],
  powerQuality: ['Qualidade energética', 'Cobertura e procedência usadas para interpretar a estimativa.', 'Cobertura é o tempo integrado dividido pela janela. Sensores CPU sobrepostos são excluídos. A estimativa padrão não substitui medidor de tomada e deve ser calibrada para comparação absoluta.'],
  sensors: ['Explorador de sensores', 'Histórico das séries mais recentes do tipo selecionado.', 'Fonte: LibreHardwareMonitor. Unidades e nomes vêm do hardware. Até oito séries são exibidas para manter legibilidade; sensores sem atualização permanecem visíveis na tabela com seu timestamp.'],
  processes: ['Processos', 'Snapshot dos processos com maior CPU no instante mais recente.', 'CPU e memória são valores do coletor, não médias da janela. Processos curtos podem não aparecer e o snapshot não substitui tracing ou profiling.'],
  heartbeat: ['Saúde da coleta', 'Último resultado de cada coletor e idade da amostra.', 'Falha, erro textual ou amostra antiga indica cobertura incompleta. O endpoint /api/health verifica o processo web; /api/ready também verifica banco e schema.'],
};

function escapeHTML(value){
  return String(value).replace(/[&<>'"]/g, c=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));
}
function info(key){
  const item=INDICATORS[key];
  if(!item) return '';
  return `<p class="indicator-summary">${escapeHTML(item[1])}</p><details><summary>Definição e metodologia</summary><p><strong>${escapeHTML(item[0])}.</strong> ${escapeHTML(item[2])}</p></details>`;
}
function metric(title, id, key, tall=false){
  return `<div class="metric"><h3>${escapeHTML(title)}</h3><div class="chart-wrap${tall?' tall':''}"><canvas id="${id}"></canvas></div>${info(key)}</div>`;
}
function card(label, value, tone=''){ return `<div class="card ${tone}"><span>${escapeHTML(label)}</span><strong>${escapeHTML(value ?? '—')}</strong></div>`; }

function init(){
  els.tabs=document.querySelectorAll('nav#tabs button'); els.panels=document.querySelectorAll('.tab');
  els.win=document.getElementById('window'); els.winHint=document.getElementById('window-hint');
  els.auto=document.getElementById('auto'); els.interval=document.getElementById('interval');
  els.intervalVal=document.getElementById('interval-val'); els.refresh=document.getElementById('refresh');
  els.lastUpdated=document.getElementById('last-updated'); els.header=document.getElementById('header-metrics');
  els.footerDb=document.getElementById('footer-db');
  if([...els.win.options].some(o=>o.value===S.win)) els.win.value=S.win; else S.win='1 hour';
  els.interval.value=String(S.intervalSec); els.intervalVal.textContent=S.intervalSec+'s';
  els.tabs.forEach(b=>b.addEventListener('click',()=>setTab(b.dataset.tab)));
  els.win.addEventListener('change',()=>{S.win=els.win.value;localStorage.setItem('win',S.win);updateCurrent();});
  els.refresh.addEventListener('click',()=>updateCurrent());
  els.auto.addEventListener('change',()=>els.auto.checked?schedule():unschedule());
  els.interval.addEventListener('input',e=>{S.intervalSec=parseInt(e.target.value,10);els.intervalVal.textContent=S.intervalSec+'s';localStorage.setItem('interval',String(S.intervalSec));});
  els.interval.addEventListener('change',()=>{if(els.auto.checked)schedule();});
  document.addEventListener('visibilitychange',()=>document.hidden?unschedule():(els.auto.checked&&schedule()));
  setTab(S.tab); loadHeader(); schedule();
}

function setTab(tab){
  if(!TAB_NAMES.has(tab)) tab='overview';
  S.tab=tab; localStorage.setItem('tab',tab);
  const historical=WINDOW_TABS.has(tab); els.win.disabled=!historical; els.winHint.hidden=historical;
  els.tabs.forEach(b=>{const active=b.dataset.tab===tab;b.classList.toggle('active',active);b.setAttribute('aria-selected',String(active));});
  els.panels.forEach(p=>{const active=p.id==='tab-'+tab;p.classList.toggle('active',active);p.hidden=!active;});
  ensureMounted(tab); updateCurrent();
}

function ensureMounted(tab){
  if(S.mounted.has(tab)) return;
  const el=document.getElementById('tab-'+tab); if(!el) return;
  if(tab==='overview') el.innerHTML=`
    <div class="section"><h2>Estado atual</h2><div id="ov-cards" class="cards"><div class="skeleton"></div></div>${info('heartbeat')}</div>
    <div class="section"><h2>Recursos</h2><div class="grid">${metric('CPU','ov-cpu','cpu')}${metric('Memória','ov-mem','memory')}${metric('GPU','ov-gpu','gpu')}${metric('Potência estimada','ov-power','powerInstant')}</div></div>
    <div class="section"><h2>Armazenamento e rede</h2><div class="grid">${metric('Disco usado','ov-disk','capacity')}<div><h3>Interfaces</h3><div id="ov-net" class="table-wrap"></div>${info('netCumulative')}</div></div></div>`;
  else if(tab==='cpu') el.innerHTML=`<div class="section"><h2>CPU</h2><div class="grid">${metric('Uso (%)','cpu-cpu','cpu')}${metric('Frequência (MHz)','cpu-freq','cpuFreq')}</div></div><div class="section"><h2>Temperaturas</h2><div id="cpu-temps" class="grid"></div>${info('temperature')}</div>`;
  else if(tab==='memory') el.innerHTML=`<div class="section"><h2>Memória</h2><div class="grid">${metric('Uso (%)','mem-p','memory')}${metric('Usada (GiB)','mem-gb','memory')}</div></div>`;
  else if(tab==='gpu') el.innerHTML=`<div class="section"><h2>GPU</h2><div class="grid">${metric('Temperatura (°C)','gpu-temp','temperature')}${metric('Utilização (%)','gpu-util','gpu')}${metric('Potência nativa (W)','gpu-power','powerInstant')}</div></div>`;
  else if(tab==='power') el.innerHTML=`
    <div class="section"><h2>Energia</h2><div id="power-cards" class="cards"></div>${info('powerQuality')}</div>
    <div class="section"><div class="grid">${metric('Potência (W)','power-w','powerInstant',true)}${metric('Energia acumulada','power-wh','powerEnergy',true)}</div></div>
    <div class="section"><h2>Totais por período</h2><div id="power-periods" class="table-wrap"></div>${info('powerEnergy')}</div>
    <div class="section"><h2>Fontes detectadas</h2><div id="power-sources" class="table-wrap"></div>${info('powerQuality')}</div>`;
  else if(tab==='sensors') el.innerHTML=`
    <div class="section"><h2>Explorador</h2><label class="selector">Tipo <select id="sensor-type"></select></label>${metric('Histórico','sensor-history','sensors',true)}</div>
    <div class="section"><h2>Últimos valores</h2><div id="sensors-table" class="table-wrap"></div>${info('sensors')}</div>`;
  else if(tab==='disk') el.innerHTML=`
    <div class="section"><h2>Volumes</h2><div id="disk-usage" class="table-wrap"></div><label class="selector">Volume <select id="disk-volume"></select></label>${metric('Capacidade histórica (GiB)','disk-capacity','capacity',true)}</div>
    <div class="section"><h2>Atividade</h2><label class="selector">Disco <select id="disk-io-device"></select></label><div class="grid">${metric('Throughput (MiB/s)','disk-throughput','diskThroughput')}${metric('Acumulado na janela','disk-cumulative','diskCumulative')}${metric('IOPS e ocupação','disk-iops','diskIops')}${metric('Latência média (ms)','disk-latency','diskLatency')}</div></div>
    <div class="section"><h2>Discos físicos</h2><div id="disk-phys" class="table-wrap"></div></div>
    <div class="section"><h2>SMART</h2><div id="disk-smart" class="table-wrap"></div>${metric('Temperatura SMART','disk-temp','smart',true)}</div>`;
  else if(tab==='net') el.innerHTML=`
    <div class="section"><h2>Interfaces</h2><div id="net-latest" class="table-wrap"></div><label class="selector">Interface <select id="net-iface"></select></label></div>
    <div class="section"><div id="net-cards" class="cards"></div><div class="grid">${metric('Throughput (Mbit/s)','net-throughput','netThroughput')}${metric('Acumulado na janela','net-cumulative','netCumulative')}${metric('Pacotes por segundo','net-packets','netPackets')}${metric('Erros e descartes','net-errors','netPackets')}${metric('Utilização do link (%)','net-util','netUtil')}</div></div>`;
  else if(tab==='processes') el.innerHTML=`<div class="section"><h2>Top 15 por CPU</h2><div id="proc-table" class="table-wrap"></div>${info('processes')}</div>`;
  else if(tab==='system') el.innerHTML=`<div class="section"><h2>Sistema</h2><pre id="sys-pre" class="subtle"></pre></div><div class="section"><h2>Heartbeat</h2><div id="hb-table" class="table-wrap"></div>${info('heartbeat')}</div>`;
  S.mounted.add(tab);
}

function schedule(){
  unschedule(); if(!els.auto.checked||document.hidden)return;
  S.timer=setTimeout(async()=>{await updateCurrent();await loadHeader();schedule();},S.intervalSec*1000);
}
function unschedule(){if(S.timer){clearTimeout(S.timer);S.timer=null;}}
let _updating=false;
async function updateCurrent(){
  if(_updating) return;
  _updating=true;
  try{
    const updater={overview:updateOverview,cpu:updateCpu,memory:updateMemory,gpu:updateGpu,power:updatePower,sensors:updateSensors,disk:updateDisk,net:updateNet,processes:updateProcesses,system:updateSystem}[S.tab];
    if(updater) await updater();
    els.lastUpdated.textContent='atualizado '+new Date().toLocaleTimeString();
  }catch(error){
    console.error(error); const panel=document.getElementById('tab-'+S.tab);
    if(panel&&!panel.querySelector('.err')){const p=document.createElement('p');p.className='err';p.textContent='Falha ao atualizar: '+error.message;panel.prepend(p);setTimeout(()=>p.remove(),6000);}
  }finally{_updating=false;}
}
const FETCH_TIMEOUT_MS = 10000;
const FETCH_RETRIES = 0;
async function fetchJSON(url, retries=FETCH_RETRIES){
  const controller = typeof AbortController!=='undefined' ? new AbortController() : null;
  const timer = controller ? setTimeout(()=>controller.abort(), FETCH_TIMEOUT_MS) : null;
  try {
    const r = await fetch(url, {headers:{'Cache-Control':'no-store'}, signal: controller?controller.signal:undefined});
    if(!r.ok){let message=`HTTP ${r.status}`;try{message=(await r.json()).error||message;}catch{}throw new Error(message);}
    return await r.json();
  } catch(err) {
    if (err && err.name==='AbortError') throw new Error('timeout ao buscar '+url);
    throw err;
  } finally { if(timer) clearTimeout(timer); }
}

async function settle(...promises){
  const results = await Promise.allSettled(promises);
  return results.map(r=>r.status==='fulfilled'?r.value:null);
}
async function loadHeader(){
  try{const [sys,db]=await settle(fetchJSON('/api/system'),fetchJSON('/api/db_size'));if(sys&&db){els.header.innerHTML=`<span><strong>${escapeHTML(sys.hostname||'')}</strong></span><span>up ${Math.floor((sys.uptime||0)/3600)}h</span><span>${sys.ram_gb?sys.ram_gb.toFixed(1)+' GiB RAM':''}</span><span>DB ${escapeHTML(db.size)}</span>`;els.footerDb.textContent=`${db.size} · ${db.cpu_rows} CPU · ${db.sensor_rows} sensores`;}}catch{}
}

function emptyChart(id,message='sem dados na janela'){
  const canvas=document.getElementById(id);if(!canvas)return;
  if(S.charts[id]){S.charts[id].destroy();delete S.charts[id];}
  const wrap=canvas.parentElement;let p=wrap.querySelector('.chart-empty');if(!p){p=document.createElement('p');p.className='chart-empty';wrap.appendChild(p);}p.textContent=message;
}
function upsertChart(id,labels,datasets,extra={}){
  const canvas=document.getElementById(id);if(!canvas)return;
  datasets=(datasets||[]).filter(ds=>ds.data?.some(v=>v!==null&&v!==undefined));
  if(!labels?.length||!datasets.length){emptyChart(id);return;}
  canvas.parentElement.querySelector('.chart-empty')?.remove();
  if(typeof Chart==='undefined'||window.chartLoadError){emptyChart(id,'Chart.js indisponível');return;}
  const options={responsive:true,maintainAspectRatio:false,animation:false,interaction:{mode:'index',intersect:false},spanGaps:false,
    plugins:{legend:{labels:{color:'#8a94a6',boxWidth:12,font:{size:11}}},decimation:{enabled:true},tooltip:{callbacks:{title:i=>i[0]?.label||''}}},
    scales:{x:{ticks:{color:'#6b7585',maxTicksLimit:5,maxRotation:0,callback:function(v){const l=this.getLabelForValue(v);return typeof l==='string'&&l.includes('T')?fmtAxisTime(l):l;}},grid:{color:'rgba(255,255,255,.06)'}},y:{ticks:{color:'#6b7585'},grid:{color:'rgba(255,255,255,.06)'}}},...extra};
  if(S.charts[id]){S.charts[id].data={labels,datasets};S.charts[id].options=options;S.charts[id].update('none');return;}
  S.charts[id]=new Chart(canvas.getContext('2d'),{type:datasets[0]?.type||'line',data:{labels,datasets},options});
}
function fmtAxisTime(value){const d=new Date(value);return Number.isNaN(d.getTime())?String(value):d.toLocaleString([],S.win.includes('day')?{month:'2-digit',day:'2-digit',hour:'2-digit'}:{hour:'2-digit',minute:'2-digit'});}
function aligned(rows,groupField,valueField){
  const labels=[...new Set(rows.map(r=>r.ts))].sort(); const groups=[...new Set(rows.map(r=>r[groupField]))];
  return {labels,datasets:groups.slice(0,8).map((group,i)=>{const values=new Map(rows.filter(r=>r[groupField]===group).map(r=>[r.ts,r[valueField]]));return {label:group,data:labels.map(ts=>values.has(ts)?values.get(ts):null),borderColor:COLORS[i%COLORS.length],pointRadius:0,tension:.15};})};
}
function setOptions(id,values,current,onChange){
  const select=document.getElementById(id);if(!select)return current;
  const unique=[...new Set(values.filter(Boolean))];if(!unique.length){select.innerHTML='';select.disabled=true;return null;}select.disabled=false;
  const chosen=unique.includes(current)?current:unique[0];
  if(select.dataset.values!==JSON.stringify(unique)){select.innerHTML=unique.map(v=>`<option value="${escapeHTML(v)}">${escapeHTML(v)}</option>`).join('');select.dataset.values=JSON.stringify(unique);select.onchange=()=>onChange(select.value);}
  select.value=chosen;return chosen;
}
function tableHTML(rows,columns){
  if(!rows.length)return '<p class="subtle">sem dados</p>';
  return `<table><thead><tr>${columns.map(([key,label])=>`<th>${escapeHTML(label||key)}</th>`).join('')}</tr></thead><tbody>${rows.map(row=>`<tr>${columns.map(([key])=>`<td>${row[key]===null||row[key]===undefined||row[key]===''?'<span class="muted">—</span>':escapeHTML(row[key])}</td>`).join('')}</tr>`).join('')}</tbody></table>`;
}
function fmtBytes(value){if(value===null||value===undefined)return '—';const units=['B','KiB','MiB','GiB','TiB'];let n=Number(value),i=0;while(Math.abs(n)>=1024&&i<units.length-1){n/=1024;i++;}return `${n.toFixed(i?1:0)} ${units[i]}`;}
function fmtWh(value){if(value===null||value===undefined)return '—';return value>=1000?`${(value/1000).toFixed(3)} kWh`:`${value.toFixed(2)} Wh`;}
function fmtNum(value,digits=1,suffix=''){return value===null||value===undefined?'—':Number(value).toFixed(digits)+suffix;}
function line(label,data,color,extra={}){return {label,data,borderColor:color,pointRadius:0,tension:.15,...extra};}

async function updateOverview(){
  const win=encodeURIComponent(S.win);
  const [cpu,mem,gpu,disk,net,power,hb]=await settle(fetchJSON(`/api/cpu?window=${win}`),fetchJSON(`/api/memory?window=${win}`),fetchJSON(`/api/gpu?window=${win}`),fetchJSON('/api/disk/usage'),fetchJSON('/api/net/latest'),fetchJSON(`/api/power?window=${win}`),fetchJSON('/api/heartbeat'));
  const latest=(a,key)=>Array.isArray(a)&&a.length?a[a.length-1][key]:null;
  const hbArr=Array.isArray(hb)?hb:[];
  const stale=hbArr.filter(r=>!r.success||Date.now()-new Date(r.ts)>180000).length;
  const powerSummary=power&&power.summary?power.summary:null;
  const powerMeta=power&&power.meta?power.meta:null;
  const lastPower=Array.isArray(power?.series)&&power.series.length?power.series.at(-1):null;
  const cov=powerMeta&&powerMeta.coverage_percent!=null?Math.min(100,Math.max(0,powerMeta.coverage_percent)):null;
  document.getElementById('ov-cards').innerHTML=
    card('CPU',fmtNum(latest(cpu,'cpu'),1,'%'))+
    card('Memória',fmtNum(latest(mem,'used_percent'),1,'%'))+
    card('Potência estimada',lastPower&&lastPower.estimated_w!=null?fmtNum(lastPower.estimated_w,1,' W'):'—')+
    card('Energia na janela',fmtWh(powerSummary?powerSummary.estimated_wh:null))+
    card('Cobertura energia',cov==null?'—':cov.toFixed(1)+'%',cov<80&&cov!=null?'warn':'')+
    card('Coletores com alerta',String(stale),stale?'bad':'');
  upsertChart('ov-cpu',(cpu||[]).map(r=>r.ts),[line('CPU %',(cpu||[]).map(r=>r.cpu),COLORS[0])]);
  upsertChart('ov-mem',(mem||[]).map(r=>r.ts),[line('Memória %',(mem||[]).map(r=>r.used_percent),COLORS[1])]);
  upsertChart('ov-gpu',(gpu||[]).map(r=>r.ts),[line('Temperatura °C',(gpu||[]).map(r=>r.temp),COLORS[2]),line('Utilização %',(gpu||[]).map(r=>r.util),COLORS[1])]);
  upsertChart('ov-power',(power?.series||[]).map(r=>r.ts),[line('Estimado W',(power?.series||[]).map(r=>r.estimated_w),COLORS[4]),line('Medido parcial W',(power?.series||[]).map(r=>r.measured_w),COLORS[0],{borderDash:[5,4]})]);
  upsertChart('ov-disk',(disk||[]).map(r=>r.device),[{type:'bar',label:'Usado %',data:(disk||[]).map(r=>r.used_percent),backgroundColor:COLORS[0]}],{scales:{x:{ticks:{color:'#8a94a6'}},y:{max:100,ticks:{color:'#8a94a6'},grid:{color:'rgba(255,255,255,.06)'}}}});
  document.getElementById('ov-net').innerHTML=tableHTML((net||[]).map(r=>({iface:r.iface,estado:r.is_up?'up':'down',rx:fmtBytes(r.recv),tx:fmtBytes(r.sent)})),[['iface','Interface'],['estado','Estado'],['rx','Recebido total'],['tx','Enviado total']]);
}

async function updateCpu(){const win=encodeURIComponent(S.win);const [cpu,temps]=await settle(fetchJSON(`/api/cpu?window=${win}`),fetchJSON(`/api/sensors/cpu_temps?window=${win}`));upsertChart('cpu-cpu',(cpu||[]).map(r=>r.ts),[line('CPU %',(cpu||[]).map(r=>r.cpu),COLORS[0])]);upsertChart('cpu-freq',(cpu||[]).map(r=>r.ts),[line('MHz',(cpu||[]).map(r=>r.freq),COLORS[3])]);const box=document.getElementById('cpu-temps');const names=[...new Set((temps||[]).map(r=>r.name))].slice(0,4);for(let i=0;i<4;i++){if(S.charts[`cpu-temp-${i}`]){S.charts[`cpu-temp-${i}`].destroy();delete S.charts[`cpu-temp-${i}`];}}box.innerHTML=names.map((name,i)=>metric(name,`cpu-temp-${i}`,'temperature')).join('');names.forEach((name,i)=>{const rows=(temps||[]).filter(r=>r.name===name);upsertChart(`cpu-temp-${i}`,rows.map(r=>r.ts),[line('°C',rows.map(r=>r.value),COLORS[2])]);});}
async function updateMemory(){const rows=await fetchJSON(`/api/memory?window=${encodeURIComponent(S.win)}`);upsertChart('mem-p',rows.map(r=>r.ts),[line('%',rows.map(r=>r.used_percent),COLORS[1])]);upsertChart('mem-gb',rows.map(r=>r.ts),[line('GiB',rows.map(r=>r.used_gb),COLORS[0])]);}
async function updateGpu(){const rows=await fetchJSON(`/api/gpu?window=${encodeURIComponent(S.win)}`);upsertChart('gpu-temp',rows.map(r=>r.ts),[line('°C',rows.map(r=>r.temp),COLORS[2])]);upsertChart('gpu-util',rows.map(r=>r.ts),[line('%',rows.map(r=>r.util),COLORS[1])]);upsertChart('gpu-power',rows.map(r=>r.ts),[line('W',rows.map(r=>r.power),COLORS[4])]);}

async function updatePower(){
  const data=await fetchJSON(`/api/power?window=${encodeURIComponent(S.win)}`);if(!data)return;
  const s=data.summary||{},m=data.meta||{};
  const cov=m.coverage_percent!=null?Math.min(100,Math.max(0,m.coverage_percent)):null;
  document.getElementById('power-cards').innerHTML=card('Estimado',fmtWh(s.estimated_wh))+card('Medido parcial',fmtWh(s.measured_wh))+card('Média estimada',s.average_estimated_w==null?'—':s.average_estimated_w.toFixed(1)+' W')+card('Pico estimado',s.peak_estimated_w==null?'—':s.peak_estimated_w.toFixed(1)+' W')+card('Cobertura',cov==null?'—':cov.toFixed(1)+'%',cov<80&&cov!=null?'warn':'')+card('Qualidade',m.quality?String(m.quality).replaceAll('_',' '):'—',m.quality==='partial'?'warn':'');
  const rows=Array.isArray(data.series)?data.series:[],labels=rows.map(r=>r.ts);
  upsertChart('power-w',labels,[line('CPU medida',rows.map(r=>r.cpu_w),COLORS[0]),line('GPU medida',rows.map(r=>r.gpu_measured_w),COLORS[1]),line('GPU estimada',rows.map(r=>r.gpu_estimated_w),COLORS[3],{borderDash:[4,4]}),line('Tomada estimada',rows.map(r=>r.estimated_w),COLORS[4])]);
  upsertChart('power-wh',labels,[line('Medido parcial Wh',rows.map(r=>r.cumulative_measured_wh),COLORS[0]),line('Estimado Wh',rows.map(r=>r.cumulative_estimated_wh),COLORS[4])]);
  const periods=(data.periods||{});const labelsMap={daily:'Dia',weekly:'Semana',monthly:'Mês'};
  const series=['daily','weekly','monthly'].flatMap(type=>Array.isArray(periods[type])?periods[type].map(r=>({tipo:labelsMap[type],periodo:r.period,medida:fmtWh(r.measured_wh),estimada:fmtWh(r.estimated_wh)})):[]);
  document.getElementById('power-periods').innerHTML=tableHTML(series,[['tipo','Período'],['periodo','Referência'],['medida','Medido parcial'],['estimada','Estimado']]);
  document.getElementById('power-sources').innerHTML=tableHTML((data.sources||[]).map(r=>({nome:r.name,estado:r.included?'incluída':'excluída',atual:fmtNum(r.latest,2,' W'),media:fmtNum(r.average,2,' W'),min:fmtNum(r.minimum,2),max:fmtNum(r.maximum,2),motivo:r.reason})),[['nome','Fonte'],['estado','Total'],['atual','Atual'],['media','Média'],['min','Mín. W'],['max','Máx. W'],['motivo','Critério']]);
}

async function updateSensors(){
  const latest=await fetchJSON('/api/sensors/latest');const types=[...new Set(latest.map(r=>r.type))].sort();const select=document.getElementById('sensor-type');
  if(!select.dataset.ready){select.innerHTML=types.map(t=>`<option value="${escapeHTML(t)}">${escapeHTML(t)}</option>`).join('');select.value=types.includes('power')?'power':types[0]||'';select.onchange=updateSensors;select.dataset.ready='1';}
  const type=select.value;const hist=type?await fetchJSON(`/api/sensors/history?window=${encodeURIComponent(S.win)}&type=${encodeURIComponent(type)}`):[];const chart=aligned(hist,'name','value');upsertChart('sensor-history',chart.labels,chart.datasets);
  document.getElementById('sensors-table').innerHTML=tableHTML(latest.slice(0,200).map(r=>({nome:r.name,tipo:r.type,valor:Number(r.value).toFixed(3),unidade:r.unit||'',amostra:new Date(r.ts).toLocaleString()})),[['nome','Sensor'],['tipo','Tipo'],['valor','Valor'],['unidade','Unidade'],['amostra','Última amostra']]);
}

async function updateDisk(){
  const win=encodeURIComponent(S.win);
  const [usage,history,physical,smart,smartHistory,io]=await settle(fetchJSON('/api/disk/usage'),fetchJSON(`/api/disk/usage/history?window=${win}`),fetchJSON('/api/disk/physical'),fetchJSON('/api/disk/smart'),fetchJSON(`/api/disk/smart/history?window=${win}`),fetchJSON(`/api/disk/io?window=${win}`));
  document.getElementById('disk-usage').innerHTML=tableHTML((usage||[]).map(r=>({device:r.device,mount:r.mount,total:fmtBytes(r.total_bytes),used:fmtBytes(r.used_bytes),free:fmtBytes(r.free_bytes),percent:r.used_percent?.toFixed(1)+'%'})),[['device','Volume'],['mount','Montagem'],['total','Total'],['used','Usado'],['free','Livre'],['percent','Uso']]);
  S.diskVolume=setOptions('disk-volume',(history||[]).map(r=>r.device),S.diskVolume,v=>{S.diskVolume=v;updateDisk();});const volumeRows=(history||[]).filter(r=>r.device===S.diskVolume);upsertChart('disk-capacity',volumeRows.map(r=>r.ts),[line('Usado GiB',volumeRows.map(r=>r.used_bytes/GIB),COLORS[0]),line('Livre GiB',volumeRows.map(r=>r.free_bytes/GIB),COLORS[1])]);
  S.diskIoDevice=setOptions('disk-io-device',(io||[]).map(r=>r.device),S.diskIoDevice,v=>{S.diskIoDevice=v;updateDisk();});const d=(io||[]).filter(r=>r.device===S.diskIoDevice),labels=d.map(r=>r.ts);
  upsertChart('disk-throughput',labels,[line('Leitura MiB/s',d.map(r=>r.read_bps/MIB),COLORS[0]),line('Escrita MiB/s',d.map(r=>r.write_bps/MIB),COLORS[1])]);
  upsertChart('disk-cumulative',labels,[line('Lido',d.map(r=>r.read/GIB),COLORS[0]),line('Escrito',d.map(r=>r.write/GIB),COLORS[1])]);
  upsertChart('disk-iops',labels,[line('Leitura IOPS',d.map(r=>r.read_iops),COLORS[0]),line('Escrita IOPS',d.map(r=>r.write_iops),COLORS[1]),line('Ocupação %',d.map(r=>r.busy_percent),COLORS[4],{borderDash:[4,4]})]);
  upsertChart('disk-latency',labels,[line('Leitura ms',d.map(r=>r.read_latency_ms),COLORS[0]),line('Escrita ms',d.map(r=>r.write_latency_ms),COLORS[1])]);
  document.getElementById('disk-phys').innerHTML=tableHTML((physical||[]).map(r=>({id:r.device_id,nome:r.friendly_name||r.model,tipo:r.media_type,bus:r.bus_type,saude:r.health,tamanho:r.size_gb?Math.round(r.size_gb)+' GiB':'—'})),[['id','ID'],['nome','Dispositivo'],['tipo','Mídia'],['bus','Barramento'],['saude','Saúde'],['tamanho','Tamanho']]);
  document.getElementById('disk-smart').innerHTML=tableHTML((smart||[]).map(r=>({dev:r.device,modelo:r.model,temp:r.temp===null?'—':r.temp+' °C',horas:r.poh,desgaste:r.wear===null?'—':r.wear+'%',erros:r.media_err,realocados:r.realloc,pendentes:r.pending,smart:r.passed===null?'—':r.passed?'OK':'FALHA'})),[['dev','Disco'],['modelo','Modelo'],['temp','Temp.'],['horas','Horas'],['desgaste','Desgaste'],['erros','Media errors'],['realocados','Realocados'],['pendentes','Pendentes'],['smart','SMART']]);
  const sc=aligned(smartHistory||[],'device','temp');upsertChart('disk-temp',sc.labels,sc.datasets);
}

async function updateNet(){
  const win=encodeURIComponent(S.win);const [latest,hist]=await settle(fetchJSON('/api/net/latest'),fetchJSON(`/api/net?window=${win}`));
  const histArr=Array.isArray(hist)?hist:[];
  document.getElementById('net-latest').innerHTML=tableHTML((latest||[]).map(r=>({iface:r.iface,estado:r.is_up?'up':'down',velocidade:r.speed_mbps?r.speed_mbps+' Mbit/s':'—',mtu:r.mtu,rx:fmtBytes(r.recv),tx:fmtBytes(r.sent)})),[['iface','Interface'],['estado','Estado'],['velocidade','Link'],['mtu','MTU'],['rx','Recebido total'],['tx','Enviado total']]);
  const preferred=histArr.some(r=>r.iface==='Wi-Fi')?'Wi-Fi':S.netIface;S.netIface=setOptions('net-iface',histArr.map(r=>r.iface),preferred,v=>{S.netIface=v;updateNet();});const rows=histArr.filter(r=>r.iface===S.netIface),labels=rows.map(r=>r.ts),last=rows.at(-1);
  document.getElementById('net-cards').innerHTML=card('Interface',S.netIface||'—')+card('Recebido na janela',last?fmtBytes(last.recv):'—')+card('Enviado na janela',last?fmtBytes(last.sent):'—')+card('Link',last?.speed_mbps?last.speed_mbps+' Mbit/s':'indisponível')+card('Utilização',last?.utilization_percent===null?'—':last?.utilization_percent.toFixed(2)+'%');
  upsertChart('net-throughput',labels,[line('RX Mbit/s',rows.map(r=>r.recv_bps*KBPS_8),COLORS[0]),line('TX Mbit/s',rows.map(r=>r.sent_bps*KBPS_8),COLORS[1])]);
  upsertChart('net-cumulative',labels,[line('Recebido GiB',rows.map(r=>r.recv/GIB),COLORS[0]),line('Enviado GiB',rows.map(r=>r.sent/GIB),COLORS[1])]);
  upsertChart('net-packets',labels,[line('RX pps',rows.map(r=>r.recv_pps),COLORS[0]),line('TX pps',rows.map(r=>r.sent_pps),COLORS[1])]);
  upsertChart('net-errors',labels,[line('Erros RX',rows.map(r=>r.errin),COLORS[2]),line('Erros TX',rows.map(r=>r.errout),COLORS[4]),line('Drops RX',rows.map(r=>r.dropin),COLORS[3]),line('Drops TX',rows.map(r=>r.dropout),COLORS[5])]);
  upsertChart('net-util',labels,[line('Utilização %',rows.map(r=>r.utilization_percent),COLORS[4])],{scales:{x:{ticks:{color:'#6b7585',maxTicksLimit:5}},y:{min:0,max:100,ticks:{color:'#6b7585'},grid:{color:'rgba(255,255,255,.06)'}}}});
}

async function updateProcesses(){const rows=await fetchJSON('/api/processes');document.getElementById('proc-table').innerHTML=tableHTML(rows.map(r=>({nome:r.name,pid:r.pid,cpu:fmtNum(r.cpu,1,'%'),mem:fmtNum(r.mem,1,'%'),rss:fmtNum(r.rss_mb,0,' MiB'),usuario:r.user})),[['nome','Processo'],['pid','PID'],['cpu','CPU'],['mem','Memória'],['rss','RSS'],['usuario','Usuário']]);}
async function updateSystem(){const [sys,hb]=await settle(fetchJSON('/api/system'),fetchJSON('/api/heartbeat'));document.getElementById('sys-pre').textContent=sys?JSON.stringify(sys,null,2):'sem dados';document.getElementById('hb-table').innerHTML=tableHTML((Array.isArray(hb)?hb:[]).map(r=>({coletor:r.collector,estado:r.success?'OK':'FALHA',idade:Math.max(0,Math.round((Date.now()-new Date(r.ts))/1000))+' s',erro:r.error||''})),[['coletor','Coletor'],['estado','Estado'],['idade','Idade'],['erro','Erro']]);}

init();
