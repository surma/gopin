package main

import (
	"sync"
	"time"
)

type Cache interface {
	Add(importPath, repoUrl string)
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

func (mc *MemoryCache) init() {
	if mc.cache == nil {
		mc.cache = make(map[string]string)
	}
	if mc.Mutex == nil {
		mc.Mutex = &sync.Mutex{}
	}
}

func (mc *MemoryCache) Add(importPath, repoUrl string) {
	mc.init()
	mc.Lock()
	defer mc.Unlock()
	go func() {
		time.Sleep(mc.cacheDuration)
		mc.Lock()
		defer mc.Unlock()
		delete(mc.cache, importPath)
	}()
	mc.cache[importPath] = repoUrl
}

func (mc *MemoryCache) Iter() <-chan CacheItem {
	c := make(chan CacheItem)
	go func() {
		for k, v := range mc.cache {
			c <- CacheItem{k, v}
		}
	}()
	return c
}

func (mc *MemoryCache) SetCacheDuration(d time.Duration) {
	mc.cacheDuration = d
}
