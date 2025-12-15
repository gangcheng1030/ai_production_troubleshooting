#!/bin/bash

# 测试脚本 - 分别生成 bad_server 和 good_server 的 CPU profile

set -e

echo "========================================================================"
echo "CPU Profile Generation for Bad Server and Good Server"
echo "========================================================================"
echo ""

# 函数：处理单个服务器
process_server() {
    local server_name=$1
    local profile_name=$2
    local server_dir=$3
    
    echo "========================================================================"
    echo "Processing: ${server_name}"
    echo "========================================================================"
    echo ""
    
    # 编译程序
    echo "📦 Compiling ${server_name}..."
    cd ${server_dir}
    go build -o ${server_name} main.go
    if [ $? -eq 0 ]; then
        echo "✅ Compilation successful"
    else
        echo "❌ Compilation failed"
        cd ..
        exit 1
    fi
    echo ""
    
    # 运行程序（后台）
    echo "🚀 Starting ${server_name} in background..."
    ./${server_name} > /tmp/${server_name}_output.log 2>&1 &
    local PID=$!
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
        kill $PID 2>/dev/null || true
        cd ..
        exit 1
    fi
    echo ""
    
    # 捕获CPU profile
    echo "📸 Capturing 30-second CPU profile..."
    if curl -s http://localhost:6060/debug/pprof/profile?seconds=30 -o ../${profile_name}; then
        echo "✅ CPU profile captured: ${profile_name}"
        echo "   File size: $(ls -lh ../${profile_name} | awk '{print $5}')"
    else
        echo "❌ Failed to capture CPU profile"
        kill $PID 2>/dev/null || true
        cd ..
        exit 1
    fi
    echo ""
    
    # 清理
    echo "🧹 Cleaning up ${server_name}..."
    kill $PID 2>/dev/null || true
    rm -f ${server_name}
    echo "✅ Cleanup complete"
    echo ""
    
    cd ..
}

# 主流程
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 处理 bad_server
process_server "bad_server" "bad_cpu.prof" "bad_server"

# 等待端口释放
echo "⏳ Waiting for port to be released (3 seconds)..."
sleep 3
echo ""

# 处理 good_server
process_server "good_server" "good_cpu.prof" "good_server"

echo "========================================================================"
echo "✅ All profiles generated successfully!"
echo "========================================================================"
echo ""
echo "Generated files:"
echo "  - bad_cpu.prof  : CPU profile for bad_server (using + operator)"
echo "  - good_cpu.prof : CPU profile for good_server (using strings.Builder)"
echo ""
echo "Analyze the profiles with:"
echo "  go tool pprof -top bad_cpu.prof"
echo "  go tool pprof -top good_cpu.prof"
echo "  go tool pprof -base bad_cpu.prof good_cpu.prof  # Compare"
echo ""
