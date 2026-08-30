-- Queries úteis para dashboard / verificação

-- 1. CPU últimos 10 minutos (média por minuto)
SELECT date_trunc('minute', ts) as minuto, avg(cpu_total_percent)::int as cpu_avg
FROM monitor.cpu WHERE ts > now() - interval '10 minutes' GROUP BY 1 ORDER BY 1;

-- 2. Memória última hora
SELECT ts, used_percent, used_bytes/1024/1024/1024 as used_gb FROM monitor.memory ORDER BY ts DESC LIMIT 20;

-- 3. Disco uso atual por volume
SELECT DISTINCT ON (device) device, mountpoint, used_percent, free_bytes/1024/1024/1024 as free_gb, ts
FROM monitor.disk_usage ORDER BY device, ts DESC;

-- 4. Rede throughput últimos 5 min (delta)
SELECT iface, ts, bytes_recv, bytes_sent FROM monitor.net_io WHERE iface='Wi-Fi' ORDER BY ts DESC LIMIT 10;

-- 5. GPU temp e uso
SELECT ts, name, temperature_gpu_c, utilization_gpu_percent, power_draw_w, memory_used_bytes/1024/1024 as vram_mb FROM monitor.gpu ORDER BY ts DESC LIMIT 10;

-- 6. Top 10 processos agora
SELECT name, pid, cpu_percent, memory_percent, memory_rss_bytes/1024/1024 as rss_mb, cmdline
FROM monitor.processes WHERE ts = (SELECT max(ts) FROM monitor.processes) ORDER BY cpu_percent DESC LIMIT 10;

-- 7. Serviços parados (última coleta)
SELECT name, display_name, status, start_type FROM monitor.services WHERE ts = (SELECT max(ts) FROM monitor.services) AND status != 'Running' ORDER BY name LIMIT 20;

-- 8. EventLog erros última hora
SELECT log_name, level, provider, event_id, count, latest_message FROM monitor.eventlog WHERE ts > now() - interval '1 hour' AND level='Error' ORDER BY count DESC;

-- 9. Heartbeat - coletores com falha
SELECT * FROM monitor.v_last_heartbeat WHERE success=false;

-- 10. Tamanho das tabelas
SELECT schemaname, tablename, pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size, (SELECT count(*) FROM monitor.cpu) as rows_cpu FROM pg_tables WHERE schemaname='monitor' ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- Limpeza retenção
-- DELETE FROM monitor.processes WHERE ts < now() - INTERVAL '30 days';
-- DELETE FROM monitor.connections WHERE ts < now() - INTERVAL '7 days';
