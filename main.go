package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/voxelbrain/goptions"
)

var (
	options = struct {
		Listen        string        `goptions:"-l, --listen, description='Address to bind webserver to'"`
		Redis         *url.URL      `goptions:"-r, --redis, description='URL of Redis database'"`
		CacheDuration time.Duration `goptions:"-c, --cache, description='Duration to cache requested repo URLs'"`
		Static        string        `goptions:"--static, description='Path to static content directory'"`
		Help          goptions.Help `goptions:"-h, --help, description='Show this help'"`
	}{
		Listen:        "localhost:8081",
		CacheDuration: 5 * time.Minute,
		Static:        "./static",
	}
)

func main() {
	goptions.ParseAndFail(&options)

	cache := setupCache()
	cache.SetCacheDuration(options.CacheDuration)
	staticHandler := http.FileServer(http.Dir(options.Static))

	http.Handle("/github.com/", http.StripPrefix("/github.com", NewGithub(cache)))
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("go-get") == "1" {
			RenderGoImport(w, r, cache.Iter())
			return
		}
		staticHandler.ServeHTTP(w, r)
	}))
	log.Printf("Running webserver...")
	if err := http.ListenAndServe(options.Listen, nil); err != nil {
		log.Fatalf("Could not bind to port: %s", err)
	}
}

func setupCache() Cache {
	if options.Redis == nil {
		log.Printf("USING MEMORY CACHE!")
		return NewMemoryCache()
	}

	rdb, err := redis.Dial("tcp", options.Redis.Host)
	if err != nil {
		log.Fatalf("Could not connect to redis at %s: %s", options.Redis.Host, err)
	}

	if options.Redis.User != nil {
		pw, _ := options.Redis.User.Password()
		_, err := rdb.Do("AUTH", pw)
		if err != nil {
			log.Fatalf("Could not authenticate to redis at %s: %s", options.Redis.Host, err)
		}
	}

	_, err = rdb.Do("RANDOMKEY")
	if err != nil {
		log.Fatalf("Could not run against redis: %s", err)
	}

	return NewRedisCache(rdb)
}

func injectHead(r io.Reader, hash, head string) (io.Reader, error) {
	br := bufio.NewReader(r)
	buf := &bytes.Buffer{}
	head = "refs/heads/" + head + "\n"
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(line, head) {
			fmt.Fprintf(buf, "%s%s %s", line[0:4], hash, head)
			break
		}
		buf.Write([]byte(line))
	}
	return io.MultiReader(bytes.NewReader(buf.Bytes()), br), nil
}
