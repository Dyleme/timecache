package timecache_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slices"

	"github.com/Dyleme/timecache"
)

func TestCache_Get(t *testing.T) {
	t.Parallel()
	t.Run("basic get", func(t *testing.T) {
		t.Parallel()
		c := timecache.New[int, int]()
		amount := 100
		for i := 0; i < amount; i++ {
			c.StoreDefDur(i, i)
		}
		for i := 0; i < amount; i++ {
			el, err := c.Get(i)
			assert.NoError(t, err)
			assert.Equal(t, i, el)
		}
	})
}

func TestCache_ExpirationGet(t *testing.T) {
	t.Parallel()
	t.Run("expiration", func(t *testing.T) {
		t.Parallel()
		c := timecache.New[int, int]()
		amount := 100
		expiredVals := []int{1, 10, 25, 76}
		for i := 0; i < amount; i++ {
			if slices.Contains(expiredVals, i) {
				c.Store(i, i, 10*time.Millisecond)
			} else {
				c.StoreDefDur(i, i)
			}
		}
		time.Sleep(15 * time.Millisecond)
		for i := 0; i < amount; i++ {
			el, err := c.Get(i)
			if slices.Contains(expiredVals, i) {
				assert.Error(t, err)
				assert.Equal(t, 0, el)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, i, el)
			}
		}
	})
}

func TestCache_DeletedGet(t *testing.T) {
	t.Parallel()
	t.Run("deleted", func(t *testing.T) {
		t.Parallel()
		c := timecache.New[int, int]()
		amount := 100
		for i := 0; i < amount; i++ {
			c.StoreDefDur(i, i)
		}

		deletedVals := []int{1, 10, 25, 76}
		for _, d := range deletedVals {
			c.Delete(d)
		}

		time.Sleep(15 * time.Millisecond)
		for i := 0; i < amount; i++ {
			el, err := c.Get(i)
			if slices.Contains(deletedVals, i) {
				assert.Error(t, err)
				assert.Equal(t, 0, el)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, i, el)
			}
		}
	})
}

func TestCache_CleanExpired(t *testing.T) {
	t.Parallel()
	t.Run("expiration", func(t *testing.T) {
		t.Parallel()
		c := timecache.New[int, int]()
		amount := 100
		var expiredVals []int
		for i := 0; i < amount; i++ {
			if i%10 == 0 {
				expiredVals = append(expiredVals, i)
			}
		}

		for i := 0; i < amount; i++ {
			if slices.Contains(expiredVals, i) {
				c.Store(i, i, 10*time.Millisecond)
			} else {
				c.StoreDefDur(i, i)
			}
		}

		assert.Equal(t, c.ObjectAmount(), amount)
		time.Sleep(15 * time.Millisecond)
		c.CleanExpired()
		assert.Equal(t, c.ObjectAmount(), amount-len(expiredVals))
	})

	t.Run("non blocking deletion", func(t *testing.T) {
		t.Parallel()
		c := timecache.NewWithConfig[int, int](timecache.Config{
			JanitorConfig: timecache.JanitorConfig{
				StopJanitorEvery: 10000,
			},
		})
		amount := 10000000
		for i := 0; i < amount; i++ {
			c.StoreDefDur(i, i)
		}

		expiredDone := make(chan struct{})
		var elapsedTime time.Duration
		go func() {
			startCleaning := time.Now()
			c.CleanExpired()
			elapsedTime = time.Since(startCleaning)
			expiredDone <- struct{}{}
		}()

		var maxWaitingTime time.Duration
	loop:
		for {
			select {
			default:
				startGetting := time.Now()
				c.Get(1)
				waitingTime := time.Since(startGetting)
				maxWaitingTime = max(maxWaitingTime, waitingTime)
			case <-expiredDone:
				break loop
			}
		}
		assert.Less(t, maxWaitingTime, elapsedTime)
	})
}

func max(d1, d2 time.Duration) time.Duration {
	if int64(d1) > int64(d2) {
		return d1
	}

	return d2
}
