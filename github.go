package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"regexp"
)

type Github struct {
	cache Cache
}

func NewGithub(c Cache) *Github {
	return &Github{
		cache: c,
	}
}

var (
	pathMatcher = regexp.MustCompile("^/([^/]+/[^/]+)/([0-9a-fA-F]{40})(/.*)?$")
)

func (g *Github) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := pathMatcher.FindStringSubmatch(r.URL.Path)
	if parts == nil {
		log.Printf("Regexp mismatch")
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	importPath, repo, hash, subpath := "github.com"+parts[0], parts[1], parts[2], parts[3]
	if subpath == "" {
		subpath = "/"
	}

	// If it's a HTTP request by `go get`, generate HTML file
	// containing the appropriate meta headers.
	if r.URL.Query().Get("go-get") == "1" {
		ci := CacheItem{
			ImportPath: importPath,
			RepoUrl:    path.Join("/github.com", repo, hash),
		}
		g.cache.Add(ci)
		RenderSingleGoImportMeta(w, r, ci)
		return
	}

	// Any request will be proxied to Github in th end. Only a request
	// for `/info/refs` will experience some initial manipulation.

	// Open connection to Github webserver.
	sc, err := tls.Dial("tcp4", "github.com:443", &tls.Config{
		ServerName: "github.com",
	})
	if err != nil {
		log.Printf("Could not open TLS connection to github.com: %s", err)
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	defer sc.Close()

	// Get raw connection of incoming request.
	cc, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Printf("Could not hijack connection: %s", err)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	defer cc.Close()

	// Manipulate request to look like it has been directed at
	// Github.com directly.
	r.URL.Host = "github.com"
	r.URL.Path = fmt.Sprintf("/%s/%s", repo, subpath)
	r.Header.Set("Host", "github.com")
	r.Host = "github.com"
	r.Write(sc)

	// Forward client data to Github unchanged.
	go io.Copy(sc, cc)

	scr := io.Reader(sc)
	if subpath == "/info/refs" {
		scr, err = injectHead(scr, hash, "master")
		if err != nil {
			log.Printf("Could not inject HEAD: %s", err)
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
	}
	io.Copy(cc, scr)
}
