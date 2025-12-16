package main

import (
	"context"
	"fmt"
	"time"
)

// MomentCount 模拟数据库返回的结构
type MomentCount struct {
	MomentUserId string
	Total        int
}

// 模拟graceful.Go的行为
func Go(f func()) {
	go f()
}

// 模拟原始代码的问题：在goroutine中访问外部函数的局部变量
func GetFilterMomentCounterByUserIDs(ctx context.Context, userIDs []string) ([]MomentCount, error) {
	// 模拟从数据库查询数据
	var dbmcs []MomentCount

	notExistUserIDs := []string{"user1", "user2", "user3"}
	for i, userID := range userIDs {
		dbmcs = append(dbmcs, MomentCount{
			MomentUserId: userID,
			Total:        i * 10,
		})
	}

	// 问题代码：在goroutine中直接引用局部变量dbmcs和notExistUserIDs
	// 这些变量在函数返回后可能被回收，导致goroutine访问无效内存
	Go(func() {
		kvMap := make(map[string]interface{}, len(dbmcs))

		// 访问外部函数的局部变量notExistUserIDs
		for _, v := range notExistUserIDs {
			kvMap[GetUserFilterMomentCountKey(v)] = 0 // 初始化为0
		}

		// 访问外部函数的局部变量dbmcs
		// 这里可能访问已释放的栈内存
		for _, v := range dbmcs {
			kvMap[GetUserFilterMomentCountKey(v.MomentUserId)] = v.Total
		}

		fmt.Printf("Goroutine processed %d items\n", len(kvMap))
	})

	for i, userID := range userIDs {
		dbmcs = append(dbmcs, MomentCount{
			MomentUserId: userID,
			Total:        i * 10,
		})
	}
	// 函数立即返回，栈帧可能被回收
	return dbmcs, nil
}

func GetUserFilterMomentCountKey(userID string) string {
	return fmt.Sprintf("key_%s", userID)
}

func main() {
	fmt.Println("=== 演示问题代码 ===")
	fmt.Println("问题：在goroutine中访问外部函数的局部变量")
	fmt.Println()

	ctx := context.Background()
	userIDs := []string{}
	for i := 0; i < 1000; i++ {
		userIDs = append(userIDs, fmt.Sprintf("user_%d", i))
	}

	// 问题版本
	fmt.Println("1. 执行有问题的版本...")
	for i := 0; i < 100000; i++ {
		go func() {
			GetFilterMomentCounterByUserIDs(ctx, userIDs)
		}()
	}

	// 等待goroutine执行
	time.Sleep(10 * time.Second)
	fmt.Println()

	fmt.Println()
	fmt.Println("=== 测试完成 ===")
}
