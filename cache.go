package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
)

type Cache interface {
	Add(c CacheItem)
	Iter() <-chan CacheItem
	SetCacheDuration(d time.Duration)
}

type CacheItem struct {
	ImportPath string `json:"import_path"`
	RepoUrl    string `json:"repo_url"`
}

func (c CacheItem) Hash() string {
	h := sha256.New()
	io.WriteString(h, c.ImportPath)
	return hex.EncodeToString(h.Sum(nil))
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
	key := "cacheitem:" + ci.Hash()
	payload, err := json.Marshal(ci)
	if err != nil {
		log.Printf("Could not marshal cache item: %s", err)
		return
	}
	_, err = rc.rdb.Do("SET", key, payload)
	if err != nil {
		log.Printf("Could not add cache item: %s", err)
		return
	}
	_, err = rc.rdb.Do("EXPIRE", key, int(rc.cacheDuration/time.Second))
	if err != nil {
		log.Printf("Could not expire cache item: %s", err)
		return
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
		var ci CacheItem
		redis.ScanSlice(resp, &keys)
		for _, key := range keys {
			value, err := redis.String(rc.rdb.Do("GET", key))
			if err != nil {
				log.Printf("Could not get key's value: %s", err)
				continue
			}
			err = json.Unmarshal([]byte(value), &ci)
			if err != nil {
				log.Printf("Could not decode cache item: %s", err)
				continue
			}
			c <- ci
		}
	}()
	return c
}

func (rc *RedisCache) SetCacheDuration(d time.Duration) {
	rc.cacheDuration = d
}
