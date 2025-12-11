# gRPC 连接泄漏问题演示

## 问题描述

gRPC client 每次请求都创建新连接（new dial），没有复用连接，也没有释放连接，导致 goroutine 数量持续上涨。

## 项目结构

```
goroutine_analyze/
├── proto/              # gRPC 服务定义
│   └── hello.proto
├── server/             # gRPC 服务端
│   └── main.go
├── bad_client/         # ❌ 有问题的客户端（演示问题）
│   └── main.go
├── good_client/        # ✅ 正确的客户端（正确实践）
│   └── main.go
└── README.md
```

## 使用步骤

### 1. 生成 gRPC 代码

```bash
cd goroutine_analyze

# 安装依赖
go mod tidy

# 生成 proto 文件（已生成，无需重复执行）
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/hello.proto
```

### 2. 启动服务端

```bash
go run server/main.go
```

服务端会监听两个端口：
- `:50051` - gRPC 服务端口
- `:50052` - pprof HTTP 服务端口（用于性能分析）

输出示例：
```
Server starting on :50051...
pprof server starting on :50052
访问 http://localhost:50052/debug/pprof 查看 pprof 信息
查看 goroutine: http://localhost:50052/debug/pprof/goroutine?debug=2
```

### 3. 运行有问题的客户端（演示问题）

在新终端中运行：

```bash
go run bad_client/main.go
```

**观察现象：**
- 客户端会发送 **500 个请求**
- goroutine 数量持续上涨（从 2 增长到 500+）
- 每次请求都创建新连接
- 连接没有关闭，资源泄漏
- 程序结束前会等待 10 秒，以便使用 pprof 查看详细信息

**预期结果：**
```
初始 goroutine 数量: 2
⚠️  已发送 50 个请求，goroutine: 52
⚠️  已发送 100 个请求，goroutine: 102
...
⚠️  已发送 500 个请求，goroutine: 502
最终 goroutine 数量: 502
泄漏的 goroutine: 500
```

### 4. 运行正确的客户端（对比）

在新终端中运行：

```bash
go run good_client/main.go
```

**观察现象：**
- 客户端同样发送 **500 个请求**
- goroutine 数量保持稳定（仅增加 4-6 个，这是正常的）
- 连接被复用
- 资源正确释放

**预期结果：**
```
初始 goroutine 数量: 2
✅ 已发送 50 个请求，goroutine: 6
✅ 已发送 100 个请求，goroutine: 6
...
✅ 已发送 500 个请求，goroutine: 6
最终 goroutine 数量: 6
goroutine 变化: +4
```

## 问题原因

### ❌ 错误代码示例

```go
func makeRequestBad() error {
    // 问题1：每次请求都创建新连接
    conn, err := grpc.Dial("localhost:50051", 
        grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        return err
    }
    
    // 问题2：没有 defer conn.Close()
    
    client := pb.NewHelloServiceClient(conn)
    resp, err := client.SayHello(ctx, req)
    
    return err
}
```

**问题分析：**
1. 每次调用都执行 `grpc.Dial()`，创建新的连接
2. 没有调用 `conn.Close()`，连接无法释放
3. gRPC 连接底层会创建多个 goroutine 来处理网络 I/O
4. 随着请求增加，未关闭的连接和 goroutine 不断累积
5. 最终导致内存泄漏和 goroutine 数量爆炸

## 解决方案

### ✅ 正确代码示例

```go
type GoodClient struct {
    conn   *grpc.ClientConn
    client pb.HelloServiceClient
}

// 在应用启动时创建一次连接
func NewGoodClient(address string) (*GoodClient, error) {
    conn, err := grpc.Dial(address,
        grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        return nil, err
    }
    
    return &GoodClient{
        conn:   conn,
        client: pb.NewHelloServiceClient(conn),
    }, nil
}

// 提供关闭方法
func (c *GoodClient) Close() error {
    return c.conn.Close()
}

// 复用同一个连接
func (c *GoodClient) MakeRequest() error {
    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    
    resp, err := c.client.SayHello(ctx, &pb.HelloRequest{Name: "World"})
    return err
}

// 使用方式
func main() {
    client, err := NewGoodClient("localhost:50051")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close() // 确保关闭
    
    // 复用连接发送多个请求
    for i := 0; i < 100; i++ {
        client.MakeRequest()
    }
}
```

**最佳实践：**
1. **连接复用**：在应用启动时创建连接，在应用生命周期内复用
2. **正确关闭**：使用 `defer conn.Close()` 确保资源释放
3. **连接池**：对于高并发场景，可以实现连接池
4. **监控告警**：监控 goroutine 数量，及时发现泄漏

## 性能对比

| 指标 | 错误做法 (bad_client) | 正确做法 (good_client) |
|------|---------------------|---------------------|
| 请求数量 | 500 | 500 |
| 初始 goroutine | 2 | 2 |
| 最终 goroutine | 502 | 6 |
| goroutine 增长 | +500（泄漏！） | +4（正常） |
| 连接数 | 500（每次新建） | 1（复用） |
| 内存占用 | 持续增长 | 稳定 |
| 请求延迟 | 高（每次建立连接开销） | 低（复用连接） |
| 资源泄漏 | ✗ 严重泄漏 | ✓ 无泄漏 |

## 使用 pprof 分析 goroutine 泄漏

服务端已经集成了 pprof，可以通过 HTTP 接口查看详细的 goroutine 信息。

### 查看 pprof 概览

在浏览器中打开：
```
http://localhost:50052/debug/pprof
```

### 查看 goroutine 列表

```bash
# 查看简要信息
curl http://localhost:50052/debug/pprof/goroutine?debug=1

# 查看详细堆栈
curl http://localhost:50052/debug/pprof/goroutine?debug=2

# 保存到文件
curl http://localhost:50052/debug/pprof/goroutine?debug=2 > goroutine.txt
```

### 使用 go tool pprof 交互式分析

```bash
# 进入交互式分析
go tool pprof http://localhost:50052/debug/pprof/goroutine

# 常用命令
(pprof) top       # 显示占用最多的函数
(pprof) traces    # 显示所有堆栈
(pprof) list <function>  # 查看具体函数
```

### 生成可视化图表

```bash
# 需要先安装 graphviz
# macOS: brew install graphviz
# Ubuntu: sudo apt-get install graphviz

# 生成 PDF
go tool pprof -pdf http://localhost:50052/debug/pprof/goroutine > goroutine.pdf

# 生成 PNG
go tool pprof -png http://localhost:50052/debug/pprof/goroutine > goroutine.png
```

### 实时监控 goroutine 数量

```bash
# 使用 watch 命令实时查看
watch -n 1 'curl -s http://localhost:50052/debug/pprof/goroutine?debug=1 | head -1'
```

输出示例：
```
goroutine profile: total 502  # bad_client 运行时
goroutine profile: total 6    # good_client 运行时
```

### 详细分析泄漏的 goroutine

运行 bad_client 后，保存并分析 goroutine 堆栈：

```bash
curl http://localhost:50052/debug/pprof/goroutine?debug=2 > bad_goroutine.txt
less bad_goroutine.txt
```

你会看到大量类似的泄漏 goroutine：
```
goroutine 123 [IO wait]:
google.golang.org/grpc/internal/transport.(*http2Client).reader(...)

goroutine 124 [select]:
google.golang.org/grpc/internal/transport.(*http2Client).keepalive(...)
```

这些都是 gRPC 连接相关的 goroutine，证明连接没有被正确关闭。

📖 **详细的 pprof 使用指南请查看 [PPROF_GUIDE.md](PPROF_GUIDE.md)**

## 监控建议

在生产环境中，建议监控以下指标：

1. **goroutine 数量**：`runtime.NumGoroutine()`
2. **连接数**：通过 netstat 或 ss 命令
3. **内存使用**：堆内存分配情况
4. **请求延迟**：gRPC 请求响应时间

```go
// 监控示例
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        goroutines := runtime.NumGoroutine()
        if goroutines > threshold {
            log.Printf("⚠️ Goroutine count high: %d", goroutines)
            // 发送告警
        }
    }
}()
```

## 快速运行

使用 Makefile 快速运行：

```bash
# 启动服务端
make server

# 运行有问题的客户端
make bad-client

# 运行正确的客户端
make good-client
```

或使用一键运行脚本（完整演示）：

```bash
./run_demo.sh
# 选择选项 4 进行完整演示
```

## 参考资料

- [PPROF_GUIDE.md](PPROF_GUIDE.md) - pprof 详细使用指南
- [gRPC Go Best Practices](https://github.com/grpc/grpc-go/blob/master/Documentation/concurrency.md)
- [Go gRPC Client Connection Pooling](https://github.com/grpc/grpc-go/tree/master/examples/features/load_balancing)
- [Go pprof Documentation](https://pkg.go.dev/net/http/pprof)

