package main

import (
	"github.com/garyburd/redigo/redis"
	"log"
	"strings"
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
	mc.cache = make(map[string]string)
	mc.Mutex = &sync.Mutex{}
	return mc
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
		defer close(c)
		for k, v := range mc.cache {
			c <- CacheItem{k, v}
		}
	}()
	return c
}

func (mc *MemoryCache) SetCacheDuration(d time.Duration) {
	mc.cacheDuration = d
}

type RedisCache struct {
	rdb           redis.Conn
	cacheDuration time.Duration
}

func NewRedisCache(rdb redis.Conn) *RedisCache {
	return &RedisCache{
		rdb: rdb,
	}
}

func (rc *RedisCache) Add(ci CacheItem) {
	key := "cacheitem:" + ci.ImportPath
	_, err := rc.rdb.Do("SET", key, ci.RepoUrl)
	if err != nil {
		log.Printf("Could not add cache item: %s", err)
	}
	_, err = rc.rdb.Do("EXPIRE", key, int(rc.cacheDuration/time.Second))
	if err != nil {
		log.Printf("Could not expire cache item: %s", err)
	}
}

func (rc *RedisCache) Iter() <-chan CacheItem {
	c := make(chan CacheItem)
	go func() {
		defer close(c)
		resp, err := redis.Values(rc.rdb.Do("KEYS", "cacheitem:*"))
		if err != nil {
			log.Printf("Could not get keys: %s", err)
		}
		var keys []string
		redis.ScanSlice(resp, &keys)
		for _, key := range keys {
			value, err := redis.String(rc.rdb.Do("GET", key))
			if err != nil {
				log.Printf("Could not get key's value: %s", err)
				continue
			}
			c <- CacheItem{strings.TrimPrefix(key, "cacheitem:"), value}
		}
	}()
	return c
}

func (rc *RedisCache) SetCacheDuration(d time.Duration) {
	rc.cacheDuration = d
}
