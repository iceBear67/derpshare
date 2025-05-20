package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"tailscale.com/safesocket"
)

type Exposer struct {
	client *http.Client
}

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
	exposer := &Exposer{
		client: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
					if strings.HasPrefix(addr, "local-tailscaled.sock:") {
						return safesocket.ConnectContext(ctx, *socketAddr)
					}
					return net.Dial(network, addr)
				},
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
		if exposer.checkNodeKey(nodeKey) {
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

func (e *Exposer) checkNodeKey(nodekey string) bool {
	u, _ := url.Parse("http://local-tailscaled.sock/localapi/v0/whois?addr=" + url.QueryEscape(nodekey))
	request := http.Request{
		Method: "GET",
		Header: http.Header{
			"X-Tailscale-Reason": []string{""},
		},
		URL: u,
	}
	request.Host = "local-tailscaled.sock"
	resp, err := e.client.Do(&request)
	return err == nil && resp.StatusCode == 200
}

// Taken from https://github.com/tailscale/tailscale/blob/main/paths/paths.go#L23
// BSD-3 License
// DefaultTailscaledSocket returns the path to the tailscaled Unix socket
// or the empty string if there's no reasonable default.
func (e *Exposer) DefaultTailscaledSocket() string {
	if runtime.GOOS == "windows" {
		return `\\.\pipe\ProtectedPrefix\Administrators\Tailscale\tailscaled`
	}
	if runtime.GOOS == "darwin" {
		return "/var/run/tailscaled.socket"
	}
	if runtime.GOOS == "plan9" {
		return "/srv/tailscaled.sock"
	}
	if fi, err := os.Stat("/var/run"); err == nil && fi.IsDir() {
		return "/var/run/tailscale/tailscaled.sock"
	}
	return "tailscaled.sock"
}
