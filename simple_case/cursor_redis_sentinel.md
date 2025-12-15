# Redis get 命令错误分析
_Exported on 2025/12/15 at GMT+8 10:29:59 from Cursor (2.1.46)_

---

**User**

使用 "redis-cli -h redis-test-st-1 -p 26379 -a redis-sentinel-st"连接redis，执行 "get ddd"命令时报错："ERR unknown command `get`"，请帮忙分析问题原因。

---

**Cursor**

您遇到的问题原因很明确：**您连接的是 Redis Sentinel，而不是 Redis 数据实例**。

## 问题分析

从您的连接命令可以看出：

```bash
redis-cli -h redis-test-st-1 -p 26379 -a redis-sentinel-st
```

**端口 26379 是 Redis Sentinel 的默认端口**，而不是 Redis 数据库实例的端口（通常是 6379）。

### Redis Sentinel vs Redis 实例的区别

- **Redis Sentinel（26379）**：高可用监控服务，只支持管理命令，如：
  - `SENTINEL masters`
  - `SENTINEL get-master-addr-by-name <master-name>`
  - `SENTINEL slaves <master-name>`
  - 等等

- **Redis 实例（通常 6379）**：实际的数据库实例，支持数据操作命令，如：
  - `GET`、`SET`、`DEL` 等

所以当您在 Sentinel 上执行 `get ddd` 时，Sentinel 无法识别这个命令，因为它不是数据库实例。

## 解决方案

### 方案 1：通过 Sentinel 查询主节点地址

首先连接到 Sentinel，查询主节点的地址：

```bash
redis-cli -h redis-test-st-1 -p 26379 -a redis-sentinel-st
# 然后执行
SENTINEL masters
# 或者
SENTINEL get-master-addr-by-name <your-master-name>
```

### 方案 2：直接连接 Redis 实例

如果您知道 Redis 实例的地址，直接连接到数据端口（通常是 6379）：

```bash
redis-cli -h <redis-实例地址> -p 6379 -a <redis-密码>
# 然后执行
get ddd
```

### 方案 3：使用 Sentinel 自动发现主节点

如果您的客户端支持 Sentinel，可以配置客户端自动从 Sentinel 获取主节点地址并连接。

**总结**：您需要连接到实际的 Redis 数据实例（端口 6379），而不是 Sentinel 服务（端口 26379），才能执行 `GET` 等数据操作命令。

```shellscript
redis-cli -h redis-test-st-1 -p 26379 -a redis-sentinel-st
```

```shellscript
redis-cli -h redis-test-st-1 -p 26379 -a redis-sentinel-st
# 然后执行
SENTINEL masters
# 或者
SENTINEL get-master-addr-by-name <your-master-name>
```

```shellscript
redis-cli -h <redis-实例地址> -p 6379 -a <redis-密码>
# 然后执行
get ddd
```

