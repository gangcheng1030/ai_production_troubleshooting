package main

import (
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	mathrand "math/rand"
	"net/http"
	"sync"
	"time"
)

var (
	serverURL = flag.String("url", "http://localhost:8080/upload", "Server upload URL")
)

// 生成指定大小的随机数据
func generateRandomData(sizeMB int) []byte {
	size := sizeMB * 1024 * 1024
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		log.Fatalf("Failed to generate random data: %v", err)
	}
	return data
}

// 发送单个上传请求
func uploadFile(url string, data []byte, id int) error {
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	duration := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	log.Printf("Request #%d completed in %v, response: %s", id, duration, string(body))
	return nil
}

func main() {
	flag.Parse()

	log.Printf("Starting upload test:")
	log.Printf("  Server URL: %s", *serverURL)

	// 使用 worker pool 模式发送请求
	var wg sync.WaitGroup
	requestChan := make(chan int, 5000)

	// 启动 worker goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for requestID := range requestChan {
				data := generateRandomData(mathrand.Intn(16) + 1)
				if err := uploadFile(*serverURL, data, requestID); err != nil {
					log.Printf("Worker %d: Request #%d failed: %v", workerID, requestID, err)
				}
				time.Sleep(200 * time.Millisecond)
			}
		}(i)
	}

	// 发送请求到 channel
	for i := 1; i <= 500; i++ {
		requestChan <- i
	}
	close(requestChan)

	// 等待所有请求完成
	wg.Wait()

	log.Printf("\n=== Test Completed ===")

	// 等待一下让服务器处理完成
	log.Printf("\nWaiting 5 seconds for server to process...")
	time.Sleep(5 * time.Second)

	// 触发 GC
	log.Printf("\nTriggering server GC...")
	gcURL := fmt.Sprintf("http://%s/gc", (*serverURL)[7:len(*serverURL)-7])
	if resp, err := http.Post(gcURL, "application/json", nil); err == nil {
		resp.Body.Close()
		log.Printf("GC triggered successfully")
	}

	// 等待 GC 完成
	time.Sleep(2 * time.Second)
}
