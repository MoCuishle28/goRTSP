package service

type WorkerCache struct {
	queue   []*Worker
	currIdx int
	cap     int
}

func NewWorkerCache(cap int) *WorkerCache {
	wc := &WorkerCache{queue: make([]*Worker, 0, cap), currIdx: 0, cap: cap}
	wc.init()
	return wc
}

func (wc *WorkerCache) init() {
	for i := 0; i < wc.cap; i++ {
		wc.queue = append(wc.queue, &Worker{conn: nil, id: -1, clientAddr: ""})
	}
	wc.currIdx = wc.cap - 1
}

func (wc *WorkerCache) Get() *Worker {
	var worker *Worker = nil
	if wc.currIdx >= 0 {
		worker = wc.queue[wc.currIdx]
		wc.currIdx -= 1
	}
	return worker
}

func (wc *WorkerCache) Put(w *Worker) {
	if wc.currIdx >= wc.cap-1 {
		w = nil
		return
	}
	wc.queue = append(wc.queue, w)
	wc.currIdx += 1
}

func (wc *WorkerCache) Len() int {
	return wc.currIdx + 1
}
