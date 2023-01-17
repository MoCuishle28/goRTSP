package service

import (
	"fmt"
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
