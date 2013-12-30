package main

import (
	"bufio"
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
	log.Printf(">> %s: %s | %s", r.Method, r.URL.Path, r.URL.RawQuery)
	p := knownPattern.FindStringSubmatch(r.URL.Path)
	if p == nil || len(p) == 0 {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if p[4] == "/info/refs" {
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

		br := bufio.NewReader(sc)
		found := false
		for !found {
			line, err := br.ReadString('\n')
			if err != nil {
				log.Printf("Could not read line: %s", err)
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}
			if strings.HasSuffix(line, "refs/heads/master\n") {
				log.Printf("Injecting new master HEAD")
				fmt.Fprintf(cc, "%s%s refs/heads/master\n", line[0:4], p[3])
				found = true
				continue
			}
			cc.Write([]byte(line))
		}
		go io.Copy(sc, cc)
		io.Copy(cc, br)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("https://github.com/%s/%s%s?%s", p[1], p[2], p[4], r.URL.RawQuery), http.StatusTemporaryRedirect)
}
