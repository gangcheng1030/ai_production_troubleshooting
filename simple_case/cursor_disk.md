# 常见磁盘问题及验证
_Exported on 2025/12/16 at GMT+8 16:06:09 from Cursor (2.1.46)_

---

**User**

线上有个故障我怀疑是磁盘问题导致的，请问有哪些常见的磁盘类问题，应该怎样验证？

---

**Cursor**

我来帮你梳理常见的磁盘类问题及验证方法。线上磁盘问题通常会导致服务响应慢、卡顿、甚至不可用。

## 常见磁盘问题类型

### 1. **磁盘空间不足**

**症状：**
- 应用无法写入日志
- 数据库写入失败
- 临时文件创建失败

**验证方法：**
```bash
# 查看磁盘空间使用情况
df -h

# 查看inode使用情况（文件数量限制）
df -i

# 查找大文件（前20个）
find / -type f -size +100M -exec ls -lh {} \; 2>/dev/null | sort -k5 -hr | head -20

# 查看目录占用空间（按大小排序）
du -h --max-depth=1 / | sort -hr | head -20

# 查看已删除但未释放的文件（进程仍持有文件句柄）
lsof | grep deleted
```

### 2. **磁盘IO性能瓶颈**

**症状：**
- 应用响应变慢
- CPU iowait 高
- 读写操作耗时增加

**验证方法：**
```bash
# 查看IO等待情况
iostat -x 1 10
# 关注：%util（使用率）、await（平均等待时间）、r/s、w/s

# 实时监控IO情况
iotop -oP
# 查看哪些进程在进行IO操作

# 查看CPU iowait
top
# 关注 %wa（iowait）指标

# 更详细的IO统计
vmstat 1 10
# 关注 bi（块设备读入）、bo（块设备写出）、wa（iowait）

# 查看磁盘IO队列深度
cat /proc/diskstats
# 或使用
sar -d 1 10
```

**性能基准测试：**
```bash
# 测试磁盘顺序写性能
dd if=/dev/zero of=/tmp/test bs=1M count=1024 oflag=direct
# 测试磁盘顺序读性能
dd if=/tmp/test of=/dev/null bs=1M iflag=direct

# 使用fio进行更专业的测试
fio --name=random-write --ioengine=libaio --rw=randwrite --bs=4k \
    --numjobs=4 --size=1G --runtime=60 --time_based --end_fsync=1

# 测试随机读
fio --name=random-read --ioengine=libaio --rw=randread --bs=4k \
    --numjobs=4 --size=1G --runtime=60 --time_based
```

### 3. **磁盘硬件故障**

**症状：**
- 读写错误
- 系统日志中有磁盘错误
- 磁盘IO超时

**验证方法：**
```bash
# 查看内核日志中的磁盘错误
dmesg | grep -i "error\|fail\|warn" | grep -i "sd\|disk\|ata"
tail -f /var/log/kern.log | grep -i "disk\|sd"

# 查看SMART信息（硬盘自检）
smartctl -a /dev/sda
smartctl -H /dev/sda  # 快速健康检查

# 查看磁盘错误统计
cat /sys/block/sda/stat
# 或
iostat -x | grep -i error
```

### 4. **文件系统问题**

**症状：**
- 文件损坏
- 只读文件系统
- 挂载点异常

**验证方法：**
```bash
# 查看文件系统挂载状态
mount | grep -v tmpfs
cat /proc/mounts

# 检查是否有只读挂载
mount | grep "ro,"

# 查看文件系统类型和状态
lsblk -f

# 文件系统错误检查（需要卸载或只读模式）
fsck -n /dev/sda1  # -n 表示不修复，只检查

# 查看XFS文件系统错误（如果使用XFS）
xfs_repair -n /dev/sda1
```

### 5. **磁盘队列拥塞**

**症状：**
- 请求堆积
- await时间很高
- svctm正常但await很高

**验证方法：**
```bash
# 查看设备队列深度
cat /sys/block/sda/queue/nr_requests

# 查看IO调度器
cat /sys/block/sda/queue/scheduler

# 监控IO队列长度
iostat -x 1
# 关注 avgqu-sz（平均队列长度）
```

### 6. **网络存储问题（NFS/iSCSI等）**

**症状：**
- 网络存储访问超时
- 挂载点不响应

**验证方法：**
```bash
# 查看NFS挂载状态
nfsstat -m
showmount -e <nfs_server>

# 检查NFS IO统计
nfsstat -c

# 查看挂载点是否卡死
df -h  # 如果卡住，说明有问题
timeout 5 ls /nfs/mount/point

# 检查网络延迟
ping <storage_server>
```

## 综合排查脚本

```bash
#!/bin/bash
# 磁盘问题快速诊断脚本

echo "=== 磁盘空间检查 ==="
df -h
echo ""

echo "=== Inode使用情况 ==="
df -i
echo ""

echo "=== 磁盘IO统计（5次采样） ==="
iostat -x 1 5
echo ""

echo "=== 最耗IO的进程 ==="
iotop -b -n 1 -o | head -20
echo ""

echo "=== CPU iowait 情况 ==="
vmstat 1 5
echo ""

echo "=== 磁盘错误日志 ==="
dmesg | grep -i "error\|fail" | grep -i "disk\|sd\|ata" | tail -20
echo ""

echo "=== 只读文件系统检查 ==="
mount | grep "ro,"
echo ""

echo "=== 已删除但未释放的文件 ==="
lsof 2>/dev/null | grep deleted | awk '{sum+=$8} END {print "Total:", sum/1024/1024, "MB"}'
echo ""
```

## Go应用特定的磁盘问题排查

```go
// 监控磁盘IO的代码示例
package main

import (
    "fmt"
    "os"
    "syscall"
    "time"
)

func CheckDiskSpace(path string) error {
    var stat syscall.Statfs_t
    if err := syscall.Statfs(path, &stat); err != nil {
        return err
    }
    
    // 可用空间
    available := stat.Bavail * uint64(stat.Bsize)
    // 总空间
    total := stat.Blocks * uint64(stat.Bsize)
    // 使用率
    usedPercent := float64(total-available) / float64(total) * 100
    
    fmt.Printf("磁盘空间: 总共 %d GB, 可用 %d GB, 使用率 %.2f%%\n",
        total/1024/1024/1024, available/1024/1024/1024, usedPercent)
    
    if usedPercent > 90 {
        return fmt.Errorf("磁盘空间不足，使用率 %.2f%%", usedPercent)
    }
    return nil
}

// 监控文件写入性能
func MonitorDiskWrite(path string) {
    start := time.Now()
    
    f, err := os.Create(path)
    if err != nil {
        fmt.Printf("创建文件失败: %v\n", err)
        return
    }
    defer os.Remove(path)
    defer f.Close()
    
    data := make([]byte, 1024*1024) // 1MB
    n, err := f.Write(data)
    if err != nil {
        fmt.Printf("写入失败: %v\n", err)
        return
    }
    
    if err := f.Sync(); err != nil {
        fmt.Printf("同步失败: %v\n", err)
        return
    }
    
    duration := time.Since(start)
    throughput := float64(n) / duration.Seconds() / 1024 / 1024
    
    fmt.Printf("写入 %d bytes, 耗时 %v, 吞吐量 %.2f MB/s\n", n, duration, throughput)
    
    if duration > 100*time.Millisecond {
        fmt.Printf("警告: 磁盘写入慢，耗时 %v\n", duration)
    }
}
```

## 关键指标判断标准

| 指标 | 正常范围 | 告警阈值 | 说明 |
|------|----------|----------|------|
| 磁盘使用率 | < 70% | > 85% | 超过85%需要清理 |
| Inode使用率 | < 70% | > 85% | 小文件过多 |
| %util | < 60% | > 80% | 磁盘繁忙度 |
| await | < 10ms (SSD) < 20ms (HDD) | > 50ms | 平均响应时间 |
| avgqu-sz | < 2 | > 5 | 队列长度 |
| %iowait | < 5% | > 20% | CPU等待IO时间 |

## 应急处理建议

1. **磁盘空间不足：**
   - 清理日志文件：`find /var/log -name "*.log" -mtime +7 -delete`
   - 清理临时文件：`rm -rf /tmp/*`
   - 找出占用空间最多的进程并处理

2. **IO性能差：**
   - 调整IO调度器：`echo deadline > /sys/block/sda/queue/scheduler`
   - 增加队列深度（谨慎）
   - 限制IO密集型进程（ionice）

3. **硬件故障：**
   - 尽快迁移数据
   - 准备更换硬盘
   - 联系运维团队

需要我针对某个特定的问题场景提供更详细的排查步骤吗？或者创建一个完整的磁盘问题排查文档到你的项目中？

```shellscript
# 查看磁盘空间使用情况
df -h

# 查看inode使用情况（文件数量限制）
df -i

# 查找大文件（前20个）
find / -type f -size +100M -exec ls -lh {} \; 2>/dev/null | sort -k5 -hr | head -20

# 查看目录占用空间（按大小排序）
du -h --max-depth=1 / | sort -hr | head -20

# 查看已删除但未释放的文件（进程仍持有文件句柄）
lsof | grep deleted
```

```shellscript
# 查看IO等待情况
iostat -x 1 10
# 关注：%util（使用率）、await（平均等待时间）、r/s、w/s

# 实时监控IO情况
iotop -oP
# 查看哪些进程在进行IO操作

# 查看CPU iowait
top
# 关注 %wa（iowait）指标

# 更详细的IO统计
vmstat 1 10
# 关注 bi（块设备读入）、bo（块设备写出）、wa（iowait）

# 查看磁盘IO队列深度
cat /proc/diskstats
# 或使用
sar -d 1 10
```

```shellscript
# 测试磁盘顺序写性能
dd if=/dev/zero of=/tmp/test bs=1M count=1024 oflag=direct
# 测试磁盘顺序读性能
dd if=/tmp/test of=/dev/null bs=1M iflag=direct

# 使用fio进行更专业的测试
fio --name=random-write --ioengine=libaio --rw=randwrite --bs=4k \
    --numjobs=4 --size=1G --runtime=60 --time_based --end_fsync=1

# 测试随机读
fio --name=random-read --ioengine=libaio --rw=randread --bs=4k \
    --numjobs=4 --size=1G --runtime=60 --time_based
```

```shellscript
# 查看内核日志中的磁盘错误
dmesg | grep -i "error\|fail\|warn" | grep -i "sd\|disk\|ata"
tail -f /var/log/kern.log | grep -i "disk\|sd"

# 查看SMART信息（硬盘自检）
smartctl -a /dev/sda
smartctl -H /dev/sda  # 快速健康检查

# 查看磁盘错误统计
cat /sys/block/sda/stat
# 或
iostat -x | grep -i error
```

```shellscript
# 查看文件系统挂载状态
mount | grep -v tmpfs
cat /proc/mounts

# 检查是否有只读挂载
mount | grep "ro,"

# 查看文件系统类型和状态
lsblk -f

# 文件系统错误检查（需要卸载或只读模式）
fsck -n /dev/sda1  # -n 表示不修复，只检查

# 查看XFS文件系统错误（如果使用XFS）
xfs_repair -n /dev/sda1
```

```shellscript
# 查看设备队列深度
cat /sys/block/sda/queue/nr_requests

# 查看IO调度器
cat /sys/block/sda/queue/scheduler

# 监控IO队列长度
iostat -x 1
# 关注 avgqu-sz（平均队列长度）
```

```shellscript
# 查看NFS挂载状态
nfsstat -m
showmount -e <nfs_server>

# 检查NFS IO统计
nfsstat -c

# 查看挂载点是否卡死
df -h  # 如果卡住，说明有问题
timeout 5 ls /nfs/mount/point

# 检查网络延迟
ping <storage_server>
```

```shellscript
#!/bin/bash
# 磁盘问题快速诊断脚本

echo "=== 磁盘空间检查 ==="
df -h
echo ""

echo "=== Inode使用情况 ==="
df -i
echo ""

echo "=== 磁盘IO统计（5次采样） ==="
iostat -x 1 5
echo ""

echo "=== 最耗IO的进程 ==="
iotop -b -n 1 -o | head -20
echo ""

echo "=== CPU iowait 情况 ==="
vmstat 1 5
echo ""

echo "=== 磁盘错误日志 ==="
dmesg | grep -i "error\|fail" | grep -i "disk\|sd\|ata" | tail -20
echo ""

echo "=== 只读文件系统检查 ==="
mount | grep "ro,"
echo ""

echo "=== 已删除但未释放的文件 ==="
lsof 2>/dev/null | grep deleted | awk '{sum+=$8} END {print "Total:", sum/1024/1024, "MB"}'
echo ""
```

```go
// 监控磁盘IO的代码示例
package main

import (
    "fmt"
    "os"
    "syscall"
    "time"
)

func CheckDiskSpace(path string) error {
    var stat syscall.Statfs_t
    if err := syscall.Statfs(path, &stat); err != nil {
        return err
    }
    
    // 可用空间
    available := stat.Bavail * uint64(stat.Bsize)
    // 总空间
    total := stat.Blocks * uint64(stat.Bsize)
    // 使用率
    usedPercent := float64(total-available) / float64(total) * 100
    
    fmt.Printf("磁盘空间: 总共 %d GB, 可用 %d GB, 使用率 %.2f%%\n",
        total/1024/1024/1024, available/1024/1024/1024, usedPercent)
    
    if usedPercent > 90 {
        return fmt.Errorf("磁盘空间不足，使用率 %.2f%%", usedPercent)
    }
    return nil
}

// 监控文件写入性能
func MonitorDiskWrite(path string) {
    start := time.Now()
    
    f, err := os.Create(path)
    if err != nil {
        fmt.Printf("创建文件失败: %v\n", err)
        return
    }
    defer os.Remove(path)
    defer f.Close()
    
    data := make([]byte, 1024*1024) // 1MB
    n, err := f.Write(data)
    if err != nil {
        fmt.Printf("写入失败: %v\n", err)
        return
    }
    
    if err := f.Sync(); err != nil {
        fmt.Printf("同步失败: %v\n", err)
        return
    }
    
    duration := time.Since(start)
    throughput := float64(n) / duration.Seconds() / 1024 / 1024
    
    fmt.Printf("写入 %d bytes, 耗时 %v, 吞吐量 %.2f MB/s\n", n, duration, throughput)
    
    if duration > 100*time.Millisecond {
        fmt.Printf("警告: 磁盘写入慢，耗时 %v\n", duration)
    }
}
```

