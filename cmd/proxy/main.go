// Tiny reverse proxy in front of wasmserve.
//
// wasmserve fetches wasm_exec.js from go.googlesource.com on first request,
// which fails in air-gapped or geo-restricted environments. This proxy
// listens on the public port and intercepts /wasm_exec.js to serve a local
// copy; everything else is forwarded to the wasmserve instance bound to an
// internal port.
package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	listen := flag.String("listen", ":7426", "external HTTP bind address")
	upstream := flag.String("upstream", "http://127.0.0.1:17426", "internal wasmserve address")
	wasmExec := flag.String("wasm-exec", "", "absolute path to a local wasm_exec.js (optional)")
	flag.Parse()

	u, err := url.Parse(*upstream)
	if err != nil {
		log.Fatalf("invalid upstream: %v", err)
	}
	rp := httputil.NewSingleHostReverseProxy(u)

	mux := http.NewServeMux()
	if *wasmExec != "" {
		mux.HandleFunc("/wasm_exec.js", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, r, *wasmExec)
		})
	}
	mux.Handle("/", rp)

	log.Printf("proxy listening on %s -> %s (wasm_exec.js override: %q)", *listen, *upstream, *wasmExec)
	if err := http.ListenAndServe(*listen, mux); err != nil {
		log.Fatal(err)
	}
}
