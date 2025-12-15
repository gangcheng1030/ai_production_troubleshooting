package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"time"
)

// Case 3: 频繁的字符串拼接导致CPU和内存问题

// BadExample: 使用+操作符拼接字符串
func badStringConcat(n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += fmt.Sprintf("item_%d,", i)
	}
	return result
}

// 持续执行字符串拼接操作（用于CPU profiling）
func continuousBadConcat(n int) {
	for {
		_ = badStringConcat(n)
	}
}

func main() {
	// 启动pprof HTTP服务器
	fmt.Println("Starting pprof server on http://localhost:6060")
	fmt.Println("Access the following endpoints:")
	fmt.Println("  - http://localhost:6060/debug/pprof/")
	fmt.Println("  - http://localhost:6060/debug/pprof/profile?seconds=30")
	fmt.Println("  - http://localhost:6060/debug/pprof/heap")
	fmt.Println("")

	go func() {
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			fmt.Printf("pprof server error: %v\n", err)
		}
	}()

	// 等待pprof服务器启动
	time.Sleep(1 * time.Second)

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("Starting continuous workload for CPU profiling...")
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println(strings.Repeat("=", 70) + "\n")

	// 场景1：使用坏的方法（+ 操作符）- 少量循环以避免太慢
	go func() {
		fmt.Println("Goroutine 1: Running bad string concatenation (+ operator)...")
		continuousBadConcat(500) // 减少到500避免太慢
	}()

	fmt.Println("\nTip: Use the following commands to capture and analyze CPU profile:")
	fmt.Println("  1. Capture 30s CPU profile:")
	fmt.Println("     curl http://localhost:6060/debug/pprof/profile?seconds=30 -o cpu.prof")
	fmt.Println("")
	fmt.Println("  2. Analyze with pprof:")
	fmt.Println("     go tool pprof cpu.prof")
	fmt.Println("")
	fmt.Println("  3. View in browser:")
	fmt.Println("     go tool pprof -http=:8080 cpu.prof")
	fmt.Println("")

	http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/exit" {
			os.Exit(0)
		}
		// 其它请求返回404
		http.NotFound(w, r)
	}))
}
