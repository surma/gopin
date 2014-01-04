package main

import (
	"fmt"
	"net/http"
)

func RenderGoImport(w http.ResponseWriter, r *http.Request, c <-chan CacheItem) {
	fmt.Fprintf(w, "<head>")
	for item := range c {
		fmt.Fprintf(w, `<meta name="go-import" content="%[1]s/%[2]s git http://%[1]s%[3]s">`, r.Host, item.ImportPath, item.RepoUrl)
	}
	fmt.Fprintf(w, "</head>")
}

func RenderSingleGoImport(w http.ResponseWriter, r *http.Request, ci CacheItem) {
	c := make(chan CacheItem)
	go func() {
		c <- ci
		close(c)
	}()
	RenderGoImport(w, r, c)
}
