# 使用指南 - String Concatenation CPU Analysis

## 项目结构

```
cpu_analyze/
├── README.md                    # 项目介绍和详细文档
├── QUICKSTART.md               # 快速开始指南
├── USAGE.md                    # 本文件 - 使用说明
├── case3_string_concat.go      # 主程序（演示代码）
├── analyze_cpu.sh              # CPU分析脚本（交互式）
├── test.sh                     # 测试脚本（验证程序可运行）
├── go.mod                      # Go模块文件
└── profiles/                   # 存储捕获的profile文件（运行后自动创建）
```

## 完整演示流程

### 步骤1：启动演示程序

打开第一个终端窗口：

```bash
cd /Users/chenggang/go/src/github.com/gangcheng1030/ai_production_troubleshooting/cpu_analyze

# 运行程序
go run case3_string_concat.go
```

**预期输出：**

```
Starting pprof server on http://localhost:6060
Access the following endpoints:
  - http://localhost:6060/debug/pprof/
  - http://localhost:6060/debug/pprof/profile?seconds=30
  - http://localhost:6060/debug/pprof/heap

=== Case 3: String Concatenation ===

Benchmarking with n=1000:
Bad: + operator           : Time=  50.2ms, Length=11893
Good: strings.Builder     : Time= 500.8µs, Length=11893
Good: bytes.Buffer        : Time= 520.3µs, Length=11893
...

======================================================================
Starting continuous workload for CPU profiling...
Press Ctrl+C to stop
======================================================================

Goroutine 1: Running bad string concatenation (+ operator)...
Goroutine 2: Running good string concatenation (strings.Builder)...
Goroutine 3: Running good string concatenation with more data...

Tip: Use the following commands to capture and analyze CPU profile:
...
```

程序现在持续运行，3个goroutine不断执行字符串拼接操作。

### 步骤2：分析CPU使用情况

打开第二个终端窗口：

```bash
cd /Users/chenggang/go/src/github.com/gangcheng1030/ai_production_troubleshooting/cpu_analyze

# 运行分析脚本
./analyze_cpu.sh
```

**脚本菜单：**

```
========================================
CPU Profile Analysis Menu
========================================

1) Capture and analyze CPU profile (quick)
2) Capture CPU profile and open web UI
3) Capture profile with custom duration
4) Compare latest two profiles
5) View goroutine status
6) List all captured profiles
7) Open existing profile in web UI
8) Full analysis (all steps)
0) Exit

Choose an option:
```

#### 推荐选项：选择 2（快速查看火焰图）

1. 输入 `2` 并回车
2. 等待30秒（采集CPU profile）
3. 自动打开浏览器访问 http://localhost:8080
4. 查看火焰图

#### 或选择：选择 1（快速文本分析）

1. 输入 `1` 并回车
2. 等待30秒
3. 查看终端输出的分析结果

## 方法对比

### 方法一：使用分析脚本（推荐 - 最简单）

```bash
# 快速模式
./analyze_cpu.sh quick

# 或交互式菜单
./analyze_cpu.sh

# 或直接打开Web UI
./analyze_cpu.sh web
```

**优点：**
- ✅ 自动化所有步骤
- ✅ 生成详细报告
- ✅ 保存profile到文件
- ✅ 可管理多个profile

### 方法二：手动命令（学习pprof命令）

```bash
# 1. 捕获CPU profile
curl http://localhost:6060/debug/pprof/profile?seconds=30 -o cpu.prof

# 2. 文本分析
go tool pprof -top cpu.prof

# 3. 查看特定函数
go tool pprof -list badStringConcat cpu.prof

# 4. 打开Web UI
go tool pprof -http=:8080 cpu.prof
```

**优点：**
- ✅ 学习pprof命令
- ✅ 灵活控制每一步
- ✅ 适合集成到自动化脚本

### 方法三：浏览器直接访问（快速查看）

```bash
# 1. 在浏览器中打开
http://localhost:6060/debug/pprof/

# 2. 点击 "profile" 链接（会下载30秒的profile）

# 3. 分析下载的文件
go tool pprof ~/Downloads/profile
```

**优点：**
- ✅ 无需命令行
- ✅ 快速查看

## 分析结果解读

### Top函数输出示例

```
Showing nodes accounting for 8.50s, 85.00% of 10.00s total
      flat  flat%   sum%        cum   cum%
     3.20s 32.00% 32.00%      3.20s 32.00%  main.badStringConcat
     2.15s 21.50% 53.50%      2.15s 21.50%  runtime.concatstrings
     1.85s 18.50% 72.00%      1.85s 18.50%  runtime.mallocgc
     0.80s  8.00% 80.00%      0.80s  8.00%  main.goodStringBuilder
     0.50s  5.00% 85.00%      0.50s  5.00%  runtime.memmove
```

**关键指标解读：**

- **flat**: 函数自身消耗的CPU时间
  - `badStringConcat`: 3.20秒 (32%)
  - `goodStringBuilder`: 0.80秒 (8%)
  - **差异：4倍！**

- **cum**: 函数及其调用的所有函数的总CPU时间

**发现：**
1. ✅ `badStringConcat`消耗32%的CPU
2. ✅ 大量时间在`runtime.concatstrings`（字符串拼接）
3. ✅ 大量时间在`runtime.mallocgc`（内存分配）
4. ✅ `goodStringBuilder`只用8%的CPU

### 火焰图解读

在Web UI中查看火焰图（http://localhost:8080/ui/flamegraph）：

**如何读火焰图：**
- **横轴（宽度）**：CPU时间占比（越宽越耗时）
- **纵轴（高度）**：调用栈深度
- **颜色**：随机分配，无特殊含义
- **可交互**：点击放大，搜索函数名

**你应该看到：**
1. `badStringConcat`的火焰很宽（占用CPU多）
   - 下方有大量`runtime.concatstrings`
   - 还有大量`runtime.mallocgc`

2. `goodStringBuilder`的火焰很窄（占用CPU少）
   - 更少的runtime调用

## 实战练习

### 练习1：观察性能差异

1. ✅ 启动程序
2. ✅ 捕获30秒CPU profile
3. ✅ 在火焰图中找到`badStringConcat`和`goodStringBuilder`
4. ✅ 比较它们的宽度
5. ✅ 记录flat%的差异

**问题：**
- `badStringConcat`的flat%是多少？
- `goodStringBuilder`的flat%是多少？
- 性能差异是几倍？

### 练习2：修改参数观察变化

修改`case3_string_concat.go`中的参数：

```go
// 找到这些行（在main函数中）
go func() {
    fmt.Println("Goroutine 1: Running bad string concatenation (+ operator)...")
    continuousBadConcat(500) // 改成 1000 试试
}()

go func() {
    fmt.Println("Goroutine 2: Running good string concatenation (strings.Builder)...")
    continuousGoodConcat(5000) // 改成 10000 试试
}()
```

**步骤：**
1. 修改参数
2. 重启程序
3. 重新捕获profile
4. 对比前后差异

**问题：**
- 增加循环次数后，CPU占比如何变化？
- `badStringConcat`的执行时间增加了多少？

### 练习3：对比优化效果

```bash
# 1. 捕获优化前的profile
curl http://localhost:6060/debug/pprof/profile?seconds=30 -o before.prof

# 2. 修改代码（注释掉badStringConcat的goroutine）

# 3. 重启程序

# 4. 捕获优化后的profile
curl http://localhost:6060/debug/pprof/profile?seconds=30 -o after.prof

# 5. 对比
go tool pprof -base=before.prof -top after.prof
```

**问题：**
- 总CPU使用率降低了多少？
- 哪些函数的CPU时间减少了？

## 常见场景

### 场景1：快速诊断CPU问题

```bash
# 1. 程序运行中
go run case3_string_concat.go &

# 2. 快速捕获和分析
./analyze_cpu.sh quick

# 3. 查看报告
cat profiles/cpu_*_report.txt
```

### 场景2：生成演示报告

```bash
# 1. 捕获profile
curl http://localhost:6060/debug/pprof/profile?seconds=30 -o demo.prof

# 2. 生成PDF报告
go tool pprof -pdf demo.prof > cpu_report.pdf

# 3. 生成SVG图
go tool pprof -svg demo.prof > cpu_graph.svg

# 4. 生成文本报告
go tool pprof -top demo.prof > cpu_top.txt
go tool pprof -list badStringConcat demo.prof > bad_detail.txt
go tool pprof -list goodStringBuilder demo.prof > good_detail.txt
```

### 场景3：持续监控

```bash
# 每60秒捕获一次profile
while true; do
    timestamp=$(date +%Y%m%d_%H%M%S)
    curl http://localhost:6060/debug/pprof/profile?seconds=30 -o "profiles/cpu_${timestamp}.prof"
    echo "Captured profile at ${timestamp}"
    sleep 60
done
```

## pprof交互式命令

在`go tool pprof cpu.prof`交互式界面中：

```bash
# 查看top函数
(pprof) top
(pprof) top 20           # 显示top 20
(pprof) top -cum         # 按累积时间排序

# 查看函数详情
(pprof) list main.badStringConcat
(pprof) list goodStringBuilder

# 查看调用关系
(pprof) peek badStringConcat

# 生成图形
(pprof) web              # 需要graphviz
(pprof) svg > output.svg
(pprof) pdf > output.pdf

# 查看采样的调用栈
(pprof) traces

# 按正则查找
(pprof) top badString

# 帮助
(pprof) help
```

## 验证程序运行

运行测试脚本验证一切正常：

```bash
./test.sh
```

这个脚本会：
1. ✅ 编译程序
2. ✅ 启动程序
3. ✅ 检查pprof端点
4. ✅ 捕获CPU profile
5. ✅ 分析profile
6. ✅ 清理测试文件

## 故障排查

### 问题1：端口6060被占用

**错误信息：**
```
bind: address already in use
```

**解决方法：**
```bash
# 查找占用端口的进程
lsof -i :6060

# 杀死进程
kill -9 <PID>

# 或修改程序使用其他端口
# 编辑 case3_string_concat.go，将 6060 改为 6061
```

### 问题2：无法访问pprof

**检查清单：**
1. ✅ 程序是否正在运行？
2. ✅ 防火墙是否阻止了访问？
3. ✅ 端口是否正确？

**测试：**
```bash
curl http://localhost:6060/debug/pprof/
```

### 问题3：火焰图无法显示

**原因：**缺少graphviz

**解决：**
```bash
# macOS
brew install graphviz

# Ubuntu/Debian
sudo apt-get install graphviz

# Windows
# 下载：https://graphviz.org/download/
```

### 问题4：CPU profile为空

**原因：**采样时间太短或程序CPU使用率太低

**解决：**
```bash
# 增加采样时间
curl http://localhost:6060/debug/pprof/profile?seconds=60 -o cpu.prof

# 或增加程序负载（修改循环次数）
```

## 输出文件说明

运行分析后，会生成以下文件：

```
profiles/
├── cpu_20231215_143022.prof      # CPU profile二进制文件
├── cpu_20231215_143022_report.txt # 文本分析报告
├── cpu_20231215_143530.prof
└── cpu_20231215_143530_report.txt
```

**profile文件**：二进制格式，包含采样数据
**report文件**：文本格式，包含top函数列表和详细分析

## 下一步

1. ✅ 阅读 `README.md` 了解原理
2. ✅ 阅读 `QUICKSTART.md` 快速上手
3. ✅ 运行demo并分析结果
4. ✅ 尝试修改参数观察变化
5. ✅ 在你的项目中应用这些技巧

## 相关资源

- [Go官方诊断文档](https://go.dev/doc/diagnostics)
- [pprof用户指南](https://github.com/google/pprof/blob/master/doc/README.md)
- [火焰图介绍](https://www.brendangregg.com/flamegraphs.html)
- 相关案例：
  - `../goroutine_analyze/` - Goroutine泄漏分析
  - `../memory_analyze/` - 内存问题分析

---

有问题？查看 README.md 或 QUICKSTART.md 获取更多信息。

