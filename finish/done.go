package finish

import "sync"

type Done struct {
	once   *sync.Once
	lock   *sync.RWMutex
	isDone bool
}

func NewDone() Done {
	return Done{&sync.Once{}, &sync.RWMutex{}, false}
}

func (d *Done) Finish() {
	d.once.Do(func() {
		d.lock.Lock()
		defer d.lock.Unlock()
		d.isDone = true
	})
}

func (d Done) IsDone() bool {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.isDone
}
