package util

import (
	"sync"
)

type WaitGroupWrapper struct {
	sync.WaitGroup
}

// Wrap 包装。 会阻塞一下 ，在 cb 函数执行完之后，才会退出
func (w *WaitGroupWrapper) Wrap(cb func()) {
	w.Add(1)
	go func() {
		cb()
		w.Done()
	}()
}
