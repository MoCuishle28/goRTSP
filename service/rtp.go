package service

import (
	"encoding/binary"
)

/*
 *    0                   1                   2                   3
 *    7 6 5 4 3 2 1 0|7 6 5 4 3 2 1 0|7 6 5 4 3 2 1 0|7 6 5 4 3 2 1 0
 *   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 *   |V=2|P|X|  CC   |M|     PT      |       sequence number         |
 *   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 *   |                           timestamp                           |
 *   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 *   |           synchronization source (SSRC) identifier            |
 *   +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
 *   |            contributing source (CSRC) identifiers             |
 *   :                             ....                              :
 *   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 *
 */

const (
	// TODO 确认这 4 个域在一个字节中所在位的顺序
	// first 8 bytes
	Version       = 0b11000000 //RTP 协议的版本号，占 2 位，当前协议版本号为 2。
	VersionLeft   = 6
	Padding       = 0b00100000 //填充标志，占 1 位，如果 P=1，则在该报文的尾部填充一个或多个额外的 8 位组，它们不是有效载荷的一部分。
	PaddingLeft   = 5
	Extension     = 0b00010000 //占 1 位，如果 X=1，则在 RTP 报头后跟有一个扩展报头。
	ExtensionLeft = 4
	CSRCLen       = 0b00001111 //CSRC 计数器，占 4 位，指示 CSRC 标识符的个数。

	// second 8 bytes
	Marker      = 0b10000000 //标记，占 1 位，不同的有效载荷有不同的含义，对于视频，标记一帧的结束；对于音频，标记会话的开始。
	MarkerLeft  = 7
	PayloadType = 0b01111111 //有效载荷类型，占 7 位，用于说明 RTP 报文中有效载荷的类型，如 GSM 音频、JPEM 图像等。

	// RTP 包的最大 size, 超出的话需要分片发送
	RTP_MAX_PKT_SIZE = 1400

	// RTP Header init
	RTP_VESION            = 2
	RTP_PAYLOAD_TYPE_H264 = 96
	DEFAULT_CSRCLen       = 0
	DEFAULT_Extension     = 0
	DEFAULT_Padding       = 0
	DEFAULT_Marker        = 0
	DEFAULT_Seq           = 0
	DEFAULT_Timestamp     = 0
	DEFAULT_SSRC          = 0x88923423

	// SPS, PPS
	SPS_PPS_MASK = 0x1F
)

type RtpHeader struct {
	/* byte 0 */
	First byte

	/* byte 1 */
	Second byte

	/* bytes 2,3 */
	Seq uint16 //占16位，用于标识发送者所发送的RTP报文的序列号，每发送一个报文，序列号增1。接收者通过序列号来检测报文丢失情况，重新排序报文，恢复数据。

	/* bytes 4-7 */
	Timestamp uint32 //占32位，时戳反映了该RTP报文的第一个八位组的采样时刻。接收者使用时戳来计算延迟和延迟抖动，并进行同步控制。

	/* bytes 8-11 */
	SSRC uint32 //占32位，用于标识同步信源。该标识符是随机选择的，参加同一视频会议的两个同步信源不能有相同的SSRC。

	/*标准的RTP Header 还可能存在 0-15个特约信源(CSRC)标识符

	  每个CSRC标识符占32位，可以有0～15个。每个CSRC标识了包含在该RTP报文有效载荷中的所有特约信源
	*/
}

func (h *RtpHeader) SetVersion(version uint8) {
	h.First = h.First | (version << VersionLeft)
}

func (h *RtpHeader) SetPadding(padding uint8) {
	h.First = h.First | (padding << PaddingLeft)
}

func (h *RtpHeader) SetExtension(extension uint8) {
	h.First = h.First | (extension << ExtensionLeft)
}

func (h *RtpHeader) SetCSRCLen(csrcLen uint8) {
	h.First = h.First | csrcLen
}

func (h *RtpHeader) SetMarker(marker uint8) {
	h.Second = h.Second | (marker << MarkerLeft)
}

func (h *RtpHeader) SetPayloadType(payloadType uint8) {
	h.Second = h.Second | payloadType
}

func RtpHeaderBigEndian(rtpSeq uint16, rtpTS uint32, rtpSSRC uint32) ([]byte, []byte, []byte) {
	// Network byte order is just big endian
	seq := make([]byte, 2)
	ts := make([]byte, 4)
	ssrc := make([]byte, 4)
	binary.BigEndian.PutUint16(seq, rtpSeq)
	binary.BigEndian.PutUint32(ts, rtpTS)
	binary.BigEndian.PutUint32(ssrc, rtpSSRC)
	return seq, ts, ssrc
}

func FetchFirstAndSecond(rtp *RtpHeader) (version, padding, extension, csrcLen uint8, marker, payloadType uint8) {
	version = (rtp.First & Version) >> VersionLeft
	padding = (rtp.First & Padding) >> PaddingLeft
	extension = (rtp.First & Extension) >> ExtensionLeft
	csrcLen = rtp.First & CSRCLen

	marker = (rtp.Second & Marker) >> MarkerLeft
	payloadType = rtp.Second & PayloadType
	return
}

func NewRtpHeader(csrcLen, extension, padding, version, payloadType, marker uint8, seq uint16, timestamp, ssrc uint32) *RtpHeader {
	header := new(RtpHeader)
	header.SetCSRCLen(csrcLen)
	header.SetExtension(extension)
	header.SetPadding(padding)
	header.SetVersion(version)
	header.SetPayloadType(payloadType)
	header.SetMarker(marker)
	header.Seq = seq
	header.Timestamp = timestamp
	header.SSRC = ssrc
	return header
}

type RtpPacket struct {
	Header  *RtpHeader
	Payload []byte
}

func NewRtpPacket(header *RtpHeader, payloadSize int) *RtpPacket {
	return &RtpPacket{
		Header:  header,
		Payload: make([]byte, payloadSize),
	}
}
