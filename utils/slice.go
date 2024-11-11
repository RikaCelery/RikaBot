package utils

import (
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

// Mapped Map的结果
type Mapped[R any] []*MappedItem[R]

// MappedItem 用于存储一次映射操作的结果。
type MappedItem[R any] struct {
	Ret *R
	Err error
}

// ParallelMap 并行地执行一个函数，返回一个结果列表。
func ParallelMap[T any, R any](list []T, concurrency int, transformer func(v T) (R, error)) Mapped[R] {

	var res = make(chan chan MappedItem[R], math.Min(concurrency, len(list)))

	var ret = make(Mapped[R], len(list))
	var wg = &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < len(list); i++ {
			s := <-<-res
			ret[i] = &s
		}
	}()
	for _, v := range list {
		var c = make(chan MappedItem[R], 1)
		res <- c
		wg.Add(1)
		go func(v T) {
			defer wg.Done()
			r, err := transformer(v)
			c <- MappedItem[R]{&r, err}
		}(v)
	}
	wg.Wait()
	close(res)
	return ret
}
