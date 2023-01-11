package main

import (
	"fmt"
	"net"
	"rtsp-server/service"
)

const (
	IP   = "127.0.0.1"
	PORT = "8554"
)

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
		worker := service.NewWorker(conn, count)
		go worker.Process()
		count += 1
	}
}
