package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"time"

	pb "github.com/gangcheng1030/ai_production_troubleshooting/goroutine_analyze/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// 问题代码：每次请求都创建新的连接，且不关闭
func makeRequestBad() error {
	// ❌ 每次都创建新连接
	conn, err := grpc.Dial(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to dial: %v", err)
	}

	// ❌ 没有 defer conn.Close()，连接泄漏！

	client := pb.NewHelloServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := client.SayHello(ctx, &pb.HelloRequest{Name: "World"})
	if err != nil {
		return fmt.Errorf("failed to call SayHello: %v", err)
	}

	log.Printf("Response: %s", resp.Message)
	return nil
}

func main() {
	log.Println("=== Bad Client Demo: 不复用连接，不关闭连接 ===")
	log.Println("问题：每次请求都 new dial，没有复用连接，也没有释放连接")
	log.Println("观察：goroutine 数量会持续上涨")
	log.Println()

	initialGoroutines := runtime.NumGoroutine()
	log.Printf("初始 goroutine 数量: %d", initialGoroutines)
	log.Println()

	// 模拟持续请求
	ticker := time.NewTicker(1 * time.Millisecond)
	defer ticker.Stop()

	// 监控 goroutine 数量
	go func() {
		monitorTicker := time.NewTicker(2 * time.Second)
		defer monitorTicker.Stop()
		for range monitorTicker.C {
			current := runtime.NumGoroutine()
			increase := current - initialGoroutines
			log.Printf("📊 当前 goroutine: %d (增加了 %d)", current, increase)
		}
	}()

	requestCount := 0
	for range ticker.C {
		requestCount++
		if err := makeRequestBad(); err != nil {
			log.Printf("❌ Request #%d failed: %v", requestCount, err)
			continue
		}

		if requestCount%50 == 0 {
			current := runtime.NumGoroutine()
			log.Printf("⚠️  已发送 %d 个请求，goroutine: %d", requestCount, current)
		}

		// 发送 500 个请求后停止
		if requestCount >= 500 {
			log.Println()
			log.Println("=== 测试完成 ===")
			finalGoroutines := runtime.NumGoroutine()
			log.Printf("最终 goroutine 数量: %d", finalGoroutines)
			log.Printf("泄漏的 goroutine: %d", finalGoroutines-initialGoroutines)
			log.Println()
			log.Println("💡 问题原因：")
			log.Println("   1. 每次请求都创建新的 gRPC 连接（grpc.Dial）")
			log.Println("   2. 没有调用 conn.Close() 释放连接")
			log.Println("   3. 导致底层的网络连接和 goroutine 无法释放")
			log.Println()
			log.Println("🔧 解决方案：")
			log.Println("   1. 复用连接：在应用启动时创建一次连接，多次请求复用")
			log.Println("   2. 正确关闭：如果必须创建新连接，使用 defer conn.Close()")
			log.Println("   3. 使用连接池：对于高并发场景，可以使用连接池管理")
			log.Println()
			log.Println("🔍 使用 pprof 查看详细信息：")
			log.Println("   curl http://localhost:50052/debug/pprof/goroutine?debug=2")

			// 等待一段时间以便观察
			log.Println()
			log.Println("等待 10 秒以便使用 pprof 查看 goroutine 信息...")
			time.Sleep(10 * time.Second)
			return
		}
	}
}
