package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRtpHeader(t *testing.T) {
	cases := []struct {
		first       byte
		parseFirst  []byte
		second      byte
		parseSecond []byte
		seq         uint16
		ts          uint32
		ssrc        uint32
	}{
		{
			0b10101001,
			[]byte{2, 1, 0, 9},
			0b10101101,
			[]byte{1, 45},
			42,
			uint32(time.Now().Unix()),
			42,
		},
		{
			0b11111111,
			[]byte{3, 1, 1, 15},
			0b11111111,
			[]byte{1, 127},
			42,
			uint32(time.Now().Add(time.Hour * 24).Unix()),
			999,
		},
	}
	rtp := RtpHeader{}
	for i, ca := range cases {
		t.Logf("---------- case %d ----------", i)
		rtp.First, rtp.Second = ca.first, ca.second
		rtp.Seq, rtp.Timestamp, rtp.SSRC = ca.seq, ca.ts, ca.ssrc

		version, padding, extension, csrcLen, marker, payloadType := FetchFirstAndSecond(&rtp)
		require.Equal(t, ca.parseFirst[0], version)
		require.Equal(t, ca.parseFirst[1], padding)
		require.Equal(t, ca.parseFirst[2], extension)
		require.Equal(t, ca.parseFirst[3], csrcLen)

		require.Equal(t, ca.parseSecond[0], marker)
		require.Equal(t, ca.parseSecond[1], payloadType)

		// 一般 CPU 内部使用小端序存储
		// 大端序是符合人直观的
		// 例如对于一个 32 位变量（4 字节）：0x12345678
		// 低地址 -> 高地址
		// 大端序：0x 12 34 56 78
		// 小端序：0x 78 56 34 12
		// Network byte order is just big endian
		seq, ts, ssrc := RtpHeaderBigEndian(rtp.Seq, rtp.Timestamp, rtp.SSRC)
		t.Logf("seq: %v, %04x", seq, rtp.Seq)
		t.Logf("ts: %v, %08x", ts, rtp.Timestamp)
		t.Logf("ssrc: %v, %08x", ssrc, rtp.SSRC)
	}
}

func TestNewRtpHeader(t *testing.T) {
	cases := []struct {
		csrcLen     uint8
		extension   uint8
		padding     uint8
		version     uint8
		payloadType uint8
		marker      uint8
		seq         uint16
		timestamp   uint32
		ssrc        uint32
	}{
		{
			DEFAULT_CSRCLen,
			DEFAULT_Extension,
			DEFAULT_Padding,
			RTP_VESION,
			RTP_PAYLOAD_TYPE_H264,
			DEFAULT_Marker,
			DEFAULT_Seq,
			DEFAULT_Timestamp,
			DEFAULT_SSRC,
		},
		{
			0b0111,
			1,
			1,
			0b11,
			0b01111110,
			0b1,
			42,
			999,
			998,
		},
	}

	for i, ca := range cases {
		t.Logf("----- case %d -----", i)
		header := NewRtpHeader(ca.csrcLen, ca.extension, ca.padding, ca.version, ca.payloadType, ca.marker, ca.seq, ca.timestamp, ca.ssrc)
		version, padding, extension, csrcLen, marker, payloadType := FetchFirstAndSecond(header)
		require.Equal(t, ca.version, version)
		require.Equal(t, ca.padding, padding)
		require.Equal(t, ca.extension, extension)
		require.Equal(t, ca.csrcLen, csrcLen)
		require.Equal(t, ca.marker, marker)
		require.Equal(t, ca.payloadType, payloadType)
		t.Logf("version: %v, padding: %v, extensionL %v, casrcLen: %v, marker: %v, payloadType: %v", version, padding, extension, csrcLen, marker, payloadType)

		require.Equal(t, ca.seq, header.Seq)
		require.Equal(t, ca.timestamp, header.Timestamp)
		require.Equal(t, ca.ssrc, header.SSRC)
	}
}
