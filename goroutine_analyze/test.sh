#!/bin/bash

# 自动化演示脚本：演示 gRPC 连接泄漏问题
# 功能：
# 1. 启动 server
# 2. 启动 good_client，等待2秒，保存 goroutine 信息
# 3. 启动 bad_client，等待2秒，保存 goroutine 信息

set -e  # 遇到错误立即退出

echo "========================================"
echo "gRPC 连接泄漏自动化演示"
echo "========================================"
echo ""

# 清理函数
cleanup() {
    echo ""
    echo "=== 清理资源 ==="
    
    # 停止 server（杀死整个进程组）
    if [ ! -z "$SERVER_PID" ] && kill -0 $SERVER_PID 2>/dev/null; then
        echo "停止 server (PID: $SERVER_PID)..."
        # 杀死进程组（包括 go run 和实际的 server 进程）
        kill -- -$SERVER_PID 2>/dev/null || true
        sleep 1
        # 如果还存在，强制杀死
        kill -9 -- -$SERVER_PID 2>/dev/null || true
        # 不使用 wait，因为进程组 kill 可能导致 wait 卡住
    fi
    
    # 停止 client
    if [ ! -z "$CLIENT_PID" ] && kill -0 $CLIENT_PID 2>/dev/null; then
        echo "停止 client (PID: $CLIENT_PID)..."
        kill -- -$CLIENT_PID 2>/dev/null || true
        sleep 0.5
        kill -9 -- -$CLIENT_PID 2>/dev/null || true
        # 不使用 wait，避免卡住
    fi
    
    # 通过端口号查找并杀死可能遗留的进程（兜底清理）
    echo "检查并清理端口占用..."
    sleep 0.5
    local pids=$(lsof -ti :50051,50052 2>/dev/null || true)
    if [ ! -z "$pids" ]; then
        echo "发现遗留进程，正在清理: $pids"
        echo "$pids" | xargs kill -9 2>/dev/null || true
        sleep 0.5
    fi
    
    echo "✅ 清理完成"
}

# 设置退出时自动清理
trap cleanup EXIT INT TERM

# ============================================
# 步骤 1: 启动 server
# ============================================
echo "步骤 1: 启动 gRPC Server"
echo "----------------------------------------"

# 先检查端口是否已被占用
if lsof -i :50051 >/dev/null 2>&1; then
    echo "❌ 错误: 端口 50051 已被占用"
    echo "请先停止占用端口的进程："
    lsof -i :50051
    exit 1
fi

if lsof -i :50052 >/dev/null 2>&1; then
    echo "❌ 错误: 端口 50052 已被占用"
    echo "请先停止占用端口的进程："
    lsof -i :50052
    exit 1
fi

# 启动 server（后台运行）
echo "启动 server（后台运行）..."
go run server/main.go > server.log 2>&1 &
SERVER_PID=$!
echo "Server PID: $SERVER_PID"

# 短暂等待，检查 server 是否立即失败
sleep 2

# 检查 server 进程是否还在运行
if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo ""
    echo "❌ 错误: Server 启动失败（进程已退出）"
    echo ""
    echo "Server 日志："
    cat server.log
    echo ""
    echo "可能的原因："
    echo "  1. 端口 50051 或 50052 被占用"
    echo "  2. 编译错误"
    echo "  3. 依赖问题"
    echo ""
    echo "请检查端口占用："
    echo "  lsof -i :50051,50052"
    exit 1
fi

# 记录实际的 server 进程 PID
ACTUAL_SERVER_PID=$(lsof -ti :50051 2>/dev/null || true)
if [ ! -z "$ACTUAL_SERVER_PID" ]; then
    echo "Actual Server PID: $ACTUAL_SERVER_PID"
fi
echo ""

# 等待 server 启动
echo "等待 server 完全启动..."
MAX_WAIT=10
WAITED=0
while [ $WAITED -lt $MAX_WAIT ]; do
    if curl -s http://localhost:50052/debug/pprof >/dev/null 2>&1; then
        echo ""
        echo "✅ Server 启动成功！"
        break
    fi
    sleep 1
    WAITED=$((WAITED + 1))
    echo -n "."
done
echo ""

if [ $WAITED -eq $MAX_WAIT ]; then
    echo "❌ 错误: Server 启动超时"
    echo ""
    echo "Server 日志："
    cat server.log
    echo ""
    exit 1
fi

# 查看初始 goroutine 数量
INITIAL_GOROUTINES=$(curl -s http://localhost:50052/debug/pprof/goroutine?debug=1 | head -1 | grep -oE '[0-9]+' | head -1 || echo "0")
if [ "$INITIAL_GOROUTINES" = "0" ] || [ -z "$INITIAL_GOROUTINES" ]; then
    echo "❌ 错误: 无法获取 goroutine 数量"
    echo "请检查 pprof 服务是否正常：http://localhost:50052/debug/pprof"
    exit 1
fi
echo "初始 goroutine 数量: $INITIAL_GOROUTINES"
echo ""
sleep 1

# ============================================
# 步骤 2: 运行 good_client 并采集数据
# ============================================
echo "步骤 2: 运行 good_client (正确的连接复用)"
echo "----------------------------------------"

# 启动 good_client（后台运行）
echo "启动 good_client..."
go run good_client/main.go > good_client.log 2>&1 &
CLIENT_PID=$!
echo "Good Client PID: $CLIENT_PID"
echo ""

# 等待 2 秒
echo "等待 2 秒，让客户端发送请求..."
sleep 2

# 获取 goroutine 信息
echo "采集 goroutine 信息..."
GOOD_GOROUTINES=$(curl -s http://localhost:50052/debug/pprof/goroutine?debug=1 | head -1 | grep -oE '[0-9]+' | head -1 || echo "0")
if [ "$GOOD_GOROUTINES" = "0" ] || [ -z "$GOOD_GOROUTINES" ]; then
    echo "⚠️  警告: 无法获取 goroutine 数量，使用默认值"
    GOOD_GOROUTINES=$INITIAL_GOROUTINES
fi
echo "当前 goroutine 数量: $GOOD_GOROUTINES"

# 保存 goroutine 分组统计信息
GOOD_FILE="good_goroutine.txt"
curl -s http://localhost:50052/debug/pprof/goroutine?debug=1 > "$GOOD_FILE"
echo "✅ 已保存 goroutine 信息到 $GOOD_FILE"
echo ""

# 统计信息
GOOD_INCREASE=$((GOOD_GOROUTINES - INITIAL_GOROUTINES))
echo "📊 Good Client 统计："
echo "   初始 goroutine: $INITIAL_GOROUTINES"
echo "   当前 goroutine: $GOOD_GOROUTINES"
echo "   增加数量: $GOOD_INCREASE"
echo ""

# ============================================
# 步骤 3: 运行 bad_client 并采集数据
# ============================================
echo "步骤 3: 运行 bad_client (错误的连接管理)"
echo "----------------------------------------"

# 启动 bad_client（后台运行）
echo "启动 bad_client..."
go run bad_client/main.go > bad_client.log 2>&1 &
CLIENT_PID=$!
echo "Bad Client PID: $CLIENT_PID"
echo ""

# 等待 2 秒
echo "等待 2 秒，让客户端发送请求..."
sleep 2

# 获取 goroutine 信息
echo "采集 goroutine 信息..."
BAD_GOROUTINES=$(curl -s http://localhost:50052/debug/pprof/goroutine?debug=1 | head -1 | grep -oE '[0-9]+' | head -1 || echo "0")
if [ "$BAD_GOROUTINES" = "0" ] || [ -z "$BAD_GOROUTINES" ]; then
    echo "⚠️  警告: 无法获取 goroutine 数量，使用默认值"
    BAD_GOROUTINES=$GOOD_GOROUTINES
fi
echo "当前 goroutine 数量: $BAD_GOROUTINES"

# 保存 goroutine 分组统计信息
BAD_FILE="bad_goroutine.txt"
curl -s http://localhost:50052/debug/pprof/goroutine?debug=1 > "$BAD_FILE"
echo "✅ 已保存 goroutine 信息到 $BAD_FILE"
echo ""

# 统计信息
BAD_INCREASE=$((BAD_GOROUTINES - AFTER_GOOD))
echo "📊 Bad Client 统计："
echo "   开始时 goroutine: $AFTER_GOOD"
echo "   当前 goroutine: $BAD_GOROUTINES"
echo "   增加数量: $BAD_INCREASE"
echo ""

# 等待 bad_client 完成
echo "等待 bad_client 完成..."
wait $CLIENT_PID 2>/dev/null || true
CLIENT_PID=""
echo "✅ Bad Client 运行完成"
echo ""

# 最终统计
sleep 1
FINAL_GOROUTINES=$(curl -s http://localhost:50052/debug/pprof/goroutine?debug=1 | head -1 | grep -oE '[0-9]+' | head -1)

# ============================================
# 结果对比
# ============================================
echo "========================================"
echo "结果对比"
echo "========================================"
echo ""

echo "📊 Goroutine 数量变化："
echo "   初始状态:         $INITIAL_GOROUTINES"
echo "   Good Client 期间: $GOOD_GOROUTINES (增加 $GOOD_INCREASE)"
echo "   Good Client 之后: $AFTER_GOOD"
echo "   Bad Client 期间:  $BAD_GOROUTINES (增加 $BAD_INCREASE)"
echo "   最终状态:         $FINAL_GOROUTINES (累计泄漏 $((FINAL_GOROUTINES - INITIAL_GOROUTINES)))"
echo ""

echo "📁 生成的文件："
echo "   $GOOD_FILE - Good Client 的 goroutine 信息"
echo "   $BAD_FILE  - Bad Client 的 goroutine 信息"
echo "   server.log      - Server 日志"
echo "   good_client.log - Good Client 日志"
echo "   bad_client.log  - Bad Client 日志"
echo ""

# 分析泄漏的 goroutine
echo "🔍 分析泄漏的 goroutine："
echo ""

# 统计 good_goroutine 中的 goroutine 数量
GOOD_COUNT=$(grep -c "^goroutine " "$GOOD_FILE" || true)
echo "✅ Good Client: $GOOD_COUNT 个 goroutine"

# 统计 bad_goroutine 中的 goroutine 数量
BAD_COUNT=$(grep -c "^goroutine " "$BAD_FILE" || true)
echo "❌ Bad Client:  $BAD_COUNT 个 goroutine"

# 统计泄漏的 gRPC 相关 goroutine
GOOD_GRPC=$(grep -c "grpc" "$GOOD_FILE" || true)
BAD_GRPC=$(grep -c "grpc" "$BAD_FILE" || true)
echo ""
echo "gRPC 相关的 goroutine："
echo "   Good Client: $GOOD_GRPC 个"
echo "   Bad Client:  $BAD_GRPC 个"
echo ""

# 提取最频繁的堆栈
echo "Bad Client 中最频繁的堆栈 (前5个):"
grep "^goroutine " "$BAD_FILE" | head -5

echo ""
echo "========================================"
echo "分析建议"
echo "========================================"
echo ""
echo "1️⃣  查看 Good Client 的 goroutine 信息："
echo "   cat $GOOD_FILE"
echo ""
echo "2️⃣  查看 Bad Client 的 goroutine 信息："
echo "   cat $BAD_FILE"
echo ""
echo "3️⃣  对比两个文件的差异："
echo "   diff <(grep '^goroutine' $GOOD_FILE | sort | uniq) <(grep '^goroutine' $BAD_FILE | sort | uniq)"
echo ""
echo "4️⃣  查看泄漏的 gRPC goroutine："
echo "   grep -A 10 'grpc.*transport' $BAD_FILE | head -50"
echo ""
echo "5️⃣  查看客户端日志："
echo "   cat good_client.log"
echo "   cat bad_client.log"
echo ""

echo "========================================"
echo "✅ 演示完成！"
echo "========================================"
echo ""

echo "删除日志文件..."
rm -f server.log good_client.log bad_client.log
echo "✅ 日志文件已删除"
echo "保留的文件: $GOOD_FILE, $BAD_FILE"

echo ""
echo "再见！"
