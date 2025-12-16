# 网络硬件故障排查指南
_Exported on 2025/12/16 at GMT+8_

---

**User**

有个线上问题我怀疑是网卡，网线，路由器这类网络硬件故障导致的，请教下我怎样验证这些硬件是否发生了故障。

---

**Cursor**

网络硬件故障是生产环境中常见但容易被忽视的问题。下面我将系统地介绍如何排查网卡、网线、路由器等硬件故障。

## 一、网卡（NIC）故障排查

### 1. **查看网卡基本状态**

```bash
# 查看所有网卡信息
ip link show
# 或
ifconfig -a

# 查看网卡统计信息
ip -s link show eth0

# 查看网卡驱动信息
ethtool -i eth0

# 查看网卡详细状态
ethtool eth0
```

**关键指标：**
```bash
# 期望看到：
# - State: UP
# - Link detected: yes
# - Speed: 1000Mb/s (或更高)
# - Duplex: Full (全双工)
# - Auto-negotiation: on (自动协商)
```

### 2. **检查网卡错误统计**

```bash
# 查看网卡错误计数
netstat -i

# 详细的网卡统计
cat /proc/net/dev

# 使用 ethtool 查看详细统计
ethtool -S eth0
```

**重点关注的错误指标：**

```bash
# RX errors (接收错误)
RX errors: 0
RX dropped: 0      # 接收丢包
RX overruns: 0     # 接收缓冲区溢出
RX frame: 0        # 帧错误

# TX errors (发送错误)
TX errors: 0
TX dropped: 0      # 发送丢包
TX carrier: 0      # 载波错误（网线问题）
TX collisions: 0   # 碰撞（半双工或网线问题）

# 其他重要指标
rx_crc_errors      # CRC校验错误（网线或网卡问题）
rx_length_errors   # 长度错误
tx_aborted_errors  # 发送中止
```

**判断标准：**
- 如果错误计数持续增加 → 硬件可能有问题
- carrier errors / collisions 增加 → 网线或双工模式问题
- CRC errors 增加 → 网线质量差或网卡故障

### 3. **检查网卡中断和队列**

```bash
# 查看网卡中断
cat /proc/interrupts | grep eth0

# 查看网卡队列统计
tc -s qdisc show dev eth0

# 查看丢包情况
ethtool -S eth0 | grep -i drop
ethtool -S eth0 | grep -i error
ethtool -S eth0 | grep -i crc
```

### 4. **测试网卡性能**

```bash
# 使用 iperf3 测试带宽（需要在两台机器上运行）
# 服务端：
iperf3 -s

# 客户端：
iperf3 -c <server_ip> -t 30 -P 4

# 预期结果：应该接近网卡理论带宽（如千兆网卡应该有 900+ Mbps）
```

### 5. **检查网卡驱动和固件**

```bash
# 查看内核日志中的网卡错误
dmesg | grep -i eth0
dmesg | grep -i "network\|nic\|link"

# 查看驱动版本
ethtool -i eth0

# 查看系统日志
journalctl -u NetworkManager -n 100
tail -f /var/log/syslog | grep -i network
```

**常见告警信息：**
```
- "link down" / "link up" 频繁交替 → 网线松动或损坏
- "transmit timeout" → 网卡驱动问题或硬件故障
- "NETDEV WATCHDOG" → 网卡发送队列堵塞
- "hardware error" → 网卡硬件故障
```

## 二、网线故障排查

### 1. **物理检查**

**检查清单：**
- [ ] 网线两端是否插紧（重新拔插试试）
- [ ] 网线是否被压扁、弯折、破损
- [ ] 网线长度是否超标（超五类线建议 < 100米）
- [ ] 网线是否靠近强电磁干扰源（电机、变压器等）
- [ ] 水晶头是否氧化、松动

### 2. **检查链路状态**

```bash
# 查看链路协商状态
ethtool eth0 | grep -E "Speed|Duplex|Link"

# 监控链路状态变化
watch -n 1 'ethtool eth0 | grep "Link detected"'

# 查看链路抖动
ip -s link show eth0
# 等待几分钟后再次执行，对比 "link" 计数是否增加
```

**异常情况：**
- Speed 低于预期（如千兆网卡只有 100Mbps）→ 网线质量差或网卡降速
- Duplex: Half（半双工）→ 协商失败，可能是网线或对端设备问题
- Link detected 频繁从 yes 变为 no → 网线松动或损坏

### 3. **测试网线质量**

```bash
# 使用 ping 测试丢包和延迟
ping -c 1000 -i 0.2 <gateway_or_peer_ip>

# 关键指标：
# - 丢包率应该为 0%
# - RTT 应该稳定（标准差小）
# - 如果是局域网，RTT 应该 < 1ms

# 查看详细的 ping 统计
ping -c 1000 -i 0.2 -D <ip> | tee ping_test.log
# -D: 显示时间戳
# -i 0.2: 每 0.2 秒发一个包（更密集的测试）

# 使用 mtr 进行持续监控（结合了 ping 和 traceroute）
mtr -r -c 100 <ip>
# 查看每一跳的丢包率和延迟
```

**判断标准：**
```
局域网 ping 测试：
- 丢包率 = 0%          → 正常
- 丢包率 0.1% - 1%     → 网线质量差或干扰
- 丢包率 > 1%          → 严重问题
- RTT 抖动 > 10ms      → 链路不稳定

- RTT 突然出现 > 100ms 的尖峰 → 可能是网线或交换机问题
```

### 4. **专业工具测试**（如果可用）

```bash
# 网线测试仪（硬件工具）可以检测：
# - 线序是否正确
# - 是否有断线
# - 串扰程度
# - 衰减程度
# - 长度测量

# 如果有万用表，可以测试：
# - 线芯通断
# - 阻抗是否正常（标准为 100Ω ± 15Ω）
```

## 三、交换机/路由器故障排查

### 1. **检查交换机端口状态**

```bash
# 如果有交换机管理权限（以 Cisco 为例）
show interface status
show interface <port> counters errors

# 关键指标：
# - CRC errors
# - Collisions
# - Late collisions
# - Input/Output errors
```

### 2. **查看网关/路由器连通性**

```bash
# 查看网关
ip route show
# 或
route -n

# ping 网关
ping -c 100 <gateway_ip>

# traceroute 到外部 IP，查看路由路径
traceroute -n 8.8.8.8
# 或使用 mtr
mtr -r -c 50 8.8.8.8

# 检查 ARP 表
arp -n
ip neigh show

# 如果 ARP 表中网关的 MAC 地址频繁变化，可能是：
# - 网关设备故障切换
# - ARP 欺骗
# - 网络配置错误
```

### 3. **检查端口镜像/流量分析**（需要交换机支持）

```bash
# 在交换机上配置端口镜像，然后用 tcpdump 抓包分析
tcpdump -i eth0 -w capture.pcap

# 使用 Wireshark 分析，重点看：
# - TCP Retransmission（重传）
# - TCP Dup ACK（重复确认）
# - TCP Out-of-Order（乱序）
# - Malformed packets（畸形包）
```

### 4. **检查交换机/路由器日志**

```bash
# 登录交换机查看日志（具体命令取决于设备型号）
show logging
show log

# 关注的错误信息：
# - Port flapping (端口频繁 up/down)
# - Error disabled
# - STP topology changes (生成树拓扑变化)
# - Power issues (供电问题)
```

## 四、综合诊断方法

### 1. **实时监控网络指标**

创建一个监控脚本：

```bash
#!/bin/bash
# network_monitor.sh

INTERFACE="eth0"
INTERVAL=5

echo "开始监控网卡 $INTERFACE，每 ${INTERVAL} 秒采样一次"
echo "时间戳,RX丢包,RX错误,TX丢包,TX错误,碰撞,载波错误"

while true; do
    TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
    
    # 读取网卡统计
    STATS=$(cat /sys/class/net/$INTERFACE/statistics/{rx_dropped,rx_errors,tx_dropped,tx_errors,collisions,tx_carrier_errors})
    
    # 使用 ethtool 获取更详细的统计
    CRC_ERRORS=$(ethtool -S $INTERFACE 2>/dev/null | grep -i "rx_crc_errors" | awk '{print $2}')
    
    echo "$TIMESTAMP,$STATS,$CRC_ERRORS"
    
    sleep $INTERVAL
done
```

运行：
```bash
chmod +x network_monitor.sh
./network_monitor.sh | tee network_monitor.log
```

### 2. **压力测试**

```bash
# 使用 iperf3 进行双向测试
# 服务端
iperf3 -s

# 客户端（TCP 测试）
iperf3 -c <server> -t 60 -P 10

# 客户端（UDP 测试，可以发现丢包）
iperf3 -c <server> -u -b 1000M -t 60

# 同时运行多个 iperf3 实例，模拟高负载
for i in {1..5}; do
    iperf3 -c <server> -t 60 -P 4 &
done
wait

# 观察：
# - 带宽是否达到预期
# - 是否有丢包
# - CPU 使用率（排除 CPU 瓶颈）
```

### 3. **长时间稳定性测试**

```bash
# 创建长时间 ping 测试脚本
#!/bin/bash
# long_term_ping.sh

TARGET=$1
DURATION=$((24 * 3600))  # 24 小时

if [ -z "$TARGET" ]; then
    echo "用法: $0 <target_ip>"
    exit 1
fi

echo "开始对 $TARGET 进行长时间 ping 测试（24小时）"
timeout ${DURATION}s ping -D -i 1 $TARGET | while read line; do
    echo "$(date '+%Y-%m-%d %H:%M:%S') $line"
done | tee ping_long_term.log

# 分析结果
echo "========== 测试统计 =========="
grep "packets transmitted" ping_long_term.log | tail -1
grep "rtt min/avg/max" ping_long_term.log | tail -1

# 查找异常延迟
echo "========== RTT > 10ms 的记录 =========="
grep "time=" ping_long_term.log | awk '{
    for(i=1;i<=NF;i++) {
        if($i ~ /time=/) {
            split($i, a, "=")
            if(a[2] > 10) print $0
        }
    }
}'
```

### 4. **对比测试**

```bash
# 如果怀疑某台机器的网络有问题，可以：
# 1. 在同一交换机上找一台正常的机器
# 2. 在两台机器上同时运行相同的测试
# 3. 对比结果

# 例如：同时 ping 网关
# 正常机器：
ping -c 1000 <gateway> | tee ping_good.log

# 问题机器：
ping -c 1000 <gateway> | tee ping_bad.log

# 对比丢包率和延迟分布
```

## 五、常见故障模式和判断

### 1. **网卡硬件故障**

**症状：**
- ethtool -S 显示大量 CRC 错误、硬件错误
- dmesg 中频繁出现网卡相关错误
- 更换网线后问题依旧
- 网卡指示灯异常（不亮或闪烁异常）

**验证：**
```bash
# 查看 PCIe 错误
lspci -vv | grep -A 10 "Ethernet controller"
dmesg | grep -i "pcie\|aer"

# 如果有备用网卡，可以：
# 1. 配置备用网卡
# 2. 将流量切换到备用网卡
# 3. 观察问题是否消失
```

**解决：**
- 更新网卡驱动和固件
- 更换网卡

### 2. **网线故障**

**症状：**
- ping 有间歇性丢包（通常是规律性的）
- ethtool 显示协商速率降低（千兆降到百兆）
- Link detected 频繁抖动
- TX carrier errors 增加

**验证：**
```bash
# 重新插拔网线后问题暂时消失
# 更换网线后问题消失

# 检查网线长度
ethtool --cable-test eth0  # 某些网卡支持
```

**解决：**
- 更换质量好的网线（Cat 5e 或 Cat 6）
- 确保网线长度不超过 100 米
- 远离电磁干扰源

### 3. **交换机端口故障**

**症状：**
- 多台连接同一交换机的机器同时出现问题
- 交换机端口 LED 指示灯异常
- 交换机日志显示端口错误

**验证：**
```bash
# 将网线插到交换机的另一个端口
# 如果问题消失，说明原端口有问题

# 或者：
# 1. 用一台正常机器插到可疑端口
# 2. 运行网络测试
# 3. 看是否出现相同问题
```

**解决：**
- 使用交换机的其他端口
- 更换交换机

### 4. **光模块故障**（如果使用光纤）

**症状：**
- 光功率异常
- CRC 错误大量增加
- 链路不稳定

**验证：**
```bash
# 查看光模块信息（如果支持）
ethtool -m eth0

# 关注：
# - Laser output power (发送光功率)
# - Receiver signal average optical power (接收光功率)
# - Temperature (温度)

# 正常范围（以常见的 SFP+ 为例）：
# - TX Power: -7 ~ 0 dBm
# - RX Power: -14 ~ 0 dBm
# - Temp: 0 ~ 70°C
```

**解决：**
- 清洁光纤接头
- 更换光模块
- 检查光纤是否弯折过度

## 六、自动化检查脚本

创建一个综合的网络硬件检查脚本：

```bash
#!/bin/bash
# network_hardware_check.sh

set -e

INTERFACE=${1:-eth0}
OUTPUT_DIR="./network_check_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$OUTPUT_DIR"

echo "========================================"
echo "网络硬件诊断脚本"
echo "检查接口: $INTERFACE"
echo "报告目录: $OUTPUT_DIR"
echo "========================================"
echo ""

# 1. 基本信息
echo "[1/10] 收集基本网络信息..."
{
    echo "=== 系统信息 ==="
    uname -a
    echo ""
    
    echo "=== 网卡列表 ==="
    ip link show
    echo ""
    
    echo "=== IP 地址 ==="
    ip addr show
    echo ""
    
    echo "=== 路由表 ==="
    ip route show
    echo ""
} > "$OUTPUT_DIR/01_basic_info.txt"

# 2. 网卡详细状态
echo "[2/10] 检查网卡状态..."
{
    echo "=== 网卡状态 (ethtool) ==="
    ethtool $INTERFACE 2>&1
    echo ""
    
    echo "=== 网卡驱动信息 ==="
    ethtool -i $INTERFACE 2>&1
    echo ""
    
    echo "=== 网卡统计信息 ==="
    ethtool -S $INTERFACE 2>&1
    echo ""
} > "$OUTPUT_DIR/02_nic_status.txt"

# 3. 错误统计
echo "[3/10] 收集错误统计..."
{
    echo "=== 接口错误统计 (netstat) ==="
    netstat -i
    echo ""
    
    echo "=== 接口错误统计 (/proc/net/dev) ==="
    cat /proc/net/dev
    echo ""
    
    echo "=== 详细错误计数 ==="
    for stat in rx_errors tx_errors rx_dropped tx_dropped collisions \
                rx_crc_errors rx_frame_errors tx_carrier_errors; do
        FILE="/sys/class/net/$INTERFACE/statistics/$stat"
        if [ -f "$FILE" ]; then
            echo "$stat: $(cat $FILE)"
        fi
    done
    echo ""
} > "$OUTPUT_DIR/03_error_stats.txt"

# 4. 内核日志
echo "[4/10] 检查内核日志..."
{
    echo "=== 最近的网卡相关内核消息 ==="
    dmesg | grep -i "$INTERFACE\|network\|nic\|ethernet" | tail -100
    echo ""
} > "$OUTPUT_DIR/04_kernel_log.txt"

# 5. 中断信息
echo "[5/10] 检查中断..."
{
    echo "=== 网卡中断 ==="
    cat /proc/interrupts | head -1
    cat /proc/interrupts | grep -i "$INTERFACE\|eth"
    echo ""
} > "$OUTPUT_DIR/05_interrupts.txt"

# 6. 测试网关连通性
echo "[6/10] 测试网关连通性..."
GATEWAY=$(ip route | grep default | awk '{print $3}' | head -1)
if [ -n "$GATEWAY" ]; then
    {
        echo "=== Ping 网关 ($GATEWAY) ==="
        ping -c 100 -i 0.2 $GATEWAY 2>&1
        echo ""
    } > "$OUTPUT_DIR/06_gateway_ping.txt"
else
    echo "未找到默认网关" > "$OUTPUT_DIR/06_gateway_ping.txt"
fi

# 7. MTR 测试
echo "[7/10] 运行 MTR 测试..."
if command -v mtr &> /dev/null && [ -n "$GATEWAY" ]; then
    {
        echo "=== MTR 测试到网关 ==="
        mtr -r -c 50 $GATEWAY 2>&1
        echo ""
    } > "$OUTPUT_DIR/07_mtr_test.txt"
else
    echo "MTR 未安装或无网关" > "$OUTPUT_DIR/07_mtr_test.txt"
fi

# 8. ARP 表
echo "[8/10] 检查 ARP 表..."
{
    echo "=== ARP 表 ==="
    ip neigh show
    echo ""
    arp -n
    echo ""
} > "$OUTPUT_DIR/08_arp_table.txt"

# 9. 连接状态
echo "[9/10] 检查连接状态..."
{
    echo "=== TCP 连接统计 ==="
    ss -s
    echo ""
    
    echo "=== 连接状态分布 ==="
    ss -tan | awk '{print $1}' | sort | uniq -c | sort -rn
    echo ""
} > "$OUTPUT_DIR/09_connections.txt"

# 10. PCIe 信息（如果适用）
echo "[10/10] 检查 PCIe 信息..."
{
    echo "=== PCIe 网卡信息 ==="
    lspci | grep -i ethernet
    echo ""
    
    # 获取网卡的 PCIe 地址
    NIC_PCI=$(ethtool -i $INTERFACE 2>/dev/null | grep "bus-info" | awk '{print $2}')
    if [ -n "$NIC_PCI" ]; then
        echo "=== 详细 PCIe 信息 ($NIC_PCI) ==="
        lspci -vv -s "$NIC_PCI" 2>&1
    fi
    echo ""
} > "$OUTPUT_DIR/10_pcie_info.txt"

# 生成摘要报告
echo ""
echo "========================================"
echo "生成诊断摘要..."
echo "========================================"
{
    echo "网络硬件诊断摘要报告"
    echo "===================="
    echo "生成时间: $(date)"
    echo "接口: $INTERFACE"
    echo ""
    
    echo "## 1. 链路状态"
    LINK_STATUS=$(cat "$OUTPUT_DIR/02_nic_status.txt" | grep "Link detected" | awk '{print $3}')
    SPEED=$(cat "$OUTPUT_DIR/02_nic_status.txt" | grep "Speed:" | awk '{print $2}')
    DUPLEX=$(cat "$OUTPUT_DIR/02_nic_status.txt" | grep "Duplex:" | awk '{print $2}')
    echo "   链路: $LINK_STATUS"
    echo "   速率: $SPEED"
    echo "   双工: $DUPLEX"
    echo ""
    
    echo "## 2. 错误统计"
    for stat in rx_errors tx_errors rx_dropped tx_dropped collisions; do
        FILE="/sys/class/net/$INTERFACE/statistics/$stat"
        if [ -f "$FILE" ]; then
            COUNT=$(cat "$FILE")
            if [ "$COUNT" -gt 0 ]; then
                echo "   ⚠️  $stat: $COUNT"
            else
                echo "   ✓  $stat: $COUNT"
            fi
        fi
    done
    echo ""
    
    echo "## 3. 网关连通性"
    if [ -f "$OUTPUT_DIR/06_gateway_ping.txt" ]; then
        PACKET_LOSS=$(grep "packet loss" "$OUTPUT_DIR/06_gateway_ping.txt" | grep -oP '\d+(?=% packet loss)' || echo "N/A")
        RTT_AVG=$(grep "rtt min/avg/max" "$OUTPUT_DIR/06_gateway_ping.txt" | awk -F'/' '{print $5}' || echo "N/A")
        echo "   丢包率: $PACKET_LOSS%"
        echo "   平均 RTT: $RTT_AVG ms"
    fi
    echo ""
    
    echo "## 4. 关键告警"
    WARNINGS=0
    
    # 检查链路状态
    if [ "$LINK_STATUS" != "yes" ]; then
        echo "   🔴 链路未连接"
        ((WARNINGS++))
    fi
    
    # 检查速率（假设期望千兆）
    if [[ "$SPEED" =~ "100" ]]; then
        echo "   ⚠️  速率降至 100Mbps（可能是网线问题）"
        ((WARNINGS++))
    fi
    
    # 检查双工模式
    if [ "$DUPLEX" == "Half" ]; then
        echo "   ⚠️  半双工模式（可能是协商问题）"
        ((WARNINGS++))
    fi
    
    # 检查错误计数
    RX_ERR=$(cat /sys/class/net/$INTERFACE/statistics/rx_errors)
    TX_ERR=$(cat /sys/class/net/$INTERFACE/statistics/tx_errors)
    if [ "$RX_ERR" -gt 100 ] || [ "$TX_ERR" -gt 100 ]; then
        echo "   ⚠️  接口错误计数较高"
        ((WARNINGS++))
    fi
    
    # 检查丢包
    if [ "$PACKET_LOSS" != "N/A" ] && [ "$PACKET_LOSS" -gt 1 ]; then
        echo "   🔴 网关 ping 丢包率 > 1%"
        ((WARNINGS++))
    fi
    
    if [ $WARNINGS -eq 0 ]; then
        echo "   ✓ 未发现明显异常"
    fi
    echo ""
    
    echo "## 5. 建议"
    if [ $WARNINGS -gt 0 ]; then
        echo "   1. 检查网线连接是否牢固"
        echo "   2. 尝试更换网线"
        echo "   3. 查看 $OUTPUT_DIR/04_kernel_log.txt 中的错误信息"
        echo "   4. 运行长时间监控脚本（network_monitor.sh）"
        echo "   5. 如果问题持续，考虑更换网卡或联系网络管理员检查交换机"
    else
        echo "   硬件层面看起来正常。如果仍有网络问题，建议检查："
        echo "   1. 上层应用配置"
        echo "   2. 防火墙规则"
        echo "   3. 路由配置"
        echo "   4. DNS 解析"
    fi
    echo ""
    
    echo "完整报告保存在: $OUTPUT_DIR/"
    
} | tee "$OUTPUT_DIR/00_SUMMARY.txt"

echo ""
echo "========================================"
echo "诊断完成！"
echo "报告位置: $OUTPUT_DIR/"
echo "摘要: $OUTPUT_DIR/00_SUMMARY.txt"
echo "========================================"
```

使用方法：
```bash
chmod +x network_hardware_check.sh

# 检查默认网卡 eth0
sudo ./network_hardware_check.sh

# 检查指定网卡
sudo ./network_hardware_check.sh ens33

# 查看摘要报告
cat network_check_*/00_SUMMARY.txt
```

## 七、判断流程图

```
网络问题
  |
  ├─> 运行 network_hardware_check.sh
  |
  ├─> 查看摘要报告
  |
  ├─> 错误计数增加？
  |   ├─ Yes ─> CRC/Frame 错误？
  |   |         ├─ Yes ─> 可能是网线或网卡问题
  |   |         |         ├─> 更换网线测试
  |   |         |         └─> 更换网卡测试
  |   |         |
  |   |         └─ Carrier/Collision 错误？
  |   |                   └─> 网线或双工模式问题
  |   |                       ├─> 检查双工设置
  |   |                       └─> 更换网线
  |   |
  |   └─ No ─> 链路状态稳定？
  |             ├─ No ─> Link 频繁抖动
  |             |        └─> 网线松动或损坏
  |             |            └─> 重新插拔/更换网线
  |             |
  |             └─ Yes ─> 速率/双工正常？
  |                       ├─ No ─> 协商问题
  |                       |        └─> 检查网线质量
  |                       |        └─> 检查对端设备
  |                       |
  |                       └─ Yes ─> 网关 ping 丢包？
  |                                 ├─ Yes ─> 运行长时间测试
  |                                 |         └─> 分析丢包规律
  |                                 |
  |                                 └─ No ─> 硬件可能正常
  |                                           └─> 检查上层问题
```

## 八、最佳实践

### 1. **建立基线**

```bash
# 在系统正常时记录基线数据
mkdir -p /opt/network_baseline
sudo ethtool -S eth0 > /opt/network_baseline/eth0_stats_baseline.txt
sudo ip -s link show eth0 > /opt/network_baseline/eth0_link_baseline.txt

# 故障时对比
diff /opt/network_baseline/eth0_stats_baseline.txt <(ethtool -S eth0)
```

### 2. **定期检查**

```bash
# 添加到 crontab，每小时检查一次
crontab -e
# 添加：
0 * * * * /usr/local/bin/network_monitor.sh >> /var/log/network_monitor.log 2>&1
```

### 3. **告警设置**

```bash
#!/bin/bash
# network_alert.sh

INTERFACE="eth0"
ERROR_THRESHOLD=100

RX_ERR=$(cat /sys/class/net/$INTERFACE/statistics/rx_errors)
TX_ERR=$(cat /sys/class/net/$INTERFACE/statistics/tx_errors)

if [ "$RX_ERR" -gt "$ERROR_THRESHOLD" ] || [ "$TX_ERR" -gt "$ERROR_THRESHOLD" ]; then
    # 发送告警（根据你的环境选择告警方式）
    echo "网卡 $INTERFACE 错误计数超过阈值: RX=$RX_ERR, TX=$TX_ERR" | \
        mail -s "网络硬件告警" admin@example.com
fi
```

### 4. **文档记录**

建议记录以下信息：
- 网络拓扑图
- 设备型号和固件版本
- 网线规格和布线路径
- 历史故障记录
- 更换记录

## 九、总结

**快速诊断步骤：**

1. **运行综合检查脚本**
   ```bash
   sudo ./network_hardware_check.sh eth0
   ```

2. **查看摘要报告**
   ```bash
   cat network_check_*/00_SUMMARY.txt
   ```

3. **根据摘要报告的建议进行排查**

4. **如有异常，依次尝试：**
   - 重新插拔网线
   - 更换网线
   - 更换交换机端口
   - 更换网卡

5. **长时间监控验证**
   ```bash
   # 运行 24 小时监控
   sudo ./network_monitor.sh &
   sudo ./long_term_ping.sh <gateway_ip> &
   ```

**关键指标记忆：**
- 丢包率应为 0%
- 局域网 RTT < 1ms
- CRC/Frame/Carrier 错误应为 0
- 链路状态应稳定（Link detected: yes）
- 速率和双工模式应符合预期

**常见故障对应：**
- CRC 错误增加 → 网线或网卡
- Carrier 错误增加 → 网线
- Link 抖动 → 网线松动
- 速率降低 → 网线质量差
- 持续丢包 → 硬件故障或干扰

希望这份指南能帮助你快速定位网络硬件问题！

