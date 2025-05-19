package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	ListenAddr   string   `json:"listenAddr"`
	UnixSockAddr string   `json:"unixSockAddr"`
	TrustSources []string `json:"trustSources"`
}

type DERPAdmitClientRequest struct {
	NodePublic string     // key to query for admission
	Source     netip.Addr // derp client's IP address
}

type DERPAdmitClientResponse struct {
	Allow bool
}

var client *http.Client

func main() {
	var flagHelp = flag.Bool("flagHelp", false, "Show flagHelp")
	var flagConfig = flag.String("config", "./config.json", "Config file")
	flag.Parse()
	if *flagHelp {
		flag.Usage()
		return
	}
	config := &Config{
		ListenAddr:   "127.0.0.1:8081",
		UnixSockAddr: "/var/run/tailscale/tailscaled.sock",
		TrustSources: []string{"http://local-tailscaled.sock/localapi/v0/whois?addr={nodekey}"},
	}
	loadConfig(config, *flagConfig)
	client = &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
				if strings.HasPrefix(addr, "local-tailscaled.sock:") {
					return net.Dial("unix", config.UnixSockAddr)
				}
				return net.Dial(network, addr)
			},
		},
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var buf = bytes.NewBuffer(nil)
		var req DERPAdmitClientRequest
		err := json.NewDecoder(io.TeeReader(io.LimitReader(r.Body, 1<<13), buf)).Decode(&req)
		if err != nil {
			log.Println("Error while parsing request: ", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var b []byte
		ok := false
		for i := range config.TrustSources {
			if checkNodeKey(config.TrustSources[i], req.NodePublic) {
				ok = true
				log.Println("Granted DERP access for ", req.NodePublic, " from ", req.Source, ".")
				break
			}
		}
		b, err = json.Marshal(&DERPAdmitClientResponse{Allow: ok})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		if !ok {
			log.Println(req.NodePublic, "has been denied for DERP access from all nodes.")
		}
		_, _ = w.Write(b)
	})
	log.Println("Listening on ", config.ListenAddr)
	err := http.ListenAndServe(config.ListenAddr, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func checkNodeKey(_url string, nodekey string) bool {
	_url = strings.ReplaceAll(_url, "{nodekey}", url.QueryEscape(nodekey))
	u, _ := url.Parse(_url)
	request := http.Request{
		Method: "GET",
		Header: http.Header{
			"X-Tailscale-Reason": []string{""},
		},
		URL: u,
	}
	request.Host = u.Host
	resp, err := client.Do(&request)
	return err == nil && resp.StatusCode == 200
}

func loadConfig(config *Config, path string) {
	var err error
	if _, err = os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			result, err := json.MarshalIndent(config, "", "  ")
			if err != nil {
				panic(err)
			}
			os.WriteFile(path, result, 0644)
			log.Println("Created config file: ", path)
			return
		}
		log.Fatal("Cannot call stat on config: ", err)
	}
	var configContent []byte
	if configContent, err = os.ReadFile(path); err != nil {
		log.Fatal("Cannot read config: ", err)
	}
	if err = json.Unmarshal(configContent, config); err != nil {
		log.Fatal("Cannot parse config: ", err)
	}
}
