package finish

import "sync"

type Done struct {
	once   *sync.Once
	done   chan any
	isDone bool
}

func NewDone() Done {
	return Done{&sync.Once{}, make(chan any), false}
}

func (d *Done) Finish() {
	d.once.Do(func() {
		close(d.done)
		d.isDone = true
	})
}

func (d Done) IsDone() bool {
	return d.isDone
}
