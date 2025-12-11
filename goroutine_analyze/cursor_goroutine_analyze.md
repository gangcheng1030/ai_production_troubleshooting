# 分析goroutine上涨原因
_Exported on 2025/12/11 at GMT+8 19:33:22 from Cursor (2.1.46)_

---

**User**

@goroutine_analyze/bad_goroutine.txt  是goroutine上涨后通过pprof获取的goroutine信息，请帮忙分析导致goroutine上涨的可能原因。

---

**Cursor**

我来帮您分析这个 goroutine 信息文件，找出导致 goroutine 上涨的原因。

## Goroutine 上涨原因分析

根据 pprof 数据，我发现了一个**严重的 gRPC 连接泄漏问题**。

### 🔍 关键发现

**总 goroutine 数：1508**

其中 **1503 个 goroutine (99.7%)** 都与 gRPC 连接相关：

- **501 个** 在 `grpc.(*Server).serveStreams` - 等待读取 HTTP/2 frame
- **501 个** 在 `grpc/internal/transport.(*loopyWriter).run` - 等待写入控制帧
- **501 个** 在 `grpc/internal/transport.(*http2Server).keepalive` - 维护 keepalive

```1:16:goroutine_analyze/bad_goroutine.txt
goroutine profile: total 1508
501 @ 0x54b18ee 0x5474cd7 0x54b0c25 0x5527ba7 0x5528e9a 0x5528e88 0x557ec05 0x558e105 0x56c6557 0x5523430 0x58956e5 0x58956b3 0x5895e25 0x58bf307 0x592b476 0x592ac16 0x54b9e21
#	0x54b0c24	internal/poll.runtime_pollWait+0x84						/Users/chenggang/sdk/go1.23.9/src/runtime/netpoll.go:351
#	0x5527ba6	internal/poll.(*pollDesc).wait+0x26						/Users/chenggang/sdk/go1.23.9/src/internal/poll/fd_poll_runtime.go:84
#	0x5528e99	internal/poll.(*pollDesc).waitRead+0x279					/Users/chenggang/sdk/go1.23.9/src/internal/poll/fd_poll_runtime.go:89
#	0x5528e87	internal/poll.(*FD).Read+0x267							/Users/chenggang/sdk/go1.23.9/src/internal/poll/fd_unix.go:165
#	0x557ec04	net.(*netFD).Read+0x24								/Users/chenggang/sdk/go1.23.9/src/net/fd_posix.go:55
#	0x558e104	net.(*conn).Read+0x44								/Users/chenggang/sdk/go1.23.9/src/net/net.go:189
#	0x56c6556	bufio.(*Reader).Read+0x196							/Users/chenggang/sdk/go1.23.9/src/bufio/bufio.go:241
#	0x552342f	io.ReadAtLeast+0x8f								/Users/chenggang/sdk/go1.23.9/src/io/io.go:335
#	0x58956e4	io.ReadFull+0x64								/Users/chenggang/sdk/go1.23.9/src/io/io.go:354
#	0x58956b2	golang.org/x/net/http2.readFrameHeader+0x32					/Users/chenggang/go/pkg/mod/golang.org/x/net@v0.32.0/http2/frame.go:237
#	0x5895e24	golang.org/x/net/http2.(*Framer).ReadFrame+0x84					/Users/chenggang/go/pkg/mod/golang.org/x/net@v0.32.0/http2/frame.go:501
#	0x58bf306	google.golang.org/grpc/internal/transport.(*http2Server).HandleStreams+0x106	/Users/chenggang/go/pkg/mod/google.golang.org/grpc@v1.70.0/internal/transport/http2_server.go:640
#	0x592b475	google.golang.org/grpc.(*Server).serveStreams+0x395				/Users/chenggang/go/pkg/mod/google.golang.org/grpc@v1.70.0/server.go:1024
#	0x592ac15	google.golang.org/grpc.(*Server).handleRawConn.func1+0x55			/Users/chenggang/go/pkg/mod/google.golang.org/grpc@v1.70.0/server.go:959
```

### 🎯 根本原因

**有 501 个 gRPC 连接处于活跃状态**，每个连接会创建 3 个 goroutine：
1. 一个用于读取请求 (HandleStreams)
2. 一个用于写入响应 (loopyWriter)  
3. 一个用于维护心跳 (keepalive)

**501 个连接 × 3 个 goroutine = 1503 个 goroutine**

### 🚨 可能的问题来源

#### 1. **客户端连接泄漏（最可能）**
- 客户端每次请求都创建新连接，但不关闭
- 没有使用连接池或连接复用
- 连接对象没有正确调用 `conn.Close()`

#### 2. **服务端配置问题**
- 没有设置合理的连接超时时间
- Keepalive 参数配置不当，导致僵尸连接无法清理
- 缺少 `MaxConnectionIdle` 等参数限制连接生命周期

#### 3. **客户端代码问题示例**

```go
// ❌ 错误做法 - 每次请求创建新连接
func makeRequest() {
    conn, _ := grpc.Dial("localhost:50051")
    client := pb.NewServiceClient(conn)
    client.SomeMethod(ctx, req)
    // 忘记关闭连接！
}

// ✅ 正确做法 - 复用连接
var globalConn *grpc.ClientConn

func init() {
    globalConn, _ = grpc.Dial("localhost:50051")
}

func makeRequest() {
    client := pb.NewServiceClient(globalConn)
    client.SomeMethod(ctx, req)
}
```

### 💡 解决方案建议

#### 1. **检查客户端代码**
- 确保 gRPC 连接被复用而不是每次请求都创建
- 使用 `defer conn.Close()` 确保连接在不需要时被关闭
- 如果有多个客户端，考虑使用连接池

#### 2. **配置服务端超时参数**
```go
grpc.NewServer(
    grpc.KeepaliveParams(keepalive.ServerParameters{
        MaxConnectionIdle:     15 * time.Minute,  // 空闲连接超时
        MaxConnectionAge:      30 * time.Minute,  // 连接最大存活时间
        MaxConnectionAgeGrace: 5 * time.Second,   // 强制关闭前的宽限期
        Time:                  5 * time.Minute,   // 心跳间隔
        Timeout:               1 * time.Second,   // 心跳超时
    }),
    grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
        MinTime:             1 * time.Minute,     // 客户端最小心跳间隔
        PermitWithoutStream: true,                // 允许无活跃流时发送心跳
    }),
)
```

#### 3. **监控连接数**
- 添加 metrics 监控活跃连接数
- 设置告警阈值（如超过 100 个连接时告警）
- 定期检查 goroutine profile

#### 4. **排查具体客户端**
可以通过 netstat 或 lsof 查看连接来源：
```bash
netstat -an | grep :50051 | wc -l
lsof -i :50051
```

您需要我帮您进一步查看客户端代码或服务端配置吗？

```plaintext
goroutine profile: total 1508
501 @ 0x54b18ee 0x5474cd7 0x54b0c25 0x5527ba7 0x5528e9a 0x5528e88 0x557ec05 0x558e105 0x56c6557 0x5523430 0x58956e5 0x58956b3 0x5895e25 0x58bf307 0x592b476 0x592ac16 0x54b9e21
#	0x54b0c24	internal/poll.runtime_pollWait+0x84						/Users/chenggang/sdk/go1.23.9/src/runtime/netpoll.go:351
#	0x5527ba6	internal/poll.(*pollDesc).wait+0x26						/Users/chenggang/sdk/go1.23.9/src/internal/poll/fd_poll_runtime.go:84
#	0x5528e99	internal/poll.(*pollDesc).waitRead+0x279					/Users/chenggang/sdk/go1.23.9/src/internal/poll/fd_poll_runtime.go:89
#	0x5528e87	internal/poll.(*FD).Read+0x267							/Users/chenggang/sdk/go1.23.9/src/internal/poll/fd_unix.go:165
#	0x557ec04	net.(*netFD).Read+0x24								/Users/chenggang/sdk/go1.23.9/src/net/fd_posix.go:55
#	0x558e104	net.(*conn).Read+0x44								/Users/chenggang/sdk/go1.23.9/src/net/net.go:189
#	0x56c6556	bufio.(*Reader).Read+0x196							/Users/chenggang/sdk/go1.23.9/src/bufio/bufio.go:241
#	0x552342f	io.ReadAtLeast+0x8f								/Users/chenggang/sdk/go1.23.9/src/io/io.go:335
#	0x58956e4	io.ReadFull+0x64								/Users/chenggang/sdk/go1.23.9/src/io/io.go:354
#	0x58956b2	golang.org/x/net/http2.readFrameHeader+0x32					/Users/chenggang/go/pkg/mod/golang.org/x/net@v0.32.0/http2/frame.go:237
#	0x5895e24	golang.org/x/net/http2.(*Framer).ReadFrame+0x84					/Users/chenggang/go/pkg/mod/golang.org/x/net@v0.32.0/http2/frame.go:501
#	0x58bf306	google.golang.org/grpc/internal/transport.(*http2Server).HandleStreams+0x106	/Users/chenggang/go/pkg/mod/google.golang.org/grpc@v1.70.0/internal/transport/http2_server.go:640
#	0x592b475	google.golang.org/grpc.(*Server).serveStreams+0x395				/Users/chenggang/go/pkg/mod/google.golang.org/grpc@v1.70.0/server.go:1024
#	0x592ac15	google.golang.org/grpc.(*Server).handleRawConn.func1+0x55			/Users/chenggang/go/pkg/mod/google.golang.org/grpc@v1.70.0/server.go:959
```

```go
// ❌ 错误做法 - 每次请求创建新连接
func makeRequest() {
    conn, _ := grpc.Dial("localhost:50051")
    client := pb.NewServiceClient(conn)
    client.SomeMethod(ctx, req)
    // 忘记关闭连接！
}

// ✅ 正确做法 - 复用连接
var globalConn *grpc.ClientConn

func init() {
    globalConn, _ = grpc.Dial("localhost:50051")
}

func makeRequest() {
    client := pb.NewServiceClient(globalConn)
    client.SomeMethod(ctx, req)
}
```

```go
grpc.NewServer(
    grpc.KeepaliveParams(keepalive.ServerParameters{
        MaxConnectionIdle:     15 * time.Minute,  // 空闲连接超时
        MaxConnectionAge:      30 * time.Minute,  // 连接最大存活时间
        MaxConnectionAgeGrace: 5 * time.Second,   // 强制关闭前的宽限期
        Time:                  5 * time.Minute,   // 心跳间隔
        Timeout:               1 * time.Second,   // 心跳超时
    }),
    grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
        MinTime:             1 * time.Minute,     // 客户端最小心跳间隔
        PermitWithoutStream: true,                // 允许无活跃流时发送心跳
    }),
)
```

```shellscript
netstat -an | grep :50051 | wc -l
lsof -i :50051
```

