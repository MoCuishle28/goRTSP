package service

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	BUF_SIZE = 10000
	SEP      = "\n"

	OPTIONS  = "OPTIONS"
	DESCRIBE = "DESCRIBE"
	SETUP    = "SETUP"
	PLAY     = "PLAY"

	CSEQ      = "CSeq"
	TRANSPORT = "Transport:"

	SERVER_RTP_PORT  = 55532
	SERVER_RTCP_PORT = 55533
)

type parseResult struct {
	Method         string
	URL            string
	Version        string
	CSeq           int
	ClientRtpPort  int
	ClientRtcpPort int
}

func (pr *parseResult) String() string {
	return fmt.Sprintf("Method: %s\nURL: %s\nVersion: %s\nCSeq: %d\nClientRtpPort: %d\nClientRtcpPort: %d\n",
		pr.Method, pr.URL, pr.Version, pr.CSeq, pr.ClientRtpPort, pr.ClientRtcpPort)
}

func NewParseResult() *parseResult {
	return &parseResult{
		Method:         "",
		URL:            "",
		Version:        "",
		CSeq:           -1,
		ClientRtpPort:  -1,
		ClientRtcpPort: -1,
	}
}

type Worker struct {
	conn       net.Conn
	id         int
	clientAddr string
}

func NewWorker(conn net.Conn, id int) *Worker {
	return &Worker{conn: conn, id: id, clientAddr: conn.RemoteAddr().String()}
}

func (w *Worker) String() string {
	return fmt.Sprintf("{ID: %d, clientAddr: %v}", w.id, w.clientAddr)
}

func (w *Worker) Process() {
	defer w.conn.Close()
	defer fmt.Printf("========== close conn[%v] =============\n", w)

	// 针对当前连接做发送和接收操作
	reader := bufio.NewReader(w.conn)

	var rBuf [BUF_SIZE]byte
	for {
		recv_size, err := reader.Read(rBuf[:])
		if err != nil {
			fmt.Printf("read from conn failed, err:%v\n", err)
			break
		}

		recv := string(rBuf[:recv_size])
		fmt.Printf("[%d] 收到的数据 (size is %d):\n%v\n", w.id, recv_size, recv)

		req, err := parseRecv(recv)
		if err != nil {
			fmt.Printf("parse Recv err:%v\n", err)
		}
		fmt.Printf("parse result:%+v\n", req)

		err = nil
		var resp string = ""
		switch req.Method {
		case OPTIONS:
			resp, err = handleOptions(req)
		case DESCRIBE:
			resp, err = handleDescribe(req)
		case SETUP:
			resp, err = handleSetup(req)
		case PLAY:
			resp, err = handlePlay(req)
		default:
			fmt.Printf("unknow methd: %s\n", req.Method)
		}

		if err != nil {
			fmt.Printf("failed to create %s response msg, err: %v\n", req.Method, err)
			break
		}

		fmt.Printf("create resp:\n%v\n", resp)
		sBuf := []byte(resp)
		if _, err := w.conn.Write(sBuf[:]); err != nil {
			fmt.Printf("failed to handle %s, err:%v", req.Method, err)
		}
		fmt.Printf("============= handle %s done =============\n", req.Method)

		// 开始发送视频数据
		if req.Method == PLAY {
			w.sendVideoData(req)
			break // 退出循环
		}
	}
}

func parseRecv(recv string) (*parseResult, error) {
	res := NewParseResult()
	lines := strings.Split(recv, SEP)
	if len(lines) == 0 {
		return nil, fmt.Errorf("recv is empty")
	}

	var err error = nil
	for _, line := range lines {
		if isMethodLine(line) {
			if _, err = fmt.Sscanf(line, "%s %s %s\r\n", &res.Method, &res.URL, &res.Version); err != nil {
				fmt.Printf("parse Method err: %v\n", err)
				return nil, fmt.Errorf("parse Method line err: %v", err)
			}
		} else if isCSeqLine(line) {
			if _, err = fmt.Sscanf(line, "CSeq: %d\r\n", &res.CSeq); err != nil {
				fmt.Printf("parse CSeq err: %v\n", err)
				return nil, fmt.Errorf("parse CSeq line err: %v", err)
			}
		} else if isTransportLine(line) {
			if _, err = fmt.Sscanf(line, "Transport: RTP/AVP/UDP;unicast;client_port=%d-%d\r\n",
				&res.ClientRtpPort, &res.ClientRtcpPort); err != nil {
				fmt.Printf("parse Transport line err: %v\n", err)
				return nil, fmt.Errorf("parse Transport err: %v", err)
			}
		} else {
			// TODO
		}
	}
	return res, nil
}

func isMethodLine(line string) bool {
	return strings.HasPrefix(line, OPTIONS) ||
		strings.HasPrefix(line, DESCRIBE) ||
		strings.HasPrefix(line, SETUP) ||
		strings.HasPrefix(line, PLAY)
}

func isCSeqLine(line string) bool {
	return strings.HasPrefix(line, CSEQ)
}

func isTransportLine(line string) bool {
	return strings.HasPrefix(line, TRANSPORT)
}

func handleOptions(req *parseResult) (string, error) {
	resp := fmt.Sprintf("RTSP/1.0 200 OK\r\n"+
		"CSeq: %d\r\n"+
		"Public: OPTIONS, DESCRIBE, SETUP, PLAY\r\n"+
		"\r\n",
		req.CSeq)
	return resp, nil
}

func handleDescribe(req *parseResult) (string, error) {
	var sdp string
	var localIp string

	_, err := fmt.Sscanf(req.URL, "rtsp://%s", &localIp)
	if err != nil {
		return "", fmt.Errorf("handleDescribe scan IP failed, err:%v", err)
	}

	sdp = fmt.Sprintf("v=0\r\n"+
		"o=- 9%d 1 IN IP4 %s\r\n"+
		"t=0 0\r\n"+
		"a=control:*\r\n"+
		"m=video 0 RTP/AVP 96\r\n"+
		"a=rtpmap:96 H264/90000\r\n"+
		"a=control:track0\r\n",
		time.Now().Unix(), localIp)

	resp := fmt.Sprintf("RTSP/1.0 200 OK\r\nCSeq: %d\r\n"+
		"Content-Base: %s\r\n"+
		"Content-type: application/sdp\r\n"+
		"Content-length: %du\r\n\r\n"+
		"%s",
		req.CSeq,
		req.URL,
		len(sdp),
		sdp)
	return resp, nil
}

func handleSetup(req *parseResult) (string, error) {
	resp := fmt.Sprintf("RTSP/1.0 200 OK\r\n"+
		"CSeq: %d\r\n"+
		"Transport: RTP/AVP;unicast;client_port=%d-%d;server_port=%d-%d\r\n"+
		"Session: 66334873\r\n"+
		"\r\n",
		req.CSeq,
		req.ClientRtpPort,
		req.ClientRtpPort+1,
		SERVER_RTP_PORT,
		SERVER_RTCP_PORT)
	return resp, nil
}

func handlePlay(req *parseResult) (string, error) {
	resp := fmt.Sprintf("RTSP/1.0 200 OK\r\n"+
		"CSeq: %d\r\n"+
		"Range: npt=0.000-\r\n"+
		"Session: 66334873; timeout=10\r\n\r\n",
		req.CSeq)
	return resp, nil
}

func (w *Worker) sendVideoData(req *parseResult) {
	fmt.Printf("start play\n")
	// printf("client ip:%s\n", clientIP);
	// printf("client port:%d\n", clientRtpPort);

	for {
		fmt.Printf("send video data...\n")
		time.Sleep(5 * time.Second)
	}
}
