# DERPShare
A simple Proof-Of-Concept server for sharing DERP servers among multiple tailnets.    
Thanks to tailscale/safesocket, Linux, Windows and Darwin are supported.

## Usage

To begin with, your DERP node _must_ support the `--verify-client-url` option, which can be checked in derpers' help message. (`-h` option)  

Once `share.go` is built and ran at first time, a configuration named `config.json` will be automatically generated.
```json
{
  "listenAddr": "127.0.0.1:8081",
  "unixSockAddr": "/var/run/tailscale/tailscaled.sock",
  "trustSources": [
    "http://local-tailscaled.sock/localapi/v0/whois?addr={nodekey}"
  ]
}
```
Usually, you don't need to modify the `unixSockAddr` option. The default one should be usable on Windows and most Linux distros.
(for some distros, like synology) If not, you can find the correct one by [checking tailscale's source](https://github.com/tailscale/tailscale/blob/3cc80cce6ac045c64a410ae19d86d8100b567a26/paths/paths.go#L23)

Then, (re)start your DERP server with these options:
```
$ derper <... other args> \
  --verify-clients=false \
  --verify-client-url-fail-open=false \
  --verify-client-url=http://127.0.0.1:8081
```

Be sure to check if your node is still usable in your tailnet. By default, derpshare simply forwards DERP's nodekey authentication request to your local tailscaled, and it should just act like `-verify-clients=true` option.

In this configuration, your DERP node is only accessible to nodes from your tailnet. To trust nodes from other tailnets, you need to provide trust source address accordingly.

### Exposer

`exposer.go` is a simple server that implements nodekey authentication by forwarding the request to tailscaled but won't respond any node information like what tailscaled will do.

```
go run ./exposer.go -h
Usage of exposer:
  -flagHelp
        Show flagHelp
  -listen-addr string
        Listen address (default ":8081")
  -secret-key string
        Secret key
  -socket-addr string
        Tailscaled socket address (default "/var/run/tailscale/tailscaled.sock")
```

Example setup:

You're Alice. And Bob wants to use your DERP server. Both of them have tailscaled intalled on their machines.
```
(Bob) $ nohup ./exposer -secret-key 114514 &
(Alice) $ cat ./config.json
{
  "listenAddr": ":8081",
  "unixSockAddr": "/var/run/tailscale/tailscaled.sock",
  "trustSources": [
    "http://local-tailscaled.sock/localapi/v0/whois?addr={nodekey}"
    "http://bob:8081/?nodekey={nodekey}&secret=114514
  ]
}
(Alice) $ nohup ./share &
(Alice) $ ./derper --verify-client-url=http://127.0.0.1:8081 --verify-clients=false --verify-client-url-fail-open=false
```

And that's done. In RL scenarios, you may want to run these daemons as systemd services or in GNU Screen.  
Enjoy