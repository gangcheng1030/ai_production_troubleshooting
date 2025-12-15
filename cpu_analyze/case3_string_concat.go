package main

import (
	"bytes"
	"fmt"
	"net/http"
	_ "net/http/pprof"
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

// GoodExample1: 使用strings.Builder
func goodStringBuilder(n int) string {
	var builder strings.Builder
	builder.Grow(n * 12) // 预分配容量（估算每项约12字节）
	for i := 0; i < n; i++ {
		builder.WriteString(fmt.Sprintf("item_%d,", i))
	}
	return builder.String()
}

// GoodExample2: 使用bytes.Buffer
func goodBytesBuffer(n int) string {
	var buffer bytes.Buffer
	buffer.Grow(n * 12)
	for i := 0; i < n; i++ {
		buffer.WriteString(fmt.Sprintf("item_%d,", i))
	}
	return buffer.String()
}

// GoodExample3: 使用[]byte + append
func goodByteSlice(n int) string {
	data := make([]byte, 0, n*12)
	for i := 0; i < n; i++ {
		data = append(data, fmt.Sprintf("item_%d,", i)...)
	}
	return string(data)
}

// GoodExample4: 使用strings.Join（当有切片时）
func goodStringsJoin(n int) string {
	items := make([]string, n)
	for i := 0; i < n; i++ {
		items[i] = fmt.Sprintf("item_%d", i)
	}
	return strings.Join(items, ",")
}

// Benchmark: 比较不同方法的性能
func benchmark(name string, fn func(int) string, n int) {
	start := time.Now()
	result := fn(n)
	elapsed := time.Since(start)

	fmt.Printf("%-25s: Time=%8v, Length=%d\n", name, elapsed, len(result))
}

// MemoryAnalysis: 分析内存使用情况
func memoryAnalysis() {
	fmt.Println("\n=== Memory Analysis ===")
	fmt.Println("\nFor n=10000 iterations:")

	fmt.Println("\n1. Bad Approach (+ operator):")
	fmt.Println("   - Allocations: ~10,000 times")
	fmt.Println("   - Total bytes copied: O(n²) ≈ 50MB")
	fmt.Println("   - Each concatenation creates a new string")
	fmt.Println("   - Example: \"a\" + \"b\" creates new string \"ab\"")

	fmt.Println("\n2. Good Approach (strings.Builder):")
	fmt.Println("   - Allocations: ~10 times (grows dynamically)")
	fmt.Println("   - Total bytes copied: O(n) ≈ 120KB")
	fmt.Println("   - Reuses internal buffer")
	fmt.Println("   - With Grow(): can reduce to 1 allocation")

	fmt.Println("\n3. Good Approach (pre-allocated slice):")
	fmt.Println("   - Allocations: 1 time (if capacity is correct)")
	fmt.Println("   - Total bytes copied: O(n) ≈ 120KB")
	fmt.Println("   - Best performance when size is known")
}

// ExplainWhySlow: 解释为什么字符串拼接慢
func explainWhySlow() {
	fmt.Println("\n=== Why String Concatenation is Slow ===")

	fmt.Println("\nGo strings are immutable. Every concatenation:")
	fmt.Println("1. Allocates new memory")
	fmt.Println("2. Copies old string")
	fmt.Println("3. Appends new string")
	fmt.Println("4. Old string becomes garbage")

	fmt.Println("\nExample with 3 strings:")
	fmt.Println("  s1 = \"hello\"         // 5 bytes")
	fmt.Println("  s2 = s1 + \" world\"   // allocate 11 bytes, copy 5+6")
	fmt.Println("  s3 = s2 + \"!\"        // allocate 12 bytes, copy 11+1")
	fmt.Println("  Total copied: 5 + 6 + 11 + 1 = 23 bytes")

	fmt.Println("\nWith n iterations:")
	fmt.Println("  Total allocations: O(n)")
	fmt.Println("  Total bytes copied: O(n²)")
	fmt.Println("  Time complexity: O(n²)")
}

// RealWorldExample: 真实场景示例
func realWorldExample() {
	fmt.Println("\n=== Real World Example ===")

	// 场景1：构建SQL查询
	fmt.Println("\n1. Building SQL Query (BAD):")
	start := time.Now()
	query := "SELECT * FROM users WHERE id IN ("
	for i := 0; i < 1000; i++ {
		if i > 0 {
			query += ","
		}
		query += fmt.Sprintf("%d", i)
	}
	query += ")"
	fmt.Printf("   Time: %v, Length: %d\n", time.Since(start), len(query))

	fmt.Println("\n2. Building SQL Query (GOOD):")
	start = time.Now()
	var builder strings.Builder
	builder.Grow(1000 * 5)
	builder.WriteString("SELECT * FROM users WHERE id IN (")
	for i := 0; i < 1000; i++ {
		if i > 0 {
			builder.WriteString(",")
		}
		fmt.Fprintf(&builder, "%d", i)
	}
	builder.WriteString(")")
	query = builder.String()
	fmt.Printf("   Time: %v, Length: %d\n", time.Since(start), len(query))

	// 场景2：构建JSON字符串（注意：实际应用应使用json.Marshal）
	fmt.Println("\n3. Building JSON String (BAD):")
	start = time.Now()
	jsonStr := "{"
	for i := 0; i < 100; i++ {
		if i > 0 {
			jsonStr += ","
		}
		jsonStr += fmt.Sprintf(`"key%d":"value%d"`, i, i)
	}
	jsonStr += "}"
	fmt.Printf("   Time: %v, Length: %d\n", time.Since(start), len(jsonStr))

	fmt.Println("\n4. Building JSON String (GOOD):")
	start = time.Now()
	var jsonBuilder strings.Builder
	jsonBuilder.Grow(100 * 25)
	jsonBuilder.WriteString("{")
	for i := 0; i < 100; i++ {
		if i > 0 {
			jsonBuilder.WriteString(",")
		}
		fmt.Fprintf(&jsonBuilder, `"key%d":"value%d"`, i, i)
	}
	jsonBuilder.WriteString("}")
	jsonStr = jsonBuilder.String()
	fmt.Printf("   Time: %v, Length: %d\n", time.Since(start), len(jsonStr))
}

// BestPractices: 最佳实践
func bestPractices() {
	fmt.Println("\n=== Best Practices ===")

	fmt.Println("\n1. Use strings.Builder for string concatenation")
	fmt.Println("   var builder strings.Builder")
	fmt.Println("   builder.Grow(expectedSize) // Pre-allocate if size known")
	fmt.Println("   builder.WriteString(s)")

	fmt.Println("\n2. Use bytes.Buffer for byte operations")
	fmt.Println("   var buffer bytes.Buffer")
	fmt.Println("   buffer.Write(data)")

	fmt.Println("\n3. Use strings.Join when you have a slice")
	fmt.Println("   result := strings.Join(slice, separator)")

	fmt.Println("\n4. Use fmt.Fprintf with strings.Builder")
	fmt.Println("   fmt.Fprintf(&builder, \"format\", args...)")

	fmt.Println("\n5. Pre-allocate capacity when size is known")
	fmt.Println("   builder.Grow(n * averageItemSize)")

	fmt.Println("\n6. Avoid string concatenation in loops")
	fmt.Println("   DON'T: for i := 0; i < n; i++ { s += x }")
	fmt.Println("   DO:    use strings.Builder")
}

// 持续执行字符串拼接操作（用于CPU profiling）
func continuousBadConcat(n int) {
	for {
		_ = badStringConcat(n)
	}
}

func continuousGoodConcat(n int) {
	for {
		_ = goodStringBuilder(n)
	}
}

func runCase3() {
	fmt.Println("=== Case 3: String Concatenation ===\n")

	// 小规模测试（快速）
	fmt.Println("Benchmarking with n=1000:")
	benchmark("Bad: + operator", badStringConcat, 1000)
	benchmark("Good: strings.Builder", goodStringBuilder, 1000)
	benchmark("Good: bytes.Buffer", goodBytesBuffer, 1000)
	benchmark("Good: []byte + append", goodByteSlice, 1000)
	benchmark("Good: strings.Join", goodStringsJoin, 1000)

	// 中等规模测试
	fmt.Println("\nBenchmarking with n=5000:")
	benchmark("Bad: + operator", badStringConcat, 5000)
	benchmark("Good: strings.Builder", goodStringBuilder, 5000)

	// 大规模测试（注意：badStringConcat会很慢）
	fmt.Println("\nBenchmarking with n=10000:")
	fmt.Println("Skipping bad approach (would take too long)")
	benchmark("Good: strings.Builder", goodStringBuilder, 10000)
	benchmark("Good: strings.Join", goodStringsJoin, 10000)

	// 内存分析
	memoryAnalysis()

	// 解释原因
	explainWhySlow()

	// 真实场景示例
	realWorldExample()

	// 最佳实践
	bestPractices()
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

	// 运行一次基准测试
	runCase3()

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("Starting continuous workload for CPU profiling...")
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println(strings.Repeat("=", 70) + "\n")

	// 启动多个goroutine执行不同的字符串拼接操作
	// 这样可以在CPU profile中看到性能差异

	// 场景1：使用坏的方法（+ 操作符）- 少量循环以避免太慢
	go func() {
		fmt.Println("Goroutine 1: Running bad string concatenation (+ operator)...")
		continuousBadConcat(500) // 减少到500避免太慢
	}()

	// 场景2：使用好的方法（strings.Builder）
	go func() {
		fmt.Println("Goroutine 2: Running good string concatenation (strings.Builder)...")
		continuousGoodConcat(5000)
	}()

	// 场景3：使用好的方法（strings.Builder）- 更多数据
	go func() {
		fmt.Println("Goroutine 3: Running good string concatenation with more data...")
		continuousGoodConcat(10000)
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

	// 保持主程序运行
	select {}
}
