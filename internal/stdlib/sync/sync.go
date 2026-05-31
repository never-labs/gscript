package sync

import (
	"fmt"
	stdsync "sync"
)

type TaskErrors struct {
	mu       stdsync.Mutex
	firstErr error
	errCount int
}

func AddWaitGroup(wg *stdsync.WaitGroup, delta int) (err error) {
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("sync.waitgroup: invalid counter state")
		}
	}()
	wg.Add(delta)
	return nil
}

func (e *TaskErrors) Record(err error) bool {
	if err == nil {
		return false
	}
	first := false
	e.mu.Lock()
	if e.firstErr == nil {
		e.firstErr = err
		first = true
	}
	e.errCount++
	e.mu.Unlock()
	return first
}

func (e *TaskErrors) Error() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.firstErr
}

func (e *TaskErrors) Count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.errCount
}
