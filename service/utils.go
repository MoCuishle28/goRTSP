package service

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

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

func ListenUDP(addrStr string) (*net.UDPAddr, *net.UDPConn, error) {
	addr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		fmt.Println("resolve udp addr failed:", err)
		return nil, nil, err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println("listen udp svr failed:", err)
		return nil, nil, err
	}

	return addr, conn, nil
}

func FetchIPAndPort(addr string) (ip string, portNum int, err error) {
	arr := strings.Split(addr, ":")
	ip, port := arr[0], arr[1]

	portNum, err = strconv.Atoi(port)
	if err != nil {
		return "", -1, err
	}
	return ip, portNum, nil
}
