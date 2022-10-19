package wait

import (
	"sync"
	"time"
)

// Wait 类似于WaitGroup，它可以等待超时
type Wait struct {
	wg sync.WaitGroup
}

// Add 将可能为负数的 delta 添加到 WaitGroup 计数器
func (w *Wait) Add(delta int) {
	w.wg.Add(delta)
}

// Done 将 WaitGroup 计数器减一
func (w *Wait) Done() {
	w.wg.Done()
}

// Wait 阻塞，直到 WaitGroup 计数器为零
func (w *Wait) Wait() {
	w.wg.Wait()
}

// WaitWithTimeout 阻塞直到 WaitGroup 计数器为零或超时
// 如果超时返回true
func (w *Wait) WaitWithTimeout(timeout time.Duration) bool {
	c := make(chan bool, 1)
	go func() {
		defer close(c)
		w.wg.Wait()
		c <- true
	}()
	select {
	// 正常完成
	case <-c:
		return false
	// 超时
	case <-time.After(timeout):
		return true
	}
}
