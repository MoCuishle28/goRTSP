package service

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchIPAndPort(t *testing.T) {
	cases := []struct {
		addr string
		ip   string
		port int
		err  error
	}{
		{
			"127.0.0.1:42",
			"127.0.0.1",
			42,
			nil,
		},
		{
			"127.0.0.1::42",
			"",
			-1,
			fmt.Errorf("error"),
		},
	}

	for i, ca := range cases {
		ip, port, err := FetchIPAndPort(ca.addr)

		t.Logf("[%d] %+v", i, ca)
		if ca.err == nil {
			require.NoError(t, err)
			require.Equal(t, ca.ip, ip)
			require.Equal(t, ca.port, port)
		} else {
			require.Error(t, err)
		}
	}
}

type mockFileReader struct {
	data []byte
}

var currIdx int

func (rd mockFileReader) Read(p []byte) (n int, err error) {
	if currIdx >= len(rd.data) {
		return 0, io.EOF
	}

	i := 0
	for ; i < len(p); i, currIdx = i+1, currIdx+1 {
		if currIdx == len(rd.data) {
			break
		}
		p[i] = rd.data[currIdx]
	}
	return i, nil
}

func NewMockFileReader(data []byte) io.Reader {
	currIdx = 0
	return mockFileReader{data: data}
}

func TestReadH264Worker(t *testing.T) {
	cases := []struct {
		src    []byte
		target []byte
	}{
		{
			[]byte{0, 0, 0, 1, 6, 5, 90, 179, 225, 99, 48, 140, 60, 158, 79, 194, 0, 0, 0, 1, 15, 15, 15, 15},
			[]byte{6, 5, 90, 179, 225, 99, 48, 140, 60, 158, 79, 194, 15, 15, 15, 15},
		},
		{
			[]byte{0, 0, 0, 1, 6, 5, 90, 179, 225, 99, 48, 140, 60, 158, 79, 194, 0, 0, 0, 1, 15, 15, 15, 15, 15},
			[]byte{6, 5, 90, 179, 225, 99, 48, 140, 60, 158, 79, 194, 15, 15, 15, 15, 15},
		},
		{
			[]byte{0, 0, 0, 1, 6, 5, 90, 179, 0, 0, 2, 0, 0, 15, 0, 0, 0, 1, 52, 64},
			[]byte{6, 5, 90, 179, 0, 0, 2, 0, 0, 15, 52, 64},
		},
	}

	for i, ca := range cases {
		t.Logf("------------ case %d ------------", i)
		out := make(chan []byte)
		errChan := make(chan error)
		rd := NewMockFileReader(ca.src)
		go ReadH264Worker(rd, out, errChan)

		data := make([]byte, 0, len(ca.target))
		readTime := 0
	READ:
		for {
			readTime++
			select {
			case pkg := <-out:
				t.Logf("[%d] pkg: %v", readTime, pkg)
				data = append(data, pkg...)
			case err := <-errChan:
				t.Logf("err: %v", err)
				break READ
			}
		}

		require.Equal(t, ca.target, data)
	}
}

func TestReadTrueH264(t *testing.T) {
	out := make(chan []byte, 5)
	errChan := make(chan error)

	f, err := os.Open("../videos/test.h264")
	if err != nil {
		t.Error(err)
		return
	}
	defer f.Close()
	go ReadH264Worker(f, out, errChan)

	wg := sync.WaitGroup{}
	readTime := 0
	writeFileChan := make(chan []byte, 5)
	wg.Add(1)
	go func() {
		t.Log("start read pipline")
		defer wg.Done()
		defer close(writeFileChan)
	READ:
		for {
			if readTime == 4 { // for debug
				break
			}
			readTime++
			select {
			case pkg := <-out:
				t.Logf("[%d] pkg: %v", readTime, pkg)
				writeFileChan <- pkg
			case err := <-errChan:
				t.Logf("err: %v", err)
				break READ
			}
		}
	}()

	wg.Add(1)
	go func() {
		t.Log("start write pipline")
		defer wg.Done()
		file, err := os.OpenFile("../videos/fetch-h264.bin", os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			t.Error(err)
		}
		defer file.Close()

		//写入文件时，使用带缓存的 *Writer
		write := bufio.NewWriter(file)

		for pkg := range writeFileChan {
			t.Logf("pkg: %v", pkg)
			write.Write(pkg)
		}
		//Flush将缓存的文件真正写入到文件中
		write.Flush()
	}()

	wg.Wait()
	// hexdump fetch.bin | head -n 12 > my.bin
}
