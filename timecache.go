package main

import (
	"errors"
	"sync"
	"time"

	"golang.org/x/exp/constraints"
)

var (
	ErrNotExists = errors.New("obj not exists")
)

const (
	defaultCleanPeriodTime = time.Minute
	defaultStoreDuration   = 10 * time.Minute
)

type Cache[K constraints.Ordered, V any] struct {
	m            map[K]expVal[V]
	mu           sync.RWMutex
	defStoreTime time.Duration

	cleanPeriod time.Duration

	janitorStops     bool
	stopJanitorEvery int
}

type Config struct {
	StoreTime     time.Duration
	JanitorConfig JanitorConfig
}
type JanitorConfig struct {
	CleanPeriod      time.Duration
	StopJanitorEvery int
}

func (c *Cache[K, V]) runJanitor() {
	ticker := time.NewTicker(c.cleanPeriod)
	for {
		<-ticker.C
		c.CleanExpired()
	}
}

type expVal[V any] struct {
	expTime time.Time
	val     V
}

func NewCache[K constraints.Ordered, V any]() *Cache[K, V] {
	return NewCacheWithConfig[K, V](Config{
		JanitorConfig: JanitorConfig{
			CleanPeriod: defaultCleanPeriodTime,
		},
	})
}

func NewCacheWithConfig[K constraints.Ordered, V any](cfg Config) *Cache[K, V] {
	storeTime := cfg.StoreTime
	if storeTime == 0 {
		storeTime = defaultStoreDuration
	}
	c := Cache[K, V]{
		m:                make(map[K]expVal[V]),
		defStoreTime:     cfg.StoreTime,
		cleanPeriod:      cfg.JanitorConfig.CleanPeriod,
		janitorStops:     cfg.JanitorConfig.StopJanitorEvery != 0,
		stopJanitorEvery: cfg.JanitorConfig.StopJanitorEvery,
	}
	if cfg.JanitorConfig.CleanPeriod != 0 {
		go c.runJanitor()
	}
	return &c
}

func (c *Cache[K, V]) Store(key K, val V, dur time.Duration) {
	t := time.Now().UTC().Add(dur)
	c.mu.Lock()
	c.m[key] = expVal[V]{
		expTime: t,
		val:     val,
	}
	c.mu.Unlock()
}

func (c *Cache[K, V]) StoreDefDur(key K, val V) {
	c.Store(key, val, c.defStoreTime)
}

func (c *Cache[K, V]) zero() V {
	var zero V
	return zero
}

func (c *Cache[K, V]) Get(key K) (V, error) {
	now := time.Now()
	c.mu.RLock()
	obj, ok := c.m[key]
	if !ok {
		c.mu.RUnlock()
		return c.zero(), ErrNotExists
	}
	if obj.expTime.Before(now) {
		c.mu.RUnlock()
		return c.zero(), ErrNotExists
	}
	c.mu.RUnlock()
	return obj.val, nil
}

func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	delete(c.m, key)
	c.mu.Unlock()
}

func (c *Cache[K, V]) CleanExpired() {
	now := time.Now()
	c.mu.Lock()
	counter := 1
	for k, v := range c.m {
		if c.janitorStops && counter%c.stopJanitorEvery == 0 {
			c.mu.Unlock()
			c.mu.Lock()
		}
		if v.expTime.Before(now) {
			delete(c.m, k)
		}
		counter++
	}
	c.mu.Unlock()
}

func (c *Cache[K, V]) Update(key K, dur time.Duration, f func(v V) V) error {
	now := time.Now()
	newExpTime := now.Add(dur)
	c.mu.Lock()
	obj, ok := c.m[key]
	if !ok {
		c.mu.Unlock()
		return ErrNotExists
	}
	if obj.expTime.Before(now) {
		c.mu.Unlock()
		return ErrNotExists
	}
	obj.val = f(obj.val)
	obj.expTime = newExpTime
	c.m[key] = obj

	c.mu.Unlock()

	return nil
}

func (c *Cache[K, V]) ObjectAmount() int {
	c.mu.RLock()
	amount := len(c.m)
	c.mu.RUnlock()
	return amount
}
