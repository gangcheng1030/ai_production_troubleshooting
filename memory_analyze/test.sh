#!/bin/bash
# compare_servers.sh - 自动化对比 good_server 和 bad_server 的内存使用情况

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== FastHTTP 文件上传内存对比测试 ===${NC}"
echo ""

# 配置
GOOD_SERVER_PORT=8080
GOOD_PPROF_PORT=6060
BAD_SERVER_PORT=8080
BAD_PPROF_PORT=6060

# 清理函数
cleanup() {
    echo ""
    echo -e "${YELLOW}清理进程...${NC}"
    if [ ! -z "$GOOD_SERVER_PID" ]; then
        kill $GOOD_SERVER_PID 2>/dev/null || true
        wait $GOOD_SERVER_PID 2>/dev/null || true
    fi
    if [ ! -z "$BAD_SERVER_PID" ]; then
        kill $BAD_SERVER_PID 2>/dev/null || true
        wait $BAD_SERVER_PID 2>/dev/null || true
    fi
}

# 设置退出时清理
trap cleanup EXIT INT TERM

# 检查端口是否可用
check_port() {
    local port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1 ; then
        echo -e "${RED}错误: 端口 $port 已被占用${NC}"
        lsof -Pi :$port -sTCP:LISTEN
        exit 1
    fi
}

# 等待端口可用
wait_for_port() {
    local port=$1
    local timeout=10
    local count=0
    
    while ! nc -z localhost $port 2>/dev/null; do
        sleep 0.5
        count=$((count + 1))
        if [ $count -gt $((timeout * 2)) ]; then
            echo -e "${RED}错误: 端口 $port 启动超时${NC}"
            return 1
        fi
    done
    return 0
}

# 检查所有端口
echo -e "${YELLOW}检查端口可用性...${NC}"
check_port $GOOD_SERVER_PORT
check_port $GOOD_PPROF_PORT
check_port $BAD_SERVER_PORT
check_port $BAD_PPROF_PORT
echo -e "${GREEN}所有端口可用${NC}"
echo ""

#############################################
# 测试 Good Server
#############################################

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}测试 Good Server${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 启动 good_server
echo -e "${YELLOW}[1/5] 启动 good_server...${NC}"
go run ./good_server/main.go > good_server.log 2>&1 &
GOOD_SERVER_PID=$!
echo "PID: $GOOD_SERVER_PID"

# 等待服务器启动
sleep 2
if ! kill -0 $GOOD_SERVER_PID 2>/dev/null; then
    echo -e "${RED}good_server 启动失败！查看 good_server.log${NC}"
    cat good_server.log
    exit 1
fi

# 等待端口可用
if ! wait_for_port $GOOD_SERVER_PORT; then
    echo -e "${RED}good_server 端口未响应${NC}"
    exit 1
fi
echo -e "${GREEN}good_server 启动成功${NC}"
echo ""

# 启动 client 测试
echo -e "${YELLOW}[3/5] 启动 client 测试 good_server...${NC}"
echo "请求 URL: http://localhost:$GOOD_SERVER_PORT/upload"
go run ./client/main.go -url=http://localhost:$GOOD_SERVER_PORT/upload > good_client.log 2>&1

echo -e "${GREEN}client 测试完成${NC}"
echo ""

# 等待服务器处理完成
echo -e "${YELLOW}[4/5] 等待服务器处理完成...${NC}"
sleep 2

# 获取测试后的内存快照（inuse 内存）
echo -e "${YELLOW}[5/5] 获取 inuse 内存信息...${NC}"
curl -s http://localhost:$GOOD_PPROF_PORT/debug/pprof/heap > good_heap.prof
echo -e "${GREEN}已保存: good_heap.prof${NC}"

# 停止 good_server
echo -e "${YELLOW}停止 good_server...${NC}"
curl -s http://localhost:$GOOD_SERVER_PORT/exit > /dev/null 2>&1
echo -e "${GREEN}good_server 已停止${NC}"
echo ""

# 等待端口释放
sleep 2

#############################################
# 测试 Bad Server
#############################################

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}测试 Bad Server${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 启动 bad_server
echo -e "${YELLOW}[1/5] 启动 bad_server...${NC}"
go run ./bad_server/main.go > bad_server.log 2>&1 &
BAD_SERVER_PID=$!
echo "PID: $BAD_SERVER_PID"

# 等待服务器启动
sleep 2
if ! kill -0 $BAD_SERVER_PID 2>/dev/null; then
    echo -e "${RED}bad_server 启动失败！查看 bad_server.log${NC}"
    cat bad_server.log
    exit 1
fi

# 等待端口可用
if ! wait_for_port $BAD_SERVER_PORT; then
    echo -e "${RED}bad_server 端口未响应${NC}"
    exit 1
fi
echo -e "${GREEN}bad_server 启动成功${NC}"
echo ""

# 启动 client 测试
echo -e "${YELLOW}[3/5] 启动 client 测试 bad_server...${NC}"
echo "请求 URL: http://localhost:$BAD_SERVER_PORT/upload"
go run ./client/main.go -url=http://localhost:$BAD_SERVER_PORT/upload > bad_client.log 2>&1

echo -e "${GREEN}client 测试完成${NC}"
echo ""

# 等待服务器处理完成
echo -e "${YELLOW}[4/5] 等待服务器处理完成...${NC}"
sleep 2

# 获取测试后的内存快照（inuse 内存）
echo -e "${YELLOW}[5/5] 获取 inuse 内存信息...${NC}"
curl -s http://localhost:$BAD_PPROF_PORT/debug/pprof/heap > bad_heap.prof
echo -e "${GREEN}已保存: bad_heap.prof${NC}"

# 停止 bad_server
echo -e "${YELLOW}停止 bad_server...${NC}"
curl -s http://localhost:$BAD_SERVER_PORT/exit > /dev/null 2>&1
echo -e "${GREEN}bad_server 已停止${NC}"
echo ""

# 等待端口释放
sleep 2

# 清理日志文件
rm -f good_server.log good_client.log bad_server.log bad_client.log