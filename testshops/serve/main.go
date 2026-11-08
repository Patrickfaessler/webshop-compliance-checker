// Command serve serves the generated test shops (../site) on 127.0.0.1 only, so they can be visited
// and scanned locally. Spec: specs/09-testing.md (T-3), scanning needs ALLOW_LOOPBACK_TARGETS=1 (C-SSRF-DEV).
//
//	cd testshops/serve; go run . [-addr 127.0.0.1:5500] [-dir ../site]
package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:5500", "listen address (keep it on loopback)")
	dir := flag.String("dir", "../site", "folder with the generated shops")
	flag.Parse()
	log.Printf("test shops: http://%s/  (teehaus/, radwerk/, seifen/)", *addr)
	log.Fatal(http.ListenAndServe(*addr, http.FileServer(http.Dir(*dir))))
}
