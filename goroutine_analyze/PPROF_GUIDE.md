# pprof 使用指南

本文档介绍如何使用 pprof 工具分析 goroutine 泄漏问题。

## pprof 服务配置

gRPC Server 已经配置了 pprof HTTP 服务，监听在 **50052** 端口。

启动服务端后，你可以通过以下方式访问 pprof 信息：

```bash
# 启动服务端
go run server/main.go
```

服务端输出：
```
Server starting on :50051...
pprof server starting on :50052
访问 http://localhost:50052/debug/pprof 查看 pprof 信息
查看 goroutine: http://localhost:50052/debug/pprof/goroutine?debug=2
```

## 🔍 查看 pprof 信息的方法

### 方法 1: 浏览器查看概览

在浏览器中打开：

```
http://localhost:50052/debug/pprof
```

你会看到 pprof 首页，包括：
- goroutine: 当前所有 goroutine 的数量和堆栈
- heap: 堆内存分配情况
- threadcreate: 线程创建情况
- block: 阻塞分析
- mutex: 互斥锁争用分析

### 方法 2: 命令行查看 goroutine 列表

查看简单的 goroutine 统计：

```bash
curl http://localhost:50052/debug/pprof/goroutine?debug=1
```

输出示例：
```
goroutine profile: total 502
500 @ 0x1038f40 0x1001234 ...
    google.golang.org/grpc/internal/transport.(*http2Client).reader
    ...

2 @ 0x1038f40 0x1002345 ...
    main.main.func1
    ...
```

### 方法 3: 查看详细的 goroutine 堆栈

查看所有 goroutine 的完整堆栈信息：

```bash
curl http://localhost:50052/debug/pprof/goroutine?debug=2
```

或者保存到文件：

```bash
curl http://localhost:50052/debug/pprof/goroutine?debug=2 > goroutine_stack.txt
```

这会输出每个 goroutine 的详细堆栈，例如：

```
goroutine 123 [IO wait]:
internal/poll.(*FD).Read(0xc000104000, {0xc00014e000, 0x1000, 0x1000})
    /usr/local/go/src/internal/poll/fd_unix.go:167 +0x2f
net.(*netFD).Read(0xc000104000, {0xc00014e000?, 0x0?, 0x0?})
    /usr/local/go/src/net/fd_posix.go:55 +0x2a
...
google.golang.org/grpc/internal/transport.(*http2Client).reader(0xc000206000)
    google.golang.org/grpc/internal/transport/http2_client.go:1523 +0x5b2
created by google.golang.org/grpc/internal/transport.newHTTP2Client
    google.golang.org/grpc/internal/transport/http2_client.go:326 +0x11c5
```

### 方法 4: 使用 go tool pprof 交互式分析

下载 goroutine profile 并交互式分析：

```bash
go tool pprof http://localhost:50052/debug/pprof/goroutine
```

进入交互式界面后，可以使用以下命令：

```
(pprof) top
(pprof) top10
(pprof) list <function_name>
(pprof) web        # 生成可视化图表（需要安装 graphviz）
(pprof) traces     # 查看所有堆栈
```

### 方法 5: 生成可视化图表

生成 goroutine 的可视化图：

```bash
# 安装 graphviz（如果还没有）
# macOS: brew install graphviz
# Ubuntu: sudo apt-get install graphviz

# 生成 PDF 图表
go tool pprof -pdf http://localhost:50052/debug/pprof/goroutine > goroutine.pdf

# 生成 PNG 图表
go tool pprof -png http://localhost:50052/debug/pprof/goroutine > goroutine.png

# 生成 SVG 图表
go tool pprof -svg http://localhost:50052/debug/pprof/goroutine > goroutine.svg
```

## 📊 实战演示

### 场景 1: 分析 bad_client 的 goroutine 泄漏

#### 步骤 1: 启动服务端

```bash
# 终端 1
cd goroutine_analyze
go run server/main.go
```

#### 步骤 2: 运行有问题的客户端

```bash
# 终端 2
go run bad_client/main.go
```

客户端会发送 500 个请求，然后等待 10 秒以便你查看 pprof 信息。

#### 步骤 3: 在客户端运行期间查看 goroutine

```bash
# 终端 3 - 查看 goroutine 数量
watch -n 1 'curl -s http://localhost:50052/debug/pprof/goroutine?debug=1 | head -1'
```

你会看到 goroutine 数量持续增长：
```
goroutine profile: total 52
goroutine profile: total 102
goroutine profile: total 152
...
goroutine profile: total 502
```

#### 步骤 4: 查看详细的泄漏 goroutine 堆栈

```bash
# 终端 3 - 保存完整堆栈
curl http://localhost:50052/debug/pprof/goroutine?debug=2 > bad_client_goroutine.txt

# 查看泄漏的 goroutine
less bad_client_goroutine.txt
```

你会看到大量类似的 goroutine：

```
goroutine 123 [IO wait]:
...
google.golang.org/grpc/internal/transport.(*http2Client).reader(...)
    google.golang.org/grpc/internal/transport/http2_client.go:1523

goroutine 124 [select]:
google.golang.org/grpc/internal/transport.(*http2Client).keepalive(...)
    google.golang.org/grpc/internal/transport/http2_client.go:1234

goroutine 125 [chan receive]:
google.golang.org/grpc/internal/transport.(*controlBuffer).get(...)
    google.golang.org/grpc/internal/transport/controlbuf.go:398
```

**分析**：这些都是 gRPC 连接相关的 goroutine，说明连接没有被正确关闭。

#### 步骤 5: 使用 go tool pprof 分析

```bash
go tool pprof http://localhost:50052/debug/pprof/goroutine
```

进入交互式界面后：

```
(pprof) top
Showing nodes accounting for 500, 99.01% of 505 total
      flat  flat%   sum%        cum   cum%
       500 99.01% 99.01%        500 99.01%  google.golang.org/grpc/internal/transport.(*http2Client).reader

(pprof) list http2Client
```

### 场景 2: 对比 good_client 的正常情况

#### 步骤 1: 运行正确的客户端

```bash
# 终端 2（停止 bad_client 后）
go run good_client/main.go
```

#### 步骤 2: 查看 goroutine 数量

```bash
# 终端 3
curl -s http://localhost:50052/debug/pprof/goroutine?debug=1 | head -1
```

输出：
```
goroutine profile: total 8
```

即使发送 500 个请求，goroutine 数量也保持稳定在 8 左右。

#### 步骤 3: 对比堆栈信息

```bash
curl http://localhost:50052/debug/pprof/goroutine?debug=2 > good_client_goroutine.txt
```

对比两个文件：
```bash
wc -l bad_client_goroutine.txt good_client_goroutine.txt
```

输出：
```
  5000 bad_client_goroutine.txt    # 大量泄漏的 goroutine
    80 good_client_goroutine.txt    # 正常数量
```

## 🎯 如何识别泄漏的 goroutine

### 特征 1: 大量相同堆栈的 goroutine

如果看到大量相同或相似的 goroutine 堆栈，可能存在泄漏：

```bash
curl -s http://localhost:50052/debug/pprof/goroutine?debug=1
```

输出：
```
goroutine profile: total 502
500 @ 0x1038f40 0x1001234 ...    ← 500 个相同的堆栈！
    google.golang.org/grpc/internal/transport.(*http2Client).reader
```

### 特征 2: 持续增长的 goroutine 数量

使用 watch 命令持续观察：

```bash
watch -n 1 'curl -s http://localhost:50052/debug/pprof/goroutine?debug=1 | head -1'
```

如果数量持续增长，很可能存在泄漏。

### 特征 3: 与业务操作相关的增长

例如，每次 API 调用都增加 4 个 goroutine，说明连接没有被复用。

## 📈 监控和告警

### 实时监控 goroutine 数量

创建一个简单的监控脚本：

```bash
#!/bin/bash
# monitor_goroutine.sh

THRESHOLD=100
while true; do
    COUNT=$(curl -s http://localhost:50052/debug/pprof/goroutine?debug=1 | head -1 | grep -oE '[0-9]+' | head -1)
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Goroutines: $COUNT"
    
    if [ "$COUNT" -gt "$THRESHOLD" ]; then
        echo "⚠️  WARNING: Goroutine count exceeds threshold ($THRESHOLD)"
        # 这里可以发送告警
    fi
    
    sleep 5
done
```

使用：
```bash
chmod +x monitor_goroutine.sh
./monitor_goroutine.sh
```

### 定期保存 goroutine profile

```bash
#!/bin/bash
# save_goroutine_profile.sh

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
curl http://localhost:50052/debug/pprof/goroutine?debug=2 > "goroutine_${TIMESTAMP}.txt"
echo "Saved goroutine profile to goroutine_${TIMESTAMP}.txt"
```

### 对比不同时间点的 goroutine

```bash
# 保存基线
curl http://localhost:50052/debug/pprof/goroutine > baseline.prof

# 运行一段时间后保存
curl http://localhost:50052/debug/pprof/goroutine > current.prof

# 对比差异
go tool pprof -base baseline.prof current.prof
```

## 🔧 常用 pprof 命令速查

| 命令 | 说明 |
|------|------|
| `curl http://localhost:50052/debug/pprof` | 查看 pprof 首页 |
| `curl http://localhost:50052/debug/pprof/goroutine?debug=1` | 查看 goroutine 统计 |
| `curl http://localhost:50052/debug/pprof/goroutine?debug=2` | 查看完整堆栈 |
| `go tool pprof http://localhost:50052/debug/pprof/goroutine` | 交互式分析 |
| `go tool pprof -top http://localhost:50052/debug/pprof/goroutine` | 直接显示 top |
| `go tool pprof -pdf http://localhost:50052/debug/pprof/goroutine > out.pdf` | 生成 PDF |
| `go tool pprof -png http://localhost:50052/debug/pprof/goroutine > out.png` | 生成 PNG |
| `go tool pprof -base old.prof new.prof` | 对比差异 |

## 📝 最佳实践

### 1. 在开发环境启用 pprof

```go
import _ "net/http/pprof"

func main() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
    
    // 应用逻辑
}
```

### 2. 生产环境的 pprof 安全

生产环境应该限制 pprof 访问：

```go
import (
    "net/http"
    "net/http/pprof"
)

func main() {
    // 创建一个独立的 ServeMux
    mux := http.NewServeMux()
    mux.HandleFunc("/debug/pprof/", pprof.Index)
    mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
    mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
    mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
    mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
    
    // 只监听内网地址
    go http.ListenAndServe("127.0.0.1:6060", mux)
    
    // 或者添加认证中间件
}
```

### 3. 定期采样和分析

建议在生产环境中：
- 定期（如每小时）自动采样 goroutine profile
- 对比基线，检测异常增长
- 设置告警阈值
- 保留历史 profile 以便分析趋势

## 🎓 延伸阅读

- [pprof 官方文档](https://pkg.go.dev/net/http/pprof)
- [Go Blog - Profiling Go Programs](https://go.dev/blog/pprof)
- [Debugging performance issues in Go programs](https://go.dev/blog/profiling-go-programs)
- [Understanding pprof](https://jvns.ca/blog/2017/09/24/profiling-go-with-pprof/)

## 总结

pprof 是 Go 程序性能分析的利器，特别适合：
- ✅ 诊断 goroutine 泄漏
- ✅ 分析内存泄漏
- ✅ 识别性能瓶颈
- ✅ 定位阻塞问题

在本 demo 中，通过 pprof 你可以清楚地看到：
- **bad_client**: 500+ goroutine，大量重复的 gRPC 连接堆栈
- **good_client**: 8 goroutine，正常的连接管理

记住：**定期使用 pprof 监控你的 Go 应用，及早发现问题！**

