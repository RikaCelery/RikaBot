package utils

import (
	"fmt"
	"sync"

	"github.com/FloatTech/floatbox/math"
)

// Reverse 反转slice
func Reverse[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// Contains 判断slice中是否包含某元素
func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// Mapped 是一个泛型别名，代表一个存储指向 MappedItem 结构的切片。
// 它用于保存并行映射操作的结果。
type Mapped[R any] []*MappedItem[R]

// MappedItem 是一个泛型结构体，用于存储映射操作的单个结果。
// 它包含一个结果值和一个错误信息。
type MappedItem[R any] struct {
	Ret *R
	Err error
}

// ParallelMap 是一个并行映射函数，它接受一个元素类型为 T 的切片、并发数和一个转换函数作为参数。
// 它使用并发协程来并行地将转换函数应用于切片中的每个元素，并返回一个 Mapped 类型的结果。
// 这个函数解决了需要并行处理大量数据的需求，同时限制了并发数量以避免过度消耗资源。
func ParallelMap[T any, R any](list []T, concurrency int, transformer func(v T) (R, error)) Mapped[R] {
	// 创建一个通道，用于接收每个元素的映射结果。
	var res = make(chan chan MappedItem[R], math.Min(concurrency, len(list)))

	// 初始化结果切片，用于存储最终的映射结果。
	var ret = make(Mapped[R], len(list))

	// 创建一个等待组，用于等待所有协程完成。
	var wg = &sync.WaitGroup{}

	// 创建一个互斥锁，用于保护对 ret 切片的写操作。
	var mu sync.Mutex

	// 启动一个协程，用于从通道中接收结果并填充到结果切片中。
	go func() {
		defer wg.Done()
		for i := 0; i < len(list); i++ {
			s := <-<-res
			mu.Lock()
			ret[i] = &s
			mu.Unlock()
		}
	}()

	// 创建一个信号量，用于控制并发数。
	sem := make(chan struct{}, concurrency)

	// 遍历输入列表，为每个元素启动一个协程进行映射操作。
	for _, v := range list {
		// 创建一个通道，用于存储当前元素的映射结果。
		var c = make(chan MappedItem[R], 1)
		res <- c

		wg.Add(1)
		sem <- struct{}{} // 获取信号量

		// 启动一个协程，用于执行映射操作并将结果发送到通道中。
		go func(v T) {
			defer func() {
				wg.Done()
				<-sem // 释放信号量
			}()
			defer func() {
				if r := recover(); r != nil {
					c <- MappedItem[R]{Ret: nil, Err: fmt.Errorf("panic in transformer: %v", r)} //nolint:forbidigo
				}
			}()
			r, err := transformer(v)
			c <- MappedItem[R]{&r, err}
		}(v)
	}

	// 等待所有任务提交后关闭结果通道。
	close(res)

	// 等待所有协程完成。
	wg.Wait()

	// 返回最终的映射结果。
	return ret
}
