package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/surma/httptools"
	"github.com/voxelbrain/goptions"
)

var (
	options = struct {
		Listen        string        `goptions:"-l, --listen, description='Address to bind webserver to'"`
		Hostname      string        `goptions:"-h, --hostname, description='Hostname of this instance of gopin'"`
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
	http.Handle("/github.com/", http.StripPrefix("/github.com", NewGithub(cache)))
	http.handle("/", GoGetRouter{
		GoGet: listCacheHandler(cache),
		Else:  http.FileServer(http.Dir(options.Static)),
	})
	if err := http.ListenAndServe(options.Listen, nil); err != nil {
		log.Fatalf("Could not bind to port: %s", err)
	}
}

func setupCache() Cache {
	if options.Redis == nil {
		return &MemoryCache{}
	}

	rdb, err := redis.Dial("tcp", options.Redis.Host)
	if err != nil {
		log.Fatalf("Could not connect to redis at %s: %s", options.Redis.Host, err)
	}

	if options.RedisAddr.User != nil {
		pw, _ := options.RedisAddr.User.Password()
		_, err := db.Do("AUTH", pw)
		if err != nil {
			log.Fatalf("Could not authenticate to redis at %s: %s", options.Redis.Host, err)
		}
	}
	// FIXME: Actually use redis
	return &MemoryCache{}
}

var (
	knownPattern = regexp.MustCompilePOSIX("^/github.com/([^/]+)/([^/]+)/([0-9a-f]+)(/.+)?$")
)

func handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		fmt.Fprintf(w, "<head>")
		for meta, _ := range cache {
			fmt.Fprintf(w, meta)
		}
		fmt.Fprintf(w, "</head>")
		return
	}

	p := knownPattern.FindStringSubmatch(r.URL.Path)
	if p == nil || len(p) == 0 {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if (p[4] == "" || p[4] == "/") && r.URL.RawQuery == "go-get=1" {
		meta := fmt.Sprintf(`<meta name="go-import" content="gopin.localtest.me%[1]s git http://gopin.localtest.me%[1]s">`, r.URL.Path)
		cache[meta] = true
		io.WriteString(w, "<head>"+meta+"</head>")
		return
	}

	sc, err := tls.Dial("tcp4", "github.com:443", &tls.Config{
		ServerName: "github.com",
	})
	if err != nil {
		log.Printf("Could not open TLS connection to github.com: %s", err)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	defer sc.Close()

	cc, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Printf("Could not hijack connection: %s", err)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	defer cc.Close()

	r.URL.Host = "github.com"
	r.URL.Path = fmt.Sprintf("/%s/%s%s", p[1], p[2], p[4])
	r.Header.Set("Host", "github.com")
	r.Host = "github.com"
	r.Write(sc)

	go io.Copy(sc, cc)

	scr := io.Reader(sc)
	if p[4] == "/info/refs" {
		scr, err = injectHead(scr, p[3], "master")
		if err != nil {
			log.Printf("Could not inject HEAD: %s", err)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
	}
	go io.Copy(sc, cc)
	io.Copy(cc, scr)
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

func listCacheHandler(c CachedMap) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "<head>")
		for item := range c.Iter() {
			fmt.Fprintf(w, `<meta name="go-import" content="%[1]s%[2]s git http://%[1]s%[3]s">`, r.Host, item.Key, item.Value)
		}
		fmt.Fprintf(w, "</head>")
	})
}
