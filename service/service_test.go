package service

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRecv(t *testing.T) {
	cases := []struct {
		recv string
		pass bool
	}{
		{
			fmt.Sprintf("%s rtsp://127.0.0.1:8554 RTSP/1.0\r\nCSeq: 1\r\nUser-Agent: Lavf57.83.100\r\n", OPTIONS),
			true,
		},
		{
			fmt.Sprintf("%s rtsp://127.0.0.1:8554 RTSP/1.0\r\nCSeq: 1\r\nUser-Agent: Lavf57.83.100\r\n", DESCRIBE),
			true,
		},
		{
			fmt.Sprintf("%s rtsp://127.0.0.1:8554 RTSP/1.0\r\nCSeq: 1\r\nUser-Agent: Lavf57.83.100\r\n", SETUP),
			true,
		},
		{
			fmt.Sprintf("%s rtsp://127.0.0.1:8554 RTSP/1.0\r\nCSeq: 1\r\nUser-Agent: Lavf57.83.100\r\n", PLAY),
			true,
		},
		{
			"OPTIONS rtsp://127.0.0.1:8554 RTSP/1.0\r\nCSeq: a\r\nUser-Agent: Lavf57.83.100\r\n",
			false,
		},
	}

	for i, ca := range cases {
		res, err := parseRecv(ca.recv)
		t.Logf("[case %d] res: %+v\n", i, res)

		if !ca.pass {
			t.Logf("err:%v\n", err)
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}

}
