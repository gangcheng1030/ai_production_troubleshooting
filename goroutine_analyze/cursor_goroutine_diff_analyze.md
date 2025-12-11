# 分析goroutine上涨原因
_Exported on 2025/12/11 at GMT+8 19:25:32 from Cursor (2.1.46)_

---

**User**

@goroutine_analyze/good_goroutine.txt  是goroutine上涨前通过pprof得到的goroutine信息， @goroutine_analyze/bad_goroutine.txt  是goroutine上涨后通过pprof得到的goroutine信息，请帮忙分析goroutine上涨的可能原因。

---

**Cursor**

我来帮你分析这两个goroutine pprof文件，找出goroutine上涨的原因。

## Goroutine 泄漏分析报告

分析完成！我发现了严重的 **gRPC 连接泄漏问题**。

### 📊 关键数据对比

| 指标 | 正常状态 (good) | 异常状态 (bad) | 增长倍数 |
|------|----------------|----------------|----------|
| **总 Goroutine 数** | 8 | 1508 | 188.5x |
| **gRPC HandleStreams** | 1 | 501 | 501x |
| **gRPC loopyWriter** | 1 | 501 | 501x |
| **gRPC keepalive** | 1 | 501 | 501x |

### 🔍 根本原因

**存在 501 个未正确释放的 gRPC 连接**

每个 gRPC 连接在服务端会创建 **3 个 goroutine**：

1. **HandleStreams** (line 2-16) - 处理 gRPC 流请求
   - 堆栈：`google.golang.org/grpc.(*Server).serveStreams` → `(*http2Server).HandleStreams`
   - 阻塞在：`ReadFrame` 等待客户端数据

2. **loopyWriter** (line 18-21) - 处理控制缓冲区写入
   - 堆栈：`(*controlBuffer).get` → `(*loopyWriter).run`
   - 阻塞在：等待控制消息

3. **keepalive** (line 23-24) - 维持连接心跳
   - 堆栈：`(*http2Server).keepalive`
   - 阻塞在：定时检查

### 💡 可能的问题场景

1. **客户端连接泄漏**
   - 客户端创建连接后没有正确关闭 (`conn.Close()`)
   - 使用了连接池但池大小配置不当
   - 客户端异常退出但 TCP 连接未断开

2. **服务端配置问题**
   - gRPC 服务端未配置合理的超时参数
   - `MaxConnectionIdle` 未设置
   - `MaxConnectionAge` 未设置
   - `KeepAlive` 策略配置不当

3. **网络问题**
   - 客户端和服务端之间存在网络分区
   - 防火墙导致半开连接
   - TCP 连接处于 CLOSE_WAIT 状态

### 🛠️ 建议的解决方案

#### 1. **服务端添加连接管理配置**

```go
import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/keepalive"
    "time"
)

grpcServer := grpc.NewServer(
    grpc.KeepaliveParams(keepalive.ServerParameters{
        MaxConnectionIdle:     15 * time.Minute,  // 空闲连接最大存活时间
        MaxConnectionAge:      30 * time.Minute,  // 连接最大存活时间
        MaxConnectionAgeGrace: 5 * time.Minute,   // 强制关闭前的宽限期
        Time:                  5 * time.Minute,   // ping 间隔
        Timeout:               20 * time.Second,  // ping 超时
    }),
    grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
        MinTime:             5 * time.Minute,     // 客户端最小 ping 间隔
        PermitWithoutStream: true,                // 允许无流时发送 ping
    }),
)
```

#### 2. **客户端确保正确关闭连接**

```go
conn, err := grpc.Dial(address, grpc.WithInsecure())
if err != nil {
    log.Fatal(err)
}
defer conn.Close() // 确保关闭！

// 使用连接池时
client := pb.NewYourServiceClient(conn)
// ... 使用完毕后
conn.Close()
```

#### 3. **排查工具**

```bash
# 查看 TCP 连接状态
netstat -an | grep :50051 | awk '{print $6}' | sort | uniq -c

# 或使用 ss 命令
ss -tan | grep :50051 | awk '{print $1}' | sort | uniq -c

# 监控 goroutine 数量变化
watch -n 1 'curl -s http://localhost:6060/debug/pprof/goroutine?debug=1 | head -1'
```

#### 4. **添加监控告警**

建议对以下指标添加告警：
- Goroutine 数量超过阈值（如 100）
- gRPC 连接数异常增长
- 内存持续增长

### 📝 下一步行动

1. ✅ 检查客户端代码，确认是否有 `defer conn.Close()`
2. ✅ 在 gRPC 服务端添加 `keepalive` 配置
3. ✅ 检查网络层是否有连接追踪问题
4. ✅ 添加 goroutine 和连接数监控
5. ✅ 考虑在高峰期限制并发连接数

需要我帮你查看服务端的配置代码，或者生成完整的修复方案吗？

```go
import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/keepalive"
    "time"
)

grpcServer := grpc.NewServer(
    grpc.KeepaliveParams(keepalive.ServerParameters{
        MaxConnectionIdle:     15 * time.Minute,  // 空闲连接最大存活时间
        MaxConnectionAge:      30 * time.Minute,  // 连接最大存活时间
        MaxConnectionAgeGrace: 5 * time.Minute,   // 强制关闭前的宽限期
        Time:                  5 * time.Minute,   // ping 间隔
        Timeout:               20 * time.Second,  // ping 超时
    }),
    grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
        MinTime:             5 * time.Minute,     // 客户端最小 ping 间隔
        PermitWithoutStream: true,                // 允许无流时发送 ping
    }),
)
```

```go
conn, err := grpc.Dial(address, grpc.WithInsecure())
if err != nil {
    log.Fatal(err)
}
defer conn.Close() // 确保关闭！

// 使用连接池时
client := pb.NewYourServiceClient(conn)
// ... 使用完毕后
conn.Close()
```

```shellscript
# 查看 TCP 连接状态
netstat -an | grep :50051 | awk '{print $6}' | sort | uniq -c

# 或使用 ss 命令
ss -tan | grep :50051 | awk '{print $1}' | sort | uniq -c

# 监控 goroutine 数量变化
watch -n 1 'curl -s http://localhost:6060/debug/pprof/goroutine?debug=1 | head -1'
```

