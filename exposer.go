package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// I encountered some error when building, so I decided to duplicate some logics from share.go
// to make them independent.

func main() {
	var flagListenAddr = flag.String("listen-addr", ":8081", "Listen address")
	var socketAddr = flag.String("socket-addr", "/var/run/tailscale/tailscaled.sock", "Tailscaled socket address")
	var flagSecretKey = flag.String("secret-key", "", "Secret key")
	var flagHelp = flag.Bool("flagHelp", false, "Show flagHelp")
	flag.Parse()
	if *flagHelp {
		flag.Usage()
		return
	}
	_client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
				if strings.HasPrefix(addr, "local-tailscaled.sock:") {
					return net.Dial("unix", *socketAddr)
				}
				return net.Dial(network, addr)
			},
		},
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		uri, err := url.Parse(r.URL.String())
		query := uri.Query()
		if err != nil {
			log.Println("Error while parsing query: ", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		secret := query.Get("secret")
		if secret != *flagSecretKey {
			log.Printf("Invalid secret: %s", secret)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		nodeKey := query.Get("nodekey")
		if nodeKey == "" {
			log.Printf("Nodekey is empty!")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if _checkNodeKey(_client, nodeKey) {
			log.Printf("Access granted for %s", nodeKey)
			w.WriteHeader(http.StatusOK)
			return
		}
		log.Printf("Invalid nodeKey: %s", nodeKey)
		w.WriteHeader(http.StatusForbidden)
	})
	log.Println("Listening on ", *flagListenAddr)
	err := http.ListenAndServe(*flagListenAddr, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func _checkNodeKey(_client *http.Client, nodekey string) bool {
	u, _ := url.Parse("http://local-tailscaled.sock/localapi/v0/whois?addr=" + url.QueryEscape(nodekey))
	request := http.Request{
		Method: "GET",
		Header: http.Header{
			"X-Tailscale-Reason": []string{""},
		},
		URL: u,
	}
	request.Host = "local-tailscaled.sock"
	resp, err := _client.Do(&request)
	return err == nil && resp.StatusCode == 200
}
