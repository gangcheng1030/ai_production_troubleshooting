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

// 好的实现：正确处理 Request.Body
func goodHandler(ctx *fasthttp.RequestCtx) {
	// 获取 body 大小
	size := ctx.Request.Header.ContentLength()

	// 重要：使用 StreamRequestBody 时，必须流式读取并丢弃请求体
	// 直接使用 RequestBodyStream() 而不是检查 IsBodyStream()
	// 因为 IsBodyStream() 可能返回 false，但仍然需要从流中读取数据
	n, err := io.Copy(io.Discard, ctx.RequestBodyStream())
	if err != nil {
		log.Printf("ERROR: Failed to read body stream: %v", err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}
	log.Printf("DEBUG: Discarded %d bytes from stream (Content-Length: %d)", n, size)

	ctx.Response.Header.SetContentType("application/json")
	ctx.SetStatusCode(fasthttp.StatusOK)
	fmt.Fprintf(ctx, `{"status":"success","size":%d,"mode":"good"}`, size)
}

// 强制 GC 端点（仅用于测试）
func gcHandler(ctx *fasthttp.RequestCtx) {
	// 使用 StreamRequestBody 时必须读取请求体（即使可能为空）
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
	// 使用 StreamRequestBody 时必须读取请求体（即使可能为空）
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
			goodHandler(ctx)
		case "/gc":
			gcHandler(ctx)
		case "/exit":
			exitHandler(ctx)
		default:
			// 404 处理器也必须读取请求体
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
	log.Printf("GC endpoint: POST http://localhost%s/gc", *addr)
	log.Printf("Exit endpoint: POST http://localhost%s/exit", *addr)

	if err := server.ListenAndServe(*addr); err != nil {
		log.Fatalf("Error in ListenAndServe: %v", err)
	}
}
