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

// 正确的做法：复用连接
type GoodClient struct {
	conn   *grpc.ClientConn
	client pb.HelloServiceClient
}

func NewGoodClient(address string) (*GoodClient, error) {
	// ✅ 只创建一次连接
	conn, err := grpc.Dial(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %v", err)
	}

	return &GoodClient{
		conn:   conn,
		client: pb.NewHelloServiceClient(conn),
	}, nil
}

func (c *GoodClient) Close() error {
	// ✅ 提供关闭方法
	return c.conn.Close()
}

func (c *GoodClient) MakeRequest() error {
	// ✅ 复用同一个连接和 client
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := c.client.SayHello(ctx, &pb.HelloRequest{Name: "World"})
	if err != nil {
		return fmt.Errorf("failed to call SayHello: %v", err)
	}

	log.Printf("Response: %s", resp.Message)
	return nil
}

func main() {
	log.Println("=== Good Client Demo: 复用连接，正确关闭 ===")
	log.Println("正确做法：创建一次连接，多次复用")
	log.Println("观察：goroutine 数量保持稳定")
	log.Println()

	initialGoroutines := runtime.NumGoroutine()
	log.Printf("初始 goroutine 数量: %d", initialGoroutines)
	log.Println()

	// ✅ 创建一次 client，复用连接
	client, err := NewGoodClient("localhost:50051")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close() // ✅ 程序结束时关闭连接

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
			log.Printf("📊 当前 goroutine: %d (变化 %+d)", current, increase)
		}
	}()

	requestCount := 0
	for range ticker.C {
		requestCount++
		if err := client.MakeRequest(); err != nil {
			log.Printf("❌ Request #%d failed: %v", requestCount, err)
			continue
		}

		if requestCount%50 == 0 {
			current := runtime.NumGoroutine()
			log.Printf("✅ 已发送 %d 个请求，goroutine: %d", requestCount, current)
		}

		// 发送 500 个请求后停止
		if requestCount >= 500 {
			log.Println()
			log.Println("=== 测试完成 ===")
			finalGoroutines := runtime.NumGoroutine()
			log.Printf("最终 goroutine 数量: %d", finalGoroutines)
			log.Printf("goroutine 变化: %+d", finalGoroutines-initialGoroutines)
			log.Println()
			log.Println("✅ 正确实践：")
			log.Println("   1. 连接复用：创建一次连接，多次请求复用")
			log.Println("   2. 正确关闭：使用 defer conn.Close() 确保资源释放")
			log.Println("   3. goroutine 数量保持稳定，没有泄漏")
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
