package main

import (
	"net/http"
)

type Github struct {
	c Cache
	http.Handler
}

func NewGithub(c Cache) *Github {
}
