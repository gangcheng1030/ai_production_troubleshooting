# PostgreSQL主从切换后性能问题
_Exported on 2025/12/16 at GMT+8 15:47:57 from Cursor (2.1.46)_

---

**User**

问题背景：业务读写都在PostgreSQL主库，进行主从切换后，新的主库CPU使用率飙升，磁盘读流量飙升，磁盘io time增加，业务读写超时严重，请帮忙分析可能的原因。

---

**Cursor**

我来帮你分析PostgreSQL主从切换后新主库性能问题的可能原因。这是一个典型的生产环境数据库故障场景。

让我创建一个详细的分析文档：

```markdown
# PostgreSQL主从切换后性能问题分析

## 问题现象

**背景**：业务读写都在PostgreSQL主库，进行主从切换后出现以下问题：

- ❌ 新主库CPU使用率飙升
- ❌ 磁盘读流量飙升  
- ❌ 磁盘IO time增加
- ❌ 业务读写超时严重

## 根因分析

### 1. 缓存冷启动问题（最可能的原因）⭐⭐⭐⭐⭐

#### 现象分析
- **磁盘读流量飙升**：说明数据需要从磁盘读取，而不是从内存读取
- **CPU飙升**：大量磁盘IO导致CPU等待，同时查询排队增加CPU使用
- **IO time增加**：直接证明磁盘IO压力大

#### 根本原因
从库提升为主库后，**shared_buffers和OS page cache都是冷的**：

```sql
-- 查看缓存命中率
SELECT 
    sum(heap_blks_read) as heap_read,
    sum(heap_blks_hit) as heap_hit,
    sum(heap_blks_hit) / (sum(heap_blks_hit) + sum(heap_blks_read)) as cache_hit_ratio
FROM pg_statio_user_tables;

-- 正常情况下 cache_hit_ratio 应该 > 0.99
-- 切换后可能降到 0.7-0.8 甚至更低
```

**为什么会这样？**
1. 原主库经过长时间运行，热数据都在内存中
2. 从库虽然有数据复制，但**缓存不会复制**
3. 切换后，所有查询需要重新从磁盘加载数据到内存

#### 解决方案

**临时缓解：**
```bash
# 1. 手动预热关键表（切换前/后立即执行）
psql -c "SELECT pg_prewarm('your_hot_table');"
psql -c "SELECT pg_prewarm('your_hot_table_pkey');"

# 2. 使用pg_prewarm扩展批量预热
CREATE EXTENSION IF NOT EXISTS pg_prewarm;

-- 预热所有大于100MB的表
SELECT pg_prewarm(oid) 
FROM pg_class 
WHERE relkind = 'r' 
  AND pg_relation_size(oid) > 100 * 1024 * 1024;
```

**长期优化：**
```sql
-- 1. 增加shared_buffers（需重启，建议为物理内存的25%）
shared_buffers = 8GB

-- 2. 调整有效缓存大小（不需重启）
effective_cache_size = 24GB  -- 建议为物理内存的50-75%

-- 3. 启用huge pages（Linux）减少TLB miss
huge_pages = try
```

---

### 2. 查询计划问题（统计信息过期）⭐⭐⭐⭐

#### 现象分析
从库的统计信息可能不准确，导致查询优化器选择错误的执行计划。

#### 根本原因
```sql
-- 从库默认情况下 autovacuum 和 autoanalyze 是开启的
-- 但统计信息更新可能落后于主库

-- 检查统计信息更新时间
SELECT 
    schemaname,
    relname,
    last_vacuum,
    last_autovacuum,
    last_analyze,
    last_autoanalyze,
    n_live_tup,
    n_dead_tup
FROM pg_stat_user_tables
ORDER BY n_live_tup DESC;
```

**可能导致的问题：**
- 选择全表扫描而不是索引扫描
- JOIN顺序错误
- 错误的JOIN算法（Hash Join vs Nested Loop）

#### 解决方案

```bash
# 切换后立即执行
# 1. 对所有表重新收集统计信息
vacuumdb -z -d your_database

# 2. 或者针对关键表
psql -c "ANALYZE your_hot_table;"

# 3. 对于特别大的表，使用采样分析
psql -c "ANALYZE your_huge_table (id, created_at);"  -- 只分析关键列
```

**监控SQL执行计划变化：**
```sql
-- 安装pg_stat_statements
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- 找出执行时间突然变长的查询
SELECT 
    query,
    calls,
    mean_exec_time,
    max_exec_time,
    stddev_exec_time
FROM pg_stat_statements
WHERE mean_exec_time > 1000  -- 大于1秒
ORDER BY mean_exec_time DESC
LIMIT 20;
```

---

### 3. 索引膨胀/损坏问题 ⭐⭐⭐

#### 现象分析
从库的索引可能存在膨胀或不一致状态。

#### 根本原因
```sql
-- 检查索引膨胀情况
SELECT
    schemaname,
    tablename,
    indexname,
    pg_size_pretty(pg_relation_size(indexrelid)) as index_size,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes
WHERE idx_scan = 0  -- 从未使用的索引
   OR pg_relation_size(indexrelid) > 100 * 1024 * 1024  -- 大于100MB
ORDER BY pg_relation_size(indexrelid) DESC;

-- 检查索引膨胀率
SELECT
    schemaname || '.' || tablename AS table,
    indexname AS index,
    pg_size_pretty(pg_relation_size(indexrelid)) AS index_size,
    idx_scan,
    ROUND(100.0 * pg_relation_size(indexrelid) / pg_relation_size(relid), 2) AS index_ratio
FROM pg_stat_user_indexes
JOIN pg_class ON pg_class.oid = indexrelid
ORDER BY pg_relation_size(indexrelid) DESC;
```

#### 解决方案

```sql
-- 1. 重建膨胀的索引（CONCURRENTLY避免锁表）
REINDEX INDEX CONCURRENTLY your_index_name;

-- 2. 批量重建表的所有索引
REINDEX TABLE CONCURRENTLY your_table_name;

-- 3. 对于非常大的表，先删除再创建
DROP INDEX CONCURRENTLY old_index;
CREATE INDEX CONCURRENTLY new_index ON table_name(column);
```

---

### 4. 连接数暴增问题 ⭐⭐⭐

#### 现象分析
主从切换后，所有应用连接可能同时涌向新主库。

#### 检查方法
```sql
-- 查看当前连接数
SELECT 
    state,
    count(*),
    max(now() - query_start) as max_duration
FROM pg_stat_activity
WHERE state IS NOT NULL
GROUP BY state;

-- 查看等待的连接
SELECT 
    pid,
    usename,
    application_name,
    client_addr,
    state,
    wait_event_type,
    wait_event,
    query
FROM pg_stat_activity
WHERE wait_event IS NOT NULL
  AND state = 'active'
ORDER BY query_start;

-- 检查连接池配置
SHOW max_connections;
SHOW superuser_reserved_connections;
```

#### 解决方案

```sql
-- 1. 临时增加最大连接数（需重启）
max_connections = 500  -- 根据实际情况调整

-- 2. 使用连接池（PgBouncer）
-- pgbouncer.ini
[databases]
your_db = host=localhost dbname=your_db

[pgbouncer]
pool_mode = transaction  -- 或 session
max_client_conn = 1000
default_pool_size = 25   -- 每个数据库的连接数
reserve_pool_size = 5

-- 3. 调整工作进程数
max_worker_processes = 16
max_parallel_workers = 8
```

---

### 5. 锁等待和长事务 ⭐⭐⭐

#### 检查方法
```sql
-- 查看锁等待
SELECT 
    blocked_locks.pid AS blocked_pid,
    blocked_activity.usename AS blocked_user,
    blocking_locks.pid AS blocking_pid,
    blocking_activity.usename AS blocking_user,
    blocked_activity.query AS blocked_statement,
    blocking_activity.query AS blocking_statement,
    blocked_activity.application_name AS blocked_application
FROM pg_catalog.pg_locks blocked_locks
JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
JOIN pg_catalog.pg_locks blocking_locks 
    ON blocking_locks.locktype = blocked_locks.locktype
    AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
    AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
    AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
    AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
    AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
    AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
    AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
    AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
    AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
    AND blocking_locks.pid != blocked_locks.pid
JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
WHERE NOT blocked_locks.granted;

-- 查看长事务
SELECT 
    pid,
    usename,
    application_name,
    state,
    now() - xact_start AS xact_duration,
    query
FROM pg_stat_activity
WHERE state != 'idle'
  AND (now() - xact_start) > interval '5 minutes'
ORDER BY xact_start;
```

#### 解决方案
```sql
-- 1. 设置语句超时
statement_timeout = 30000  -- 30秒
lock_timeout = 10000       -- 10秒

-- 2. 终止长事务（谨慎使用）
SELECT pg_terminate_backend(pid) 
FROM pg_stat_activity 
WHERE state != 'idle'
  AND (now() - xact_start) > interval '10 minutes';

-- 3. 设置空闲事务超时（PostgreSQL 14+）
idle_in_transaction_session_timeout = 300000  -- 5分钟
```

---

### 6. WAL和复制配置问题 ⭐⭐

#### 检查方法
```sql
-- 检查WAL配置
SHOW wal_level;
SHOW max_wal_senders;
SHOW max_replication_slots;

-- 查看复制槽
SELECT * FROM pg_replication_slots;

-- 查看WAL发送进程
SELECT * FROM pg_stat_replication;

-- 检查归档状态
SELECT * FROM pg_stat_archiver;
```

#### 可能的问题
1. 旧主库的复制槽未清理，占用WAL空间
2. 归档配置不当，导致WAL堆积
3. synchronous_commit配置不当

#### 解决方案
```sql
-- 1. 清理无用的复制槽
SELECT pg_drop_replication_slot('old_standby_slot');

-- 2. 调整同步复制配置
synchronous_commit = local  -- 或 off（根据业务容忍度）

-- 3. 调整WAL配置
wal_buffers = 16MB
checkpoint_timeout = 15min
checkpoint_completion_target = 0.9
```

---

### 7. 配置参数不一致 ⭐⭐

#### 检查方法
```bash
# 比较主从配置文件
diff /path/to/old_master/postgresql.conf /path/to/new_master/postgresql.conf

# 查看运行时配置
psql -c "SELECT name, setting, unit FROM pg_settings WHERE source != 'default' ORDER BY name;"
```

#### 常见问题配置
```sql
-- 1. 工作内存不足
work_mem = 64MB  -- 单个操作的内存，根据查询复杂度调整
maintenance_work_mem = 1GB  -- VACUUM、CREATE INDEX使用

-- 2. 随机页成本
random_page_cost = 1.1  -- SSD应该设置为1.1-1.3

-- 3. 并行查询
max_parallel_workers_per_gather = 4
parallel_setup_cost = 1000
parallel_tuple_cost = 0.1

-- 4. 预写日志
wal_compression = on  -- 减少WAL大小
wal_writer_delay = 200ms
```

---

## 排查流程

### 第一步：快速评估（5分钟内）

```bash
# 1. 查看缓存命中率
psql -c "
SELECT 
    sum(heap_blks_hit) / nullif(sum(heap_blks_hit) + sum(heap_blks_read), 0)::float as cache_hit_ratio
FROM pg_statio_user_tables;
"

# 2. 查看TOP慢查询
psql -c "
SELECT 
    substring(query, 1, 100) as query,
    calls,
    round(mean_exec_time::numeric, 2) as mean_ms,
    round(total_exec_time::numeric, 2) as total_ms
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;
"

# 3. 查看连接数
psql -c "SELECT count(*), state FROM pg_stat_activity GROUP BY state;"

# 4. 查看锁等待
psql -c "SELECT count(*) FROM pg_locks WHERE NOT granted;"
```

### 第二步：立即处理（10分钟内）

```bash
# 1. 预热关键表
psql -c "
DO \$\$
DECLARE
    r RECORD;
BEGIN
    FOR r IN 
        SELECT schemaname, tablename 
        FROM pg_tables 
        WHERE schemaname = 'public' 
          AND pg_relation_size(quote_ident(schemaname) || '.' || quote_ident(tablename)) > 100*1024*1024
    LOOP
        EXECUTE 'SELECT pg_prewarm(''' || quote_ident(r.schemaname) || '.' || quote_ident(r.tablename) || ''')';
    END LOOP;
END;
\$\$;
"

# 2. 更新统计信息
vacuumdb -z -j 4 your_database  # -j 4 表示4个并行任务

# 3. 终止长事务
psql -c "
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE state != 'idle'
  AND (now() - xact_start) > interval '10 minutes'
  AND query NOT LIKE '%pg_terminate_backend%';
"
```

### 第三步：深度分析（30分钟内）

```bash
# 1. 生成系统报告
psql -c "
-- 磁盘IO统计
SELECT 
    schemaname,
    tablename,
    heap_blks_read,
    heap_blks_hit,
    idx_blks_read,
    idx_blks_hit,
    round(100.0 * heap_blks_hit / nullif(heap_blks_hit + heap_blks_read, 0), 2) as cache_hit_ratio
FROM pg_statio_user_tables
WHERE heap_blks_read + heap_blks_hit > 0
ORDER BY heap_blks_read DESC
LIMIT 20;
"

# 2. 检查索引使用情况
psql -c "
SELECT 
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch,
    pg_size_pretty(pg_relation_size(indexrelid)) as size
FROM pg_stat_user_indexes
WHERE idx_scan < 100  -- 很少使用的索引
  AND pg_relation_size(indexrelid) > 10*1024*1024  -- 大于10MB
ORDER BY pg_relation_size(indexrelid) DESC;
"

# 3. 分析表膨胀
psql -c "
SELECT 
    schemaname,
    tablename,
    n_dead_tup,
    n_live_tup,
    round(100.0 * n_dead_tup / nullif(n_live_tup + n_dead_tup, 0), 2) as dead_ratio,
    last_vacuum,
    last_autovacuum
FROM pg_stat_user_tables
WHERE n_dead_tup > 10000
ORDER BY n_dead_tup DESC;
"
```

---

## 预防措施

### 1. 从库预热机制

**方案A：使用pg_prewarm + cron**
```sql
-- 创建预热函数
CREATE OR REPLACE FUNCTION prewarm_important_tables()
RETURNS void AS $$
DECLARE
    tables text[] := ARRAY['users', 'orders', 'products', 'inventory'];
    tbl text;
BEGIN
    FOREACH tbl IN ARRAY tables
    LOOP
        PERFORM pg_prewarm(tbl::regclass);
        RAISE NOTICE 'Prewarmed table: %', tbl;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- 在从库上设置定期预热（crontab）
*/30 * * * * psql -U postgres -d your_db -c "SELECT prewarm_important_tables();"
```

**方案B：应用层预热**
```bash
#!/bin/bash
# standby_prewarm.sh - 在从库上定期执行

TABLES=(
    "users"
    "orders"
    "products"
    "user_sessions"
)

for table in "${TABLES[@]}"; do
    echo "Prewarming $table..."
    psql -c "SELECT count(*) FROM $table;" > /dev/null
    psql -c "SELECT pg_prewarm('$table');" > /dev/null
done
```

### 2. 主从切换检查清单

```bash
#!/bin/bash
# pre_failover_check.sh

echo "=== Pre-Failover Checklist ==="

# 1. 检查复制延迟
echo "1. Replication lag:"
psql -h standby -c "SELECT now() - pg_last_xact_replay_timestamp() AS replication_lag;"

# 2. 检查从库配置
echo "2. Standby configuration:"
diff <(ssh old_master "grep -v '^#' /etc/postgresql/postgresql.conf | grep -v '^$'") \
     <(ssh new_master "grep -v '^#' /etc/postgresql/postgresql.conf | grep -v '^$'")

# 3. 预热从库
echo "3. Prewarming standby:"
psql -h standby -c "
SELECT pg_prewarm(oid) 
FROM pg_class 
WHERE relkind = 'r' 
  AND pg_relation_size(oid) > 50*1024*1024;
"

# 4. 更新统计信息
echo "4. Updating statistics on standby:"
# 注意：从库默认只读，需要特殊处理

# 5. 检查空闲连接
echo "5. Idle connections:"
psql -h old_master -c "SELECT count(*) FROM pg_stat_activity WHERE state = 'idle';"
```

### 3. 监控告警

**Prometheus + Grafana监控**
```yaml
# postgresql_exporter
scrape_configs:
  - job_name: 'postgresql'
    static_configs:
      - targets: ['postgres:9187']

# 关键指标告警
groups:
  - name: postgresql
    rules:
      - alert: PostgreSQLCacheHitRatioLow
        expr: pg_stat_database_blks_hit / (pg_stat_database_blks_hit + pg_stat_database_blks_read) < 0.95
        for: 5m
        
      - alert: PostgreSQLHighIOWait
        expr: rate(node_disk_io_time_seconds_total[5m]) > 0.8
        for: 5m
        
      - alert: PostgreSQLLongTransaction
        expr: pg_stat_activity_max_tx_duration > 600
        for: 5m
```

---

## 总结

根据症状分析，**最可能的原因是缓存冷启动问题**，建议按以下优先级处理：

### 优先级排序

1. **P0 - 立即执行（5分钟内）**
   - 预热关键表（pg_prewarm）
   - 更新统计信息（ANALYZE）
   - 终止长事务

2. **P1 - 紧急处理（30分钟内）**
   - 检查并优化慢查询
   - 重建膨胀的索引
   - 调整连接池配置

3. **P2 - 重要优化（2小时内）**
   - 调整数据库参数（shared_buffers、work_mem等）
   - 清理无用的复制槽
   - 检查表膨胀并执行VACUUM

4. **P3 - 长期改进**
   - 建立从库预热机制
   - 完善监控告警
   - 制定主从切换SOP

### 关键命令速查

```bash
# 快速诊断三板斧
psql -c "SELECT sum(heap_blks_hit) / nullif(sum(heap_blks_hit) + sum(heap_blks_read), 0) FROM pg_statio_user_tables;"  # 缓存命中率
psql -c "SELECT count(*), state FROM pg_stat_activity GROUP BY state;"  # 连接状态
psql -c "SELECT * FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 10;"  # TOP慢查询

# 快速恢复三板斧
psql -c "SELECT pg_prewarm(oid) FROM pg_class WHERE relkind='r' AND pg_relation_size(oid) > 100*1024*1024;"  # 预热
vacuumdb -z -j 4 your_database  # 更新统计
psql -c "REINDEX TABLE CONCURRENTLY your_hot_table;"  # 重建索引
```

### 预期恢复时间
- 缓存预热：10-30分钟（取决于数据量）
- 统计信息更新：5-15分钟
- 业务恢复正常：30-60分钟

---

## 参考资料

- PostgreSQL官方文档：https://www.postgresql.org/docs/current/monitoring-stats.html
- pg_prewarm扩展：https://www.postgresql.org/docs/current/pgprewarm.html
- PgBouncer连接池：https://www.pgbouncer.org/
- 性能调优指南：https://wiki.postgresql.org/wiki/Performance_Optimization

```

我已经为你创建了一份详细的PostgreSQL主从切换性能问题分析文档。根据你描述的症状（CPU飙升、磁盘读流量飙升、IO time增加），我分析出**最可能的根本原因是缓存冷启动问题**。

## 核心问题分析

**为什么会出现这个问题？**

从库提升为主库后，虽然数据是完整的，但：
- ❌ PostgreSQL的`shared_buffers`（共享缓冲区）是空的
- ❌ 操作系统的page cache是冷的  
- ❌ 所有热数据需要重新从磁盘加载到内存

这就像：原来的主库是"热车"状态，所有常用数据都在内存里；新主库是"冷车"状态，每次查询都要从磁盘读取，导致：
- 磁盘IO暴增 → 磁盘读流量飙升
- 大量IO等待 → IO time增加  
- 查询排队堆积 → CPU飙升
- 响应时间变长 → 业务超时

## 快速恢复方案（立即执行）

```bash
# 1. 预热关键表（最重要！）
psql -c "SELECT pg_prewarm(oid) FROM pg_class 
WHERE relkind='r' AND pg_relation_size(oid) > 100*1024*1024;"

# 2. 更新统计信息
vacuumdb -z -j 4 your_database

# 3. 终止长事务
psql -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity 
WHERE state != 'idle' AND (now() - xact_start) > interval '10 minutes';"
```

## 其他可能原因

文档中还详细分析了其他6个可能的原因：
2. 查询计划问题（统计信息过期）
3. 索引膨胀/损坏
4. 连接数暴增
5. 锁等待和长事务
6. WAL和复制配置问题  
7. 配置参数不一致

文档提供了完整的排查流程、检查命令、解决方案和预防措施。你可以根据实际情况逐步排查。预期30-60分钟内业务应该能恢复正常。

需要我进一步解释某个具体问题的排查方法吗？

```shellscript
# 1. 预热关键表（最重要！）
psql -c "SELECT pg_prewarm(oid) FROM pg_class 
WHERE relkind='r' AND pg_relation_size(oid) > 100*1024*1024;"

# 2. 更新统计信息
vacuumdb -z -j 4 your_database

# 3. 终止长事务
psql -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity 
WHERE state != 'idle' AND (now() - xact_start) > interval '10 minutes';"
```

