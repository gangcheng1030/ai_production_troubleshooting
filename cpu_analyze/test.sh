#!/bin/bash

# 测试脚本 - 验证程序可以正常编译和运行

set -e

echo "================================"
echo "Testing CPU Analysis Demo"
echo "================================"
echo ""

# 检查Go是否安装
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed"
    exit 1
fi

echo "✅ Go version: $(go version)"
echo ""

# 编译程序
echo "📦 Compiling case3_string_concat.go..."
go build -o case3_string_concat case3_string_concat.go
if [ $? -eq 0 ]; then
    echo "✅ Compilation successful"
else
    echo "❌ Compilation failed"
    exit 1
fi
echo ""

# 运行程序（后台）
echo "🚀 Starting program in background..."
./case3_string_concat > /tmp/cpu_demo_output.log 2>&1 &
PID=$!
echo "✅ Program started with PID: $PID"
echo ""

# 等待程序启动
echo "⏳ Waiting for program to start (5 seconds)..."
sleep 5

# 检查pprof是否可访问
echo "🔍 Checking pprof endpoint..."
if curl -s http://localhost:6060/debug/pprof/ > /dev/null; then
    echo "✅ pprof endpoint is accessible"
else
    echo "❌ pprof endpoint is not accessible"
    kill $PID
    exit 1
fi
echo ""

# 显示可用的pprof端点
echo "📊 Available pprof endpoints:"
echo "   - http://localhost:6060/debug/pprof/"
echo "   - http://localhost:6060/debug/pprof/profile?seconds=10"
echo "   - http://localhost:6060/debug/pprof/heap"
echo ""

# 捕获一个短时间的CPU profile
echo "📸 Capturing 10-second CPU profile..."
if curl -s http://localhost:6060/debug/pprof/profile?seconds=10 -o test_cpu.prof; then
    echo "✅ CPU profile captured: test_cpu.prof"
    echo "   File size: $(ls -lh test_cpu.prof | awk '{print $5}')"
else
    echo "❌ Failed to capture CPU profile"
    kill $PID
    exit 1
fi
echo ""

# 分析profile
echo "🔬 Analyzing CPU profile..."
echo "Top functions:"
go tool pprof -top test_cpu.prof | head -n 15
echo ""

# 清理
echo "🧹 Cleaning up..."
kill $PID
rm -f case3_string_concat test_cpu.prof
echo "✅ Cleanup complete"
echo ""

# 显示程序输出的前50行
echo "📋 Program output (first 50 lines):"
echo "================================"
head -n 50 /tmp/cpu_demo_output.log
echo "================================"
echo ""

echo "✅ All tests passed!"
echo ""
echo "To run the full demo:"
echo "  1. go run case3_string_concat.go"
echo "  2. ./analyze_cpu.sh"
echo ""

