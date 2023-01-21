package service

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

const (
	H264_START_CODE_SIZE = 4 // 4 or 3 bytes
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

// 读取 h264
func readByte(br *bufio.Reader) (byte, bool, error) {
	end := false
	b, err := br.ReadByte()
	if err != nil {
		if errors.Is(err, io.EOF) {
			end = true
		}
	}
	return b, end, err
}

func StartCode(buf []byte) bool {
	// if len(buf) >= 3 && buf[0] == 0 && buf[1] == 0 && buf[2] == 1 {
	// 	return true
	// }
	if len(buf) >= 4 && buf[0] == 0 && buf[1] == 0 && buf[2] == 0 && buf[3] == 1 {
		return true
	}
	return false
}

// 根据 StartCode 分割
func ReadH264Worker(rd io.Reader, out chan<- []byte, errChan chan<- error) {
	br := bufio.NewReader(rd)

	data := make([]byte, 0, 128)
	buf := make([]byte, 0, 4)
	tmpBuf := make([]byte, 0, 4)

	for {
		b, end, err := readByte(br)
		if end {
			data = append(data, buf...)
			break
		}
		if err != nil {
			errChan <- fmt.Errorf("read h264 failed: %v", err)
			return
		}

		buf = append(buf, b)
		if len(buf) == H264_START_CODE_SIZE {
			if StartCode(buf) {
				pkg := make([]byte, len(data))
				copy(pkg, data)
				out <- pkg

				data = data[:0]
				buf = buf[:0]
			} else {
				last := len(buf) - 1
				// buf 中的第一个不可能用于 start code 了
				for last > 0 && buf[last] == 0 {
					last--
				}
				data = append(data, buf[:last+1]...)

				if last != len(buf)-1 {
					tmpBuf = append(tmpBuf, buf[last+1:]...)
				}

				buf = buf[:0]
				buf = append(buf, tmpBuf...)
				tmpBuf = tmpBuf[:0]
			}
		}
	}

	// 最后一轮读取到的数据
	pkg := make([]byte, len(data))
	copy(pkg, data)
	out <- pkg

	close(out)
	close(errChan)
}
