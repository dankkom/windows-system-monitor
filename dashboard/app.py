import streamlit as st
import pandas as pd
import plotly.express as px
import plotly.graph_objects as go
from datetime import datetime, timezone
import queries as q

st.set_page_config(page_title="System Monitor", layout="wide", page_icon="🖥️")

# Sidebar - estado persistente
if "window" not in st.session_state:
    st.session_state.window = "1 hour"
if "auto" not in st.session_state:
    st.session_state.auto = True
if "interval" not in st.session_state:
    st.session_state.interval = 10

st.sidebar.title("🖥️ System Monitor")
window = st.sidebar.selectbox("Janela", ["1 hour", "6 hours", "24 hours", "7 days"], index=0, key="window")
auto = st.sidebar.checkbox("Auto-refresh", value=True, key="auto", help="Atualiza dados sem voltar à Overview")
interval = st.sidebar.slider("Intervalo (s)", 5, 60, 10, key="interval", disabled=not st.session_state.auto)
if st.session_state.auto:
    st.sidebar.caption(f"Atualiza a cada {interval}s • aba atual preservada")
else:
    st.sidebar.caption("Auto-refresh desabilitado")
if st.sidebar.button("🔄 Atualizar agora"):
    st.rerun()

# Header fixo (fora do fragment)
sys_row = q.q_system()
db_row = q.q_db_size()
colh1, colh2, colh3, colh4 = st.columns(4)
with colh1:
    st.metric("Host", sys_row[1] if sys_row else "pendulograph")
with colh2:
    st.metric("Uptime", f"{sys_row[2]//3600}h {(sys_row[2]%3600)//60}m" if sys_row else "-")
with colh3:
    st.metric("DB Size", db_row[0] if db_row else "-")
with colh4:
    st.metric("CPU cores", f"{sys_row[3]}" if sys_row else "-")
    st.caption(f"{db_row[1]} cpu rows, {db_row[2]} sensor rows" if db_row else "")

run_every = interval if st.session_state.auto else None

@st.fragment(run_every=run_every)
def render_content():
    window_f = st.session_state.window
    tabs = st.tabs(["Overview", "CPU", "Memória", "GPU", "Sensores", "Disco", "Rede", "Processos", "Sistema"])

    with tabs[0]:
        cpu_temps_latest = q.q_cpu_temps_latest()
        net_latest = q.q_net_latest()
        disk = q.q_disk_usage()
        oc1, oc2, oc3, oc4 = st.columns(4)
        with oc1:
            if cpu_temps_latest:
                tctl = next((r for r in cpu_temps_latest if "Tctl" in r[0]), cpu_temps_latest[0])
                st.metric("CPU Temp (Tctl)", f"{tctl[1]:.1f} {tctl[2]}", help=tctl[0])
            else:
                st.metric("CPU Temp", "-")
        with oc2:
            if net_latest:
                wifi = next((r for r in net_latest if r[0]=="Wi-Fi"), net_latest[0])
                st.metric("Rede Wi-Fi RX", f"{wifi[1]/1024/1024:.0f} MB", f"TX {wifi[2]/1024/1024:.0f} MB")
                st.caption(f"{wifi[0]} atual")
            else:
                st.metric("Rede", "-")
        with oc3:
            if disk:
                c_disk = next((d for d in disk if d[0]=="C:\\"), disk[0])
                st.metric("Disco C:", f"{c_disk[2]:.1f}% usado", f"{c_disk[3]:.0f} GB livres")
            else:
                st.metric("Disco", "-")
        with oc4:
            try:
                all_latest = q.q_sensors_latest()
                ssd = next((r for r in all_latest if "KINGSTON" in r[0] and r[1]=="temperature" and "Composite" in r[0]), None)
                if ssd:
                    st.metric("SSD Temp", f"{ssd[2]:.0f} {ssd[3]}", help=ssd[0])
                else:
                    st.metric("SSD", "-")
            except:
                st.metric("SSD", "-")

        c1, c2 = st.columns(2)
        with c1:
            cpu_data = q.q_cpu(window_f)
            if cpu_data:
                df = pd.DataFrame(cpu_data, columns=["ts","cpu","freq"])
                fig = px.line(df, x="ts", y="cpu", title="CPU Total %")
                fig.update_layout(height=300, margin=dict(l=10,r=10,t=40,b=10))
                st.plotly_chart(fig, use_container_width=True)
            else:
                st.info("Sem dados CPU")
        with c2:
            mem_data = q.q_memory(window_f)
            if mem_data:
                df = pd.DataFrame(mem_data, columns=["ts","used_percent","used_gb","swap"])
                fig = px.line(df, x="ts", y="used_percent", title="Memória usada %")
                fig.update_layout(height=300)
                st.plotly_chart(fig, use_container_width=True)
            else:
                st.info("Sem dados memória")
        c3, c4 = st.columns(2)
        with c3:
            gpu_data = q.q_gpu(window_f)
            if gpu_data:
                df = pd.DataFrame(gpu_data, columns=["ts","temp","util","power","vram"])
                fig = go.Figure()
                fig.add_trace(go.Scatter(x=df["ts"], y=df["temp"], name="Temp C", yaxis="y"))
                fig.add_trace(go.Scatter(x=df["ts"], y=df["util"], name="Util %", yaxis="y2"))
                fig.update_layout(title="GPU Temp & Utilização", yaxis=dict(title="Temp C"), yaxis2=dict(title="Util %", overlaying="y", side="right"), height=300)
                st.plotly_chart(fig, use_container_width=True)
        with c4:
            if disk:
                df = pd.DataFrame(disk, columns=["device","mount","used_percent","free_gb","ts"])
                fig = px.bar(df, x="device", y="used_percent", title="Uso disco % por volume (SSD/HDD)", text="used_percent")
                fig.update_layout(height=300)
                st.plotly_chart(fig, use_container_width=True)
        hb = q.q_heartbeat()
        if hb:
            dfhb = pd.DataFrame(hb, columns=["host","collector","ts","success","error"])
            st.dataframe(dfhb, use_container_width=True, height=200)

    with tabs[1]:
        cpu_temps = q.q_cpu_temps(window_f)
        cpu_temps_latest2 = q.q_cpu_temps_latest()
        if cpu_temps_latest2:
            cols = st.columns(len(cpu_temps_latest2[:4]))
            for i, (name, val, unit, ts) in enumerate(cpu_temps_latest2[:4]):
                short = name.split(":")[-1]
                cols[i].metric(short, f"{val:.1f} {unit}")
        cpu_data = q.q_cpu(window_f)
        if cpu_data:
            df = pd.DataFrame(cpu_data, columns=["ts","cpu","freq"])
            fig = px.line(df, x="ts", y="cpu", title="CPU % ao longo do tempo")
            st.plotly_chart(fig, use_container_width=True)
            fig2 = px.line(df, x="ts", y="freq", title="Frequência MHz")
            st.plotly_chart(fig2, use_container_width=True)
            if cpu_temps:
                dft = pd.DataFrame(cpu_temps, columns=["ts","name","value"])
                for name in dft["name"].unique()[:4]:
                    sub = dft[dft["name"]==name]
                    fig3 = px.line(sub, x="ts", y="value", title=f"CPU Temperatura - {name.split(':')[-1]}")
                    fig3.update_layout(yaxis_title="°C")
                    st.plotly_chart(fig3, use_container_width=True)
            st.dataframe(df.tail(20), use_container_width=True)
        else:
            st.warning("Sem dados CPU para janela")

    with tabs[2]:
        mem_data = q.q_memory(window_f)
        if mem_data:
            df = pd.DataFrame(mem_data, columns=["ts","used_percent","used_gb","swap"])
            fig = px.area(df, x="ts", y="used_percent", title="Memória %")
            st.plotly_chart(fig, use_container_width=True)
            fig2 = px.line(df, x="ts", y="used_gb", title="Memória usada GB")
            st.plotly_chart(fig2, use_container_width=True)
            st.dataframe(df.tail(20))

    with tabs[3]:
        gpu_data = q.q_gpu(window_f)
        if gpu_data:
            df = pd.DataFrame(gpu_data, columns=["ts","temp","util","power","vram"])
            st.plotly_chart(px.line(df, x="ts", y="temp", title="GPU Temperatura C"), use_container_width=True)
            st.plotly_chart(px.line(df, x="ts", y="util", title="GPU Utilização %"), use_container_width=True)
            st.plotly_chart(px.line(df, x="ts", y="power", title="GPU Power W"), use_container_width=True)
            st.dataframe(df.tail(20))
        else:
            st.info("Sem dados GPU")

    with tabs[4]:
        stype = st.selectbox("Tipo sensor", ["todos","temperature","fan","voltage","load","power","clock"], index=1, key="stype")
        latest = q.q_sensors_latest()
        if latest:
            dfl = pd.DataFrame(latest, columns=["name","type","value","unit","ts"])
            if stype != "todos":
                dfl = dfl[dfl["type"]==stype]
            dfl_sorted = dfl.sort_values("value", ascending=False).head(30)
            st.dataframe(dfl_sorted, use_container_width=True, height=400)
            if not dfl_sorted.empty:
                sel = st.selectbox("Ver histórico de", dfl_sorted["name"].head(10).tolist(), key="sens_sel")
                hist = q.q_sensors(window_f, stype if stype!="todos" else None)
                dfh = pd.DataFrame(hist, columns=["ts","name","value","unit"] if stype!="todos" else ["ts","name","value","unit","type"])
                dfh = dfh[dfh["name"]==sel]
                if not dfh.empty:
                    fig = px.line(dfh, x="ts", y="value", title=f"{sel} ao longo do tempo")
                    st.plotly_chart(fig, use_container_width=True)
        else:
            st.info("Sem sensores")

    with tabs[5]:
        st.subheader("Volumes (uso %)")
        disk = q.q_disk_usage()
        if disk:
            df = pd.DataFrame(disk, columns=["device","mount","used_percent","free_gb","ts"])
            df["free_gb"] = df["free_gb"].round(1)
            st.dataframe(df[["device","mount","used_percent","free_gb","ts"]], use_container_width=True)
            fig = px.bar(df, x="device", y="used_percent", title="Uso disco % por volume", text="used_percent", color="used_percent", color_continuous_scale="RdYlGn_r")
            fig.update_layout(height=350)
            st.plotly_chart(fig, use_container_width=True)
        # Physical disks (SSD/HDD/NVMe) - dados de hardware real
        st.subheader("Discos físicos (SSD/HDD/NVMe) - saúde")
        try:
            phys = q.q_physical_disk()
            if phys:
                dfp = pd.DataFrame(phys, columns=["device_id","friendly_name","model","media_type","bus_type","health_status","size_gb","ts"])
                dfp["size_gb"] = dfp["size_gb"].round(0)
                st.dataframe(dfp[["device_id","friendly_name","media_type","bus_type","health_status","size_gb"]], use_container_width=True)
            else:
                st.info("Sem dados physical_disk")
        except Exception as e:
            st.warning(f"physical_disk indisponível: {e}")
        # SMART - temperaturas e saúde (máximo de dados)
        st.subheader("SMART - Temperaturas, horas ligado, wear, erros (máximo)")
        try:
            smart = q.q_disk_smart_latest()
            if smart:
                dfs = pd.DataFrame(smart, columns=["device","model","temp","poh","pcycles","wear","spare","media_err","realloc","pending","passed","ts"])
                # formatação
                dfs["temp"] = dfs["temp"].round(0)
                dfs["wear"] = dfs["wear"].fillna(0)
                st.dataframe(dfs[["device","model","temp","poh","wear","spare","media_err","realloc","pending","passed"]], use_container_width=True)
                # gráfico temperatura discos
                hist = q.q_disk_smart_history(window_f)
                if hist:
                    dfh = pd.DataFrame(hist, columns=["ts","device","temp","poh","wear"])
                    fig = px.line(dfh, x="ts", y="temp", color="device", title="Temperatura discos °C (SMART + NVMe)")
                    fig.update_layout(height=350, yaxis_title="°C")
                    st.plotly_chart(fig, use_container_width=True)
                    fig2 = px.line(dfh, x="ts", y="wear", color="device", title="Desgaste % usado (NVMe percentage_used)")
                    st.plotly_chart(fig2, use_container_width=True)
                # explica outros dados possíveis
                with st.expander("Quais outros dados SMART disponíveis?"):
                    st.markdown("""
                    **Coletados agora (máximo sem poluir):**
                    - `temperature_c` (HDD 40-42C, NVMe 49C via smartctl), `power_on_hours` (SATA 6678-20038h, NVMe 5394h), `power_cycle_count`, `percentage_used` (NVMe wear 2%), `available_spare`, `media_errors`, `reallocated_sectors`, `pending_sectors`, `host_reads/writes`, `data_units_read/written`, `total_lbas`, `smart_passed`, + `raw JSON` completo (todos atributos ATA/NVMe).
                    - **Via Get-PhysicalDisk**: `MediaType` (SSD/HDD/NVMe), `BusType` (SATA/NVMe), `HealthStatus`, `Size`, `Serial`, `Firmware`.
                    - **Via LHM sensors** (já em Sensores): `Storage: Life 98%`, `Data Read 15TB`, `Power On Hours`, etc. (duplicado mas em alta frequência 15s).
                    - **O que mais dá para coletar se quiser (sem limite):** `Spin Up Time`, `Seek Error Rate`, `Load Cycle Count`, `Airflow Temperature`, `UDMA CRC Errors`, `Head Flying Hours`, `NVMe critical_warning`, `unsafe_shutdowns`, `warning_temp_time`, `Controller Busy Time` - já estão em `raw` JSON, basta promover coluna. E `Win32_PerfFormattedData_PerfDisk` (queue/latency/IOPS) já em `disk_io` + `busy_time_ms`.
                    """)
                    # mostra raw exemplo
                    if smart:
                        st.json({"exemplo_raw_keys": "ver tabela disk_smart.raw (JSON completo smartctl -a)"})
            else:
                st.info("Sem dados disk_smart - aguarde coleta 300s ou rode --once")
        except Exception as e:
            st.warning(f"disk_smart indisponível: {e}")

        io = q.q_disk_io(window_f)
        if io:
            dfio = pd.DataFrame(io, columns=["ts","device","read","write"])
            dfio = dfio.sort_values(["device","ts"])
            dfio["read_mb_s"] = dfio.groupby("device")["read"].diff() / 1024/1024 / 10
            dfio["write_mb_s"] = dfio.groupby("device")["write"].diff() / 1024/1024 / 10
            for dev in dfio["device"].unique()[:3]:
                sub = dfio[dfio["device"]==dev].dropna()
                if sub.empty:
                    continue
                fig = go.Figure()
                fig.add_trace(go.Scatter(x=sub["ts"], y=sub["read_mb_s"], name="Read MB/s"))
                fig.add_trace(go.Scatter(x=sub["ts"], y=sub["write_mb_s"], name="Write MB/s"))
                fig.update_layout(title=f"Disco {dev} throughput MB/s (intervalo 10s)", height=300)
                st.plotly_chart(fig, use_container_width=True)
        else:
            st.info("Sem dados disk_io")

    with tabs[6]:
        net = q.q_net(window_f)
        net_latest = q.q_net_latest()
        if net_latest:
            df_latest = pd.DataFrame(net_latest, columns=["iface","recv","sent","ts"])
            df_latest["recv_mb"] = df_latest["recv"]/1024/1024
            df_latest["sent_mb"] = df_latest["sent"]/1024/1024
            st.dataframe(df_latest[["iface","recv_mb","sent_mb","ts"]].round(1), use_container_width=True)
            st.caption("Total acumulado desde boot - bytes enviados/recebidos por interface")
        if net:
            dfn = pd.DataFrame(net, columns=["ts","iface","recv","sent"])
            dfn = dfn.sort_values(["iface","ts"])
            dfn["recv_kbs"] = dfn.groupby("iface")["recv"].diff() / 1024 / 10
            dfn["sent_kbs"] = dfn.groupby("iface")["sent"].diff() / 1024 / 10
            for iface in ["Wi-Fi","ZeroTier One [db64858fed1b6aac]","vEthernet (WSL (Hyper-V firewall))","Ethernet"]:
                if iface not in dfn["iface"].unique():
                    continue
                sub = dfn[dfn["iface"]==iface].dropna()
                if sub.empty or (sub["recv_kbs"].abs().max() < 1 and sub["sent_kbs"].abs().max() < 1):
                    continue
                fig = go.Figure()
                fig.add_trace(go.Scatter(x=sub["ts"], y=sub["recv_kbs"], name="RX KB/s"))
                fig.add_trace(go.Scatter(x=sub["ts"], y=sub["sent_kbs"], name="TX KB/s"))
                fig.update_layout(title=f"Rede {iface} throughput KB/s (intervalo 10s)", height=300)
                st.plotly_chart(fig, use_container_width=True)
            st.caption("Gráficos acima são taxa (delta 10s). Abaixo total acumulado MB")
            for iface in dfn["iface"].unique()[:2]:
                sub = dfn[dfn["iface"]==iface]
                sub["recv_mb"] = sub["recv"]/1024/1024
                sub["sent_mb"] = sub["sent"]/1024/1024
                fig = px.line(sub, x="ts", y="recv_mb", title=f"{iface} total RX MB")
                st.plotly_chart(fig, use_container_width=True)
            st.dataframe(dfn.tail(20))
        else:
            st.info("Sem dados rede")

    with tabs[7]:
        procs = q.q_processes()
        if procs:
            dfp = pd.DataFrame(procs, columns=["name","pid","cpu","mem","rss_mb","user"])
            st.dataframe(dfp, use_container_width=True)
            fig = px.bar(dfp.head(10), x="name", y="cpu", title="Top CPU %")
            st.plotly_chart(fig, use_container_width=True)
            fig2 = px.bar(dfp.head(10), x="name", y="rss_mb", title="Top RSS MB")
            st.plotly_chart(fig2, use_container_width=True)
        else:
            st.info("Sem processos")

    with tabs[8]:
        sys_row2 = q.q_system()
        if sys_row2:
            st.json({"hostname": sys_row2[1], "uptime_s": sys_row2[2], "cpu": sys_row2[3], "os_build": sys_row2[4], "ram_gb": round(sys_row2[5],1)})
        ev = q.q_eventlog(window_f)
        if ev:
            dfe = pd.DataFrame(ev, columns=["log","level","provider","id","count","msg"])
            st.dataframe(dfe, use_container_width=True)
        hb = q.q_heartbeat()
        if hb:
            st.dataframe(pd.DataFrame(hb, columns=["host","collector","ts","success","error"]))

render_content()

st.sidebar.caption("DB: system_monitor @ localhost:5432 | Histórico permanente (ENABLE_RETENTION=false)")
