package main

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestCacheItem_Hash(t *testing.T) {
	ci := CacheItem{}
	h1 := ci.Hash()
	ci.RepoUrl = "SomeUrl"
	h2 := ci.Hash()
	if h1 != h2 {
		t.Fatalf("Hashes unexpectedly differ")
	}
}

func TestMemoryCache_Add(t *testing.T) {
	mc := NewMemoryCache()
	mc.SetCacheDuration(1 * time.Second)

	ci := CacheItem{"1", "2"}
	mc.Add(ci)
	select {
	case rci := <-mc.Iter():
		if !reflect.DeepEqual(rci, ci) {
			t.Fatalf("Cache items do not match")
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("Did not receive any items from cache")
	}
}

func TestMemoryCache_Expire(t *testing.T) {
	mc := NewMemoryCache()
	mc.SetCacheDuration(50 * time.Millisecond)

	ci := CacheItem{"1", "2"}
	mc.Add(ci)
	time.Sleep(60 * time.Millisecond)
	c := mc.Iter()
	x, ok := <-c
	if ok {
		t.Fatalf("Unexpectedly received item from cache: %#v", x)
	}
}

func TestRedisCache_Add(t *testing.T) {
	ci := CacheItem{"key1", "value1"}
	rdb := &RedisMock{
		DoFunc: func(command string, args ...interface{}) (interface{}, error) {
			switch command {
			case "SET":
				if !reflect.DeepEqual([]interface{}{"cacheitem:" + ci.Hash(), []byte(mustMarshal(ci))}, args) {
					t.Fatalf("Unexpected set parameters: %#v", args)
				}
				return nil, nil
			case "EXPIRE":
				return nil, nil
			default:
				t.Fatalf("Unexpected command: %s", command)
			}
			panic("unreachable")
		},
	}

	rc := NewRedisCache(rdb)
	rc.Add(ci)
}

func TestRedisCache_Iter(t *testing.T) {
	ci1, ci2 := CacheItem{"key1", "value1"}, CacheItem{"key2", "value2"}
	rdb := &RedisMock{
		DoFunc: func(command string, args ...interface{}) (interface{}, error) {
			switch command {
			case "KEYS":
				return []interface{}{[]byte("cacheitem:key1"), []byte("cacheitem:key2")}, nil
			case "GET":
				switch key := args[0].(string); key {
				case "cacheitem:key1":
					return mustMarshal(ci1), nil
				case "cacheitem:key2":
					return mustMarshal(ci2), nil
				default:
					t.Fatalf("Unexpected key request: %s", key)
				}
			default:
				t.Fatalf("Unexpected command: %s", command)
			}
			panic("unreachable")
		},
	}
	rc := NewRedisCache(rdb)

	for ci := range rc.Iter() {
		if !reflect.DeepEqual(ci, ci1) && !reflect.DeepEqual(ci, ci2) {
			t.Fatalf("Unexpected cache item: %#v", ci)
		}
	}
}

func TestRedisCache_SetCacheDuration(t *testing.T) {
	d := 1 * time.Second
	ci := CacheItem{"key", "value"}
	rdb := &RedisMock{
		DoFunc: func(command string, args ...interface{}) (interface{}, error) {
			switch command {
			case "SET":
				return nil, nil
			case "EXPIRE":
				if !reflect.DeepEqual([]interface{}{"cacheitem:" + ci.Hash(), int(d / time.Second)}, args) {
					t.Fatalf("Unexpected expire parameteres: %#v", args)
				}
				return nil, nil
			default:
				t.Fatalf("Unexpected command: %s", command)
			}
			panic("unreachable")
		},
	}
	rc := NewRedisCache(rdb)
	rc.SetCacheDuration(d)
	rc.Add(ci)
}

func mustMarshal(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}
