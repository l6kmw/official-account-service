package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestTokenRefreshLockerSerializesRefresh(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	locker := NewTokenRefreshLocker(client)
	locker.retryDelay = time.Millisecond

	first, err := locker.AcquireTokenRefreshLock(ctx, "tenant-1:1:wx-component", time.Minute)
	require.NoError(t, err)
	acquired := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		second, lockErr := locker.AcquireTokenRefreshLock(ctx, "tenant-1:1:wx-component", time.Minute)
		if lockErr != nil {
			done <- lockErr
			return
		}
		close(acquired)
		if err := second.Release(ctx); err != nil {
			done <- err
			return
		}
		done <- nil
	}()

	select {
	case <-acquired:
		t.Fatal("second lock acquired before first release")
	case <-time.After(20 * time.Millisecond):
	}
	require.NoError(t, first.Release(ctx))
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("second lock was not acquired after first release")
	}
	require.NoError(t, <-done)
}

func TestTokenRefreshLockerValidatesInput(t *testing.T) {
	_, err := OpenClient(context.Background(), "")
	require.Error(t, err)

	locker := NewTokenRefreshLocker(nil)
	_, err = locker.AcquireTokenRefreshLock(context.Background(), "key", time.Minute)
	require.Error(t, err)
}
