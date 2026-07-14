package memory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"official-account-service/internal/domain/publish"
)

func TestStoreAllowsOnePublishingRecordPerArticle(t *testing.T) {
	store := NewStore(time.Now)
	errs := make(chan error, 20)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.CreatePublishRecord(context.Background(), "tenant-1", publish.Record{
				ArticleID: 1, AuthorizerID: 1, Status: publish.StatusPublishing,
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	created := 0
	conflicts := 0
	for err := range errs {
		if err == nil {
			created++
			continue
		}
		require.True(t, errors.Is(err, publish.ErrPublishInProgress))
		conflicts++
	}
	require.Equal(t, 1, created)
	require.Equal(t, 19, conflicts)
}
