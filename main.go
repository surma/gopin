package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

func main() {
	http.Handle("/", http.HandlerFunc(handler))
	err := http.ListenAndServe("localhost:8081", nil)
	if err != nil {
		log.Fatalf("Could not bind to port: %s", err)
	}
}

var (
	knownPattern = regexp.MustCompilePOSIX("^/github.com/([^/]+)/([^/]+)/([0-9a-f]+)(/.+)?$")
)

func handler(w http.ResponseWriter, r *http.Request) {
	p := knownPattern.FindStringSubmatch(r.URL.Path)
	if p == nil || len(p) == 0 {
		http.Error(w, "Not found", http.StatusNotFound)
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
