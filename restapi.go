package main

import (
	"log"
	"net/http"
)

type apiHandler struct {
	root    string
	verbose bool
	SWA     bool
}

func putHandler(w http.ResponseWriter, req *http.Request) {
	// check input
	// get body
	// Put Body
	// Communicate with doxygen, Interface
	// return ok if all okay
	w.WriteHeader(http.StatusUnauthorized)
}

func (api apiHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if api.verbose {
		log.Printf("[%s]: %s", req.Method, req.URL.Path)
	}
	switch req.Method {
	case "GET":
		if api.SWA {
			// TODO validation method
			handler := http.FileServer(http.Dir(api.root))
			handler.ServeHTTP(w, req)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	case "PUT":
		putHandler(w, req)
		return
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}
