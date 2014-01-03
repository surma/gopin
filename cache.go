package main

import (
	"sync"
	"time"
)

type Cache interface {
	Add(c CacheItem)
	Iter() <-chan CacheItem
	SetCacheDuration(d time.Duration)
}

type CacheItem struct {
	ImportPath, RepoUrl string
}

type MemoryCache struct {
	cacheDuration time.Duration
	cache         map[string]string
	*sync.Mutex
}

func NewMemoryCache() *MemoryCache {
	mc := &MemoryCache{}
	mc.init()
	return mc
}

func (mc *MemoryCache) init() {
	if mc.cache == nil {
		mc.cache = make(map[string]string)
	}
	if mc.Mutex == nil {
		mc.Mutex = &sync.Mutex{}
	}
}

func (mc *MemoryCache) Add(ci CacheItem) {
	mc.Lock()
	defer mc.Unlock()
	go func() {
		time.Sleep(mc.cacheDuration)
		mc.Lock()
		defer mc.Unlock()
		delete(mc.cache, ci.ImportPath)
	}()
	mc.cache[ci.ImportPath] = ci.RepoUrl
}

func (mc *MemoryCache) Iter() <-chan CacheItem {
	c := make(chan CacheItem)
	go func() {
		for k, v := range mc.cache {
			c <- CacheItem{k, v}
		}
		close(c)
	}()
	return c
}

func (mc *MemoryCache) SetCacheDuration(d time.Duration) {
	mc.cacheDuration = d
}
