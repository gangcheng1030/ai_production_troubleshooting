# 快速开始 (5分钟演示)

## 🚀 一键运行演示

```bash
cd goroutine_analyze
./pprof_examples.sh
```

脚本会自动完成：
1. 启动 gRPC Server
2. 运行 good_client (正确的连接管理)
3. 运行 bad_client (错误的连接管理)
4. 生成对比报告

## 📊 预期输出

### 开始

```
========================================
gRPC 连接泄漏自动化演示
========================================

步骤 1: 启动 gRPC Server
----------------------------------------
启动 server（后台运行）...
Server PID: 12345
✅ Server 启动成功！
初始 goroutine 数量: 2
```

### Good Client (正确做法)

```
步骤 2: 运行 good_client (正确的连接复用)
----------------------------------------
启动 good_client...
等待 2 秒，让客户端发送请求...
当前 goroutine 数量: 8
✅ 已保存 goroutine 详细堆栈到 good_goroutine.txt

📊 Good Client 统计：
   初始 goroutine: 2
   当前 goroutine: 8
   增加数量: 6                    ← ✅ 正常增长
```

### Bad Client (错误做法)

```
步骤 3: 运行 bad_client (错误的连接管理)
----------------------------------------
启动 bad_client...
等待 2 秒，让客户端发送请求...
当前 goroutine 数量: 208
✅ 已保存 goroutine 详细堆栈到 bad_goroutine.txt

📊 Bad Client 统计：
   开始时 goroutine: 4
   当前 goroutine: 208
   增加数量: 204                   ← ❌ 严重泄漏！
```

### 对比结果

```
========================================
结果对比
========================================

📊 Goroutine 数量变化：
   初始状态:         2
   Good Client 期间: 8 (增加 6)      ✅ 稳定
   Good Client 之后: 4
   Bad Client 期间:  208 (增加 204)   ❌ 泄漏
   最终状态:         408 (累计泄漏 406)

📁 生成的文件：
   good_goroutine.txt - Good Client 的 goroutine 堆栈
   bad_goroutine.txt  - Bad Client 的 goroutine 堆栈
   server.log         - Server 日志
   good_client.log    - Good Client 日志
   bad_client.log     - Bad Client 日志

🔍 分析泄漏的 goroutine：
✅ Good Client: 8 个 goroutine
❌ Bad Client:  408 个 goroutine

gRPC 相关的 goroutine：
   Good Client: 4 个
   Bad Client:  400 个
```

## 🔍 分析生成的文件

### 1. 查看泄漏的 goroutine 堆栈

```bash
# 查看 bad_goroutine.txt，你会看到大量重复的 goroutine
less bad_goroutine.txt
```

典型的泄漏 goroutine：
```
goroutine 123 [IO wait]:
google.golang.org/grpc/internal/transport.(*http2Client).reader(...)
    google.golang.org/grpc/internal/transport/http2_client.go:1523

goroutine 124 [select]:
google.golang.org/grpc/internal/transport.(*http2Client).keepalive(...)
    google.golang.org/grpc/internal/transport/http2_client.go:1234

... (重复数百次)
```

### 2. 统计 goroutine 数量

```bash
# Good Client 的 goroutine 数量
grep -c "^goroutine " good_goroutine.txt
# 输出: 8

# Bad Client 的 goroutine 数量
grep -c "^goroutine " bad_goroutine.txt
# 输出: 408
```

### 3. 统计泄漏的 gRPC goroutine

```bash
# Good Client
grep -c "grpc.*transport" good_goroutine.txt
# 输出: 4 (正常，一个连接需要约4个 goroutine)

# Bad Client
grep -c "grpc.*transport" bad_goroutine.txt
# 输出: 400+ (泄漏！每个未关闭的连接都有约4个 goroutine)
```

## 💡 核心要点

### ✅ Good Client (正确做法)

**代码特点**：
```go
// ✅ 创建一次连接
conn, _ := grpc.Dial("localhost:50051", ...)
defer conn.Close()  // ✅ 确保关闭

client := pb.NewHelloServiceClient(conn)

// ✅ 复用连接发送多个请求
for i := 0; i < 500; i++ {
    client.SayHello(ctx, req)
}
```

**结果**：
- 只创建 1 个连接
- goroutine 从 2 增加到 6-8（正常）
- 结束后 goroutine 恢复正常
- ✅ 无泄漏

### ❌ Bad Client (错误做法)

**代码特点**：
```go
func makeRequest() {
    // ❌ 每次都创建新连接
    conn, _ := grpc.Dial("localhost:50051", ...)
    
    // ❌ 没有 defer conn.Close()
    
    client := pb.NewHelloServiceClient(conn)
    client.SayHello(ctx, req)
    
    // 连接泄漏！
}

// 循环调用 500 次
for i := 0; i < 500; i++ {
    makeRequest()  // 每次都泄漏！
}
```

**结果**：
- 创建 500 个连接（2秒内）
- 每个连接约 4 个 goroutine
- goroutine 从 2 增加到 400+
- 结束后 goroutine 不会减少
- ❌ 严重泄漏

## 🎯 关键差异

| 指标 | Good Client | Bad Client |
|------|-------------|-----------|
| 连接数 | 1 | 500+ |
| 初始 goroutine | 2 | 2 |
| 运行时 goroutine | 6-8 | 400+ |
| goroutine 增长 | +4-6 | +400+ |
| 资源泄漏 | ✅ 无 | ❌ 有 |
| 内存使用 | 稳定 | 持续增长 |

## 📚 进一步学习

### 查看详细文档

- [AUTO_DEMO.md](AUTO_DEMO.md) - 自动化脚本详细说明
- [README.md](README.md) - 完整技术文档
- [PPROF_GUIDE.md](PPROF_GUIDE.md) - pprof 使用指南

### 手动运行（学习用）

如果想手动运行各个组件来学习：

```bash
# 终端 1: 启动 server
go run server/main.go

# 终端 2: 运行客户端
go run good_client/main.go  # 正确做法
go run bad_client/main.go   # 错误做法

# 终端 3: 查看 goroutine
curl http://localhost:50052/debug/pprof/goroutine?debug=1
curl http://localhost:50052/debug/pprof/goroutine?debug=2 > goroutine.txt
```

### 使用 Makefile

```bash
make server      # 启动 server
make good-client # 运行 good_client
make bad-client  # 运行 bad_client
```

## 🐛 故障排查

### 问题 1: 端口被占用

```
❌ 错误: 端口 50051 已被占用
```

**解决方法**：
```bash
# 查找占用端口的进程
lsof -i :50051
lsof -i :50052

# 停止进程
kill <PID>
```

### 问题 2: Server 启动超时

```
❌ 错误: Server 启动超时
```

**解决方法**：
```bash
# 查看 server 日志
cat server.log

# 检查是否有编译错误
go build ./server/main.go
```

### 问题 3: go mod 依赖问题

```
❌ 错误: cannot find package
```

**解决方法**：
```bash
go mod tidy
go mod download
```

## ✅ 总结

### 核心教训

1. **gRPC 连接是重量级资源**
   - 每个连接会创建约 4 个 goroutine
   - 用于网络 I/O、心跳检测等

2. **连接必须复用**
   - 应用启动时创建
   - 整个生命周期内复用
   - 程序结束时关闭

3. **不要每次请求都创建连接**
   - 会导致 goroutine 泄漏
   - 会导致内存泄漏
   - 性能差

4. **使用 pprof 诊断**
   - 实时监控 goroutine 数量
   - 分析堆栈找出泄漏点
   - 对比正常和异常情况

### 生产环境建议

```go
// ✅ 推荐做法
var grpcClient pb.HelloServiceClient

func init() {
    // 应用启动时创建连接
    conn, err := grpc.Dial("localhost:50051", ...)
    if err != nil {
        log.Fatal(err)
    }
    grpcClient = pb.NewHelloServiceClient(conn)
}

func CallService() {
    // 复用全局 client
    resp, err := grpcClient.SayHello(ctx, req)
    // ...
}
```

**记住：gRPC 连接要像数据库连接一样复用！**

---

现在你可以运行 `./pprof_examples.sh` 开始演示了！🚀

