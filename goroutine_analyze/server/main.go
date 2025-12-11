package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"time"

	pb "github.com/gangcheng1030/ai_production_troubleshooting/goroutine_analyze/proto"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedHelloServiceServer
}

func (s *server) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloResponse, error) {
	return &pb.HelloResponse{
		Message: fmt.Sprintf("Hello, %s!", req.Name),
	}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterHelloServiceServer(s, &server{})

	log.Printf("Server starting on :50051...")
	log.Printf("pprof server starting on :50052")
	log.Printf("访问 http://localhost:50052/debug/pprof 查看 pprof 信息")
	log.Printf("查看 goroutine: http://localhost:50052/debug/pprof/goroutine?debug=2")
	log.Println()

	// 启动 pprof HTTP 服务器
	go func() {
		if err := http.ListenAndServe(":50052", nil); err != nil {
			log.Printf("pprof server error: %v", err)
		}
	}()

	// 启动 goroutine 监控
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			log.Printf("[Server] Current goroutines: %d", runtime.NumGoroutine())
		}
	}()

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
