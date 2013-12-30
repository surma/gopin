package main

import (
	"net/http"
)

type GoGetRouter struct {
	GoGet, Else http.Handler
}

func (ggr GoGetRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("go-get") == "1" {
		ggr.GoGet(w, r)
		return
	}
	ggr.Else(w, r)
}
