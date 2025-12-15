package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	addr      = flag.String("addr", ":8080", "HTTP server address")
	pprofAddr = flag.String("pprof", ":6060", "pprof HTTP server address")
)

// 坏的实现：直接访问 Request.Body 可能导致内存问题
func badHandler(ctx *fasthttp.RequestCtx) {
	body := ctx.Request.Body()

	// 模拟处理
	size := len(body)

	// 故意不释放，模拟可能的内存泄漏场景
	// 在实际场景中，可能是将 body 传递给其他函数或存储引用

	ctx.Response.Header.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	fmt.Fprintf(ctx, `{"status":"success","size":%d,"mode":"bad"}`, size)
}

// 强制 GC 端点（仅用于测试）
func gcHandler(ctx *fasthttp.RequestCtx) {
	if ctx.RequestBodyStream() != nil {
		io.Copy(io.Discard, ctx.RequestBodyStream())
	}
	runtime.GC()
	ctx.Response.Header.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	fmt.Fprintf(ctx, `{"status":"gc triggered"}`)
}

// 退出
func exitHandler(ctx *fasthttp.RequestCtx) {
	if ctx.RequestBodyStream() != nil {
		io.Copy(io.Discard, ctx.RequestBodyStream())
	}
	ctx.Response.Header.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	fmt.Fprintf(ctx, `{"status":"exit"}`)
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

func main() {
	flag.Parse()

	// 启动 pprof 服务器
	go func() {
		log.Printf("Starting pprof server on http://localhost%s", *pprofAddr)
		log.Printf("Heap profile: http://localhost%s/debug/pprof/heap", *pprofAddr)
		log.Printf("Allocs profile: http://localhost%s/debug/pprof/allocs", *pprofAddr)
		if err := http.ListenAndServe(*pprofAddr, nil); err != nil {
			log.Fatalf("pprof server failed: %v", err)
		}
	}()

	// 配置请求处理器
	requestHandler := func(ctx *fasthttp.RequestCtx) {
		path := string(ctx.Path())

		switch path {
		case "/upload":
			badHandler(ctx)
		case "/gc":
			gcHandler(ctx)
		case "/exit":
			exitHandler(ctx)
		default:
			if ctx.RequestBodyStream() != nil {
				io.Copy(io.Discard, ctx.RequestBodyStream())
			}
			ctx.Error("Not Found", fasthttp.StatusNotFound)
		}
	}

	// 创建 fasthttp 服务器
	server := &fasthttp.Server{
		Handler: requestHandler,
		Name:    "FastHTTP-Memory-Test",
		// 配置合理的限制
		MaxRequestBodySize: 100 * 1024 * 1024, // 100MB
		ReadTimeout:        30 * time.Second,
		WriteTimeout:       30 * time.Second,
		StreamRequestBody:  true,
	}

	log.Printf("Starting fasthttp server on http://localhost%s", *addr)
	log.Printf("Upload endpoint: POST http://localhost%s/upload", *addr)
	log.Printf("Exit endpoint: POST http://localhost%s/exit", *addr)
	log.Printf("GC endpoint: POST http://localhost%s/gc", *addr)

	if err := server.ListenAndServe(*addr); err != nil {
		log.Fatalf("Error in ListenAndServe: %v", err)
	}
}
