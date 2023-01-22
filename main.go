package main

import (
	"fmt"
	"goRTSP/service"
	"net"
)

const (
	IP   = "127.0.0.1"
	PORT = "8554"
)

// 启动服务后，使用
// ffmpeg -i rtsp://127.0.0.1:8554
// 或 ffplay -i rtsp://127.0.0.1:8554
// 连接服务器
func main() {
	// 建立 tcp 服务
	addr := fmt.Sprintf("%s:%s", IP, PORT)
	listen, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("listen failed, err:%v\n", err)
		return
	}
	fmt.Printf("start RTSP server [%s]...\n", addr)

	count := 0
	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Printf("accept failed, err:%v\n", err)
			continue
		}

		// 启动一个单独的 goroutine 去处理连接
		// go process(conn, count)
		worker := CreateWorker(conn, count)
		go worker.Process()
		count += 1
	}
}

func CreateWorker(conn net.Conn, id int) *service.Worker {
	worker := service.WorkerCached.Get()
	if worker != nil {
		worker.ReFresh(conn, id)
	} else {
		worker = service.NewWorker(conn, id)
	}
	fmt.Printf("Worker Cached Len: %d\n", service.WorkerCached.Len())
	return worker
}
