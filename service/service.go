package service

import (
	"bufio"
	"fmt"
	"goRTSP/utils"
	"net"
	"os"
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

	WORKER_CACHED_CAP   = 100
	FRAME_OUT_CHAN_SIZE = 32

	TEST_VIDEO_FILE_NAME = "videos/test.h264"
	SEND_FRAME_SLEEP_GAP = 30 * time.Millisecond
	FPS                  = 29 // 每秒几帧 TODO 应该从视频文件中获取
)

var (
	WorkerCached = NewWorkerCache(WORKER_CACHED_CAP)
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
	conn           net.Conn
	id             int
	clientAddr     string
	clientRtpPort  int
	clientRtcpPort int
	rtpConn        *net.UDPConn
	rtcpConn       *net.UDPConn
}

func NewWorker(conn net.Conn, id int) *Worker {
	return &Worker{
		conn:       conn,
		id:         id,
		clientAddr: conn.RemoteAddr().String(),
		rtpConn:    nil,
		rtcpConn:   nil,
	}
}

// re-write field for re-using Worker struct
func (w *Worker) ReFresh(conn net.Conn, id int) {
	w.conn, w.id = conn, id
	w.clientAddr = conn.RemoteAddr().String()
	w.clientRtpPort, w.clientRtcpPort = -1, -1

	if w.rtpConn != nil {
		if err := w.rtpConn.Close(); err != nil {
			fmt.Printf("Close rtp conn: %v failed\n", w.rtpConn.RemoteAddr().String())
		}
	}
	if w.rtcpConn != nil {
		if err := w.rtcpConn.Close(); err != nil {
			fmt.Printf("Close rtcp conn: %v failed\n", w.rtcpConn.RemoteAddr().String())
		}
	}
	w.rtpConn, w.rtcpConn = nil, nil
}

func (w *Worker) String() string {
	return fmt.Sprintf("{ID: %d, clientAddr: %v, clientRtpPort: %v, clientRtcpPort: %v}\n",
		w.id, w.clientAddr, w.clientRtpPort, w.clientRtcpPort)
}

func (w *Worker) Process() {
	defer w.conn.Close()
	defer WorkerCached.Put(w)
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
			resp, err = handleSetup(req, w)
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

func buildRtpAndRtcpConn(protocal, clientIP string, clientRtpPort, clientRtcpPort int) (
	rtpConn *net.UDPConn,
	rtcpConn *net.UDPConn,
	err error) {

	if rtpAddr, err := net.ResolveUDPAddr(protocal, fmt.Sprintf("%v:%v", clientIP, clientRtpPort)); err != nil {
		return nil, nil, err
	} else {
		if rtpConn, err = net.DialUDP(protocal, nil, rtpAddr); err != nil {
			return nil, nil, err
		}
	}

	if rtcpAddr, err := net.ResolveUDPAddr(protocal, fmt.Sprintf("%v:%v", clientIP, clientRtcpPort)); err != nil {
		return nil, nil, err
	} else {
		if rtcpConn, err = net.DialUDP(protocal, nil, rtcpAddr); err != nil {
			return nil, nil, err
		}
	}

	return rtpConn, rtcpConn, nil
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

func handleSetup(req *parseResult, w *Worker) (string, error) {
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

	w.clientRtpPort, w.clientRtcpPort = req.ClientRtpPort, req.ClientRtcpPort
	clientIP, _, err := utils.FetchIPAndPort(w.clientAddr)
	if err != nil {
		return "", err
	}

	// TODO 建立 UDP 音频、视频的传输通道，绑定客户端端口
	if w.rtpConn, w.rtcpConn, err = buildRtpAndRtcpConn("udp", clientIP, w.clientRtpPort, w.clientRtcpPort); err != nil {
		return "", err
	}
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

	rtpHeader := NewRtpHeader(0, 0, 0, RTP_VESION, RTP_PAYLOAD_TYPE_H264, 0, 0, 0, 0x88923423)
	rtpPkg := NewRtpPacket(rtpHeader, RTP_MAX_PKT_SIZE+2)

	frameOutCh := make(chan []byte, FRAME_OUT_CHAN_SIZE)
	errChan := make(chan error)

	f, err := os.Open(TEST_VIDEO_FILE_NAME)
	if err != nil {
		fmt.Printf("open video failed: %v\n", err)
		panic(err)
	}
	defer f.Close()
	go utils.ReadH264Worker(f, frameOutCh, errChan)

	for {
		select {
		case frame := <-frameOutCh:
			if len(frame) == 0 { // init frame
				continue
			}
			if err := w.RtpSendH264Frame(frame, rtpPkg); err != nil {
				fmt.Printf("when send H264 frame, got err: %v\n", err)
			}
		case err := <-errChan:
			fmt.Printf("got err in Read H264 Worker: %v\n", err)
			return
		}

		// TODO
		time.Sleep(SEND_FRAME_SLEEP_GAP)
	}
}

func (w *Worker) RtpSendH264Frame(frame []byte, rtpPkg *RtpPacket) error {
	// 每次开始前恢复最大容量，再根据实际情况切片
	rtpPkg.Payload = rtpPkg.Payload[:RTP_MAX_PKT_SIZE+2] // TODO 这个耦合太严重了
	naluType := frame[0]
	frameSize := len(frame)

	if frameSize <= RTP_MAX_PKT_SIZE { // 一次就能发送完成
		/*   0 1 2 3 4 5 6 7 8 9
		/*  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		/*  |F|NRI|  Type   | a single NAL unit ... |
		/*  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		*/
		rtpPkg.Payload = rtpPkg.Payload[:len(frame)]
		copy(rtpPkg.Payload, frame)

		if err := w.RtpSendPkgWithUDP(rtpPkg); err != nil {
			return fmt.Errorf("send pkg with udp failed: %v", err)
		}

		rtpPkg.Header.Seq++
		// 如果是 SPS、PPS 就不需要加时间戳
		if (naluType&SPS_PPS_MASK) == 7 || (naluType&SPS_PPS_MASK) == 8 {
			return nil
		}

	} else { // nalu长度小于最大包长度：分片模式
		//*  0                   1                   2
		//*  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3
		//* +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		//* | FU indicator  |   FU header   |   FU payload   ...  |
		//* +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

		//*     FU Indicator
		//*    0 1 2 3 4 5 6 7
		//*   +-+-+-+-+-+-+-+-+
		//*   |F|NRI|  Type   |
		//*   +---------------+

		//*      FU Header
		//*    0 1 2 3 4 5 6 7
		//*   +-+-+-+-+-+-+-+-+
		//*   |S|E|R|  Type   |
		//*   +---------------+

		pkgNum := frameSize / RTP_MAX_PKT_SIZE        // 有几个完整的包
		remainPkgSize := frameSize % RTP_MAX_PKT_SIZE // 剩余不完整包的大小
		fmt.Printf("pkgNum: %v, remain pkg size: %v\n", pkgNum, remainPkgSize)
		pos := 1                                             // frame 发送位置
		rtpPkg.Payload = rtpPkg.Payload[:RTP_MAX_PKT_SIZE+2] // +2 是前两字节

		// 发送完整的包
		for i := 0; i < pkgNum; i++ {
			rtpPkg.Payload[0] = (naluType & 0x60) | 28
			rtpPkg.Payload[1] = naluType & 0x1F

			if i == 0 { //第一包数据
				rtpPkg.Payload[1] |= 0x80 // start
			} else if (remainPkgSize == 0) && (i == pkgNum-1) { //最后一包数据
				rtpPkg.Payload[1] |= 0x40 // end
			}

			// 从 pos 开始的 RTP_MAX_PKT_SIZE 字节
			copy(rtpPkg.Payload[2:], frame[pos:pos+RTP_MAX_PKT_SIZE])
			if err := w.RtpSendPkgWithUDP(rtpPkg); err != nil {
				return fmt.Errorf("send No.%d pkg with udp failed: %v", i+1, err)
			}

			rtpPkg.Header.Seq++
			pos += RTP_MAX_PKT_SIZE
		}

		// 发送剩余的数据
		if remainPkgSize > 0 {
			rtpPkg.Payload[0] = (naluType & 0x60) | 28
			rtpPkg.Payload[1] = naluType & 0x1F
			rtpPkg.Payload[1] |= 0x40 //end

			// 从 pos 位置开始的 remainPkgSize 个字节
			copy(rtpPkg.Payload[2:], frame[pos:])
			if err := w.RtpSendPkgWithUDP(rtpPkg); err != nil {
				return fmt.Errorf("send final pkg with udp failed: %v", err)
			}

			rtpPkg.Header.Seq++
		}
	}

	rtpPkg.Header.Timestamp += 90000 / FPS // TODO why 90000?
	return nil
}

func (w *Worker) RtpSendPkgWithUDP(pkg *RtpPacket) error {
	data := make([]byte, 0, 32+len(pkg.Payload))
	data = append(data, pkg.Header.First)
	data = append(data, pkg.Header.Second)

	// 转为大端序
	seq, ts, ssrc := RtpHeaderBigEndian(pkg.Header.Seq, pkg.Header.Timestamp, pkg.Header.SSRC)
	data = append(data, seq...)
	data = append(data, ts...)
	data = append(data, ssrc...)

	data = append(data, pkg.Payload...)

	// send w/ UDP
	sendBytes, err := w.rtpConn.Write(data)
	if err != nil {
		fmt.Printf("send to %v failed, err: %v\n", w.clientAddr, err)
		return err
	}
	fmt.Printf("send to %v success, send bytes size: %v\n", w.clientAddr, sendBytes)
	return nil
}
