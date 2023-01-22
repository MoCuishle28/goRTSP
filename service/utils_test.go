package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkerCached(t *testing.T) {
	cached := NewWorkerCache(5)
	w := cached.Get()
	require.Equal(t, 4, cached.Len())

	cached.Put(w)
	require.Equal(t, 5, cached.Len())

	arr := make([]*Worker, 4)
	for i := 0; i < 4; i++ {
		arr[i] = cached.Get()
	}
	require.Equal(t, 1, cached.Len())

	for _, w := range arr {
		cached.Put(w)
	}
	require.Equal(t, 5, cached.Len())

	for i := 0; i < 5; i++ {
		_ = cached.Get()
	}
	require.Equal(t, 0, cached.Len())
	w2 := cached.Get()
	require.Equal(t, 0, cached.Len())
	require.NotEqual(t, nil, w2)
}
