package service

import (
	"encoding/binary"
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
			0b10011010,
			[]byte{9, 1, 0, 2},
			0b10101101,
			[]byte{86, 1},
			42,
			uint32(time.Now().Unix()),
			42,
		},
		{
			0b11111111,
			[]byte{15, 1, 1, 3},
			0b11111111,
			[]byte{127, 1},
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

		require.Equal(t, ca.parseFirst[0], (rtp.First&CSRCLen)>>CSRCLeft)
		require.Equal(t, ca.parseFirst[1], (rtp.First&Extension)>>ExtensionLeft)
		require.Equal(t, ca.parseFirst[2], (rtp.First&Padding)>>PaddingLeft)
		require.Equal(t, ca.parseFirst[3], rtp.First&Version)

		require.Equal(t, ca.parseSecond[0], (rtp.Second&PayloadType)>>PayloadTypeLeft)
		require.Equal(t, ca.parseSecond[1], rtp.Second&Marker)

		// 一般 CPU 内部使用小端序存储
		// 大端序是符合人直观的
		// 例如对于一个 32 位变量（4 字节）：0x12345678
		// 低地址 -> 高地址
		// 大端序：0x 12 34 56 78
		// 小端序：0x 78 56 34 12
		// Network byte order is just big endian
		seq := make([]byte, 2)
		ts := make([]byte, 4)
		ssrc := make([]byte, 4)
		binary.BigEndian.PutUint16(seq, rtp.Seq)
		binary.BigEndian.PutUint32(ts, rtp.Timestamp)
		binary.BigEndian.PutUint32(ssrc, rtp.SSRC)
		t.Logf("seq: %v, %04x", seq, rtp.Seq)
		t.Logf("ts: %v, %08x", ts, rtp.Timestamp)
		t.Logf("ssrc: %v, %08x", ssrc, rtp.SSRC)
	}
}
