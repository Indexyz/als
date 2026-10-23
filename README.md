[![docker image build](https://github.com/Indexyz/als/actions/workflows/docker-image.yml/badge.svg)](https://github.com/Indexyz/als/actions/workflows/docker-image.yml)


Language: English | [简体中文](README_zh_CN.md)

# ALS - Another Looking-glass Server

## Quick start
```
docker run -d --name looking-glass --restart always --network host ghcr.io/indexyz/als:latest
```

Images support `linux/amd64` and `linux/arm64`. `latest` tracks the default branch (`master`) and is updated by pushes, scheduled builds, and manual runs on that branch. Git tag builds publish the matching image tag without updating `latest`; use a published tag or digest to pin a deployment. Pull requests build the image without publishing it.

The image workflow publishes to `ghcr.io/<repository-owner>/<repository-name>` (lowercase) using `GITHUB_TOKEN` with `packages: write`; Docker Hub credentials are no longer needed. After the first successful publication, set the GHCR package visibility to **Public** to allow anonymous pulls. Forks should use their own image path.

Local builds use Node.js 24.22.1 and Go 1.27.3; see [UI build instructions](ui/README.md). Both Dockerfiles install frontend dependencies from the committed npm lockfile.

[DEMO](http://lg.hk1-bgp.hkg.50network.com/)

If you don't want to use Docker , you can use the [compiled server](https://github.com/Indexyz/als/releases)

## Host Requirements
 - RAM: 32MB or more

## How to change config
```
# you need pass -e KEY=VALUE to docker command
# you can find the KEY below the [Image Environment Variables]
# for example, change the listen port to 8080
docker run -d \
    --name looking-glass \
    -e HTTP_PORT=8080 \
    --restart always \
    --network host \
    ghcr.io/indexyz/als:latest
``` 

## Environment variable table
| Key                       | Example                                                                | Default                                                    | Description                                                                             |
| ------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| LISTEN_IP                 | 127.0.0.1                                                              | (all ip)                                                   | which IP address will be listen use                                                     |
| HTTP_PORT                 | 80                                                                     | 80                                                         | which HTTP port should use                                                              |
| SPEEDTEST_FILE_LIST       | 100MB 1GB                                                              | 1MB 10MB 100MB 1GB                                         | size of static test files, separate with space                                          |
| LOCATION                  | "this is location"                                                     | (request from http://ipapi.co) | location string                                                                         |
| PUBLIC_IPV4               | 1.1.1.1                                                                | (fetch from http://ifconfig.co)                            | The IPv4 address of the server                                                          |
| PUBLIC_IPV6               | fe80::1                                                                | (fetch from http://ifconfig.co)                            | The IPv6 address of the server                                                          |
| DISPLAY_TRAFFIC           | true                                                                   | true                                                       | Toggle the streaming traffic graph                                                      |
| ENABLE_SPEEDTEST          | true                                                                   | true                                                       | Toggle the speedtest feature                                                            |
| UTILITIES_PING            | true                                                                   | true                                                       | Toggle the ping feature                                                                 |
| UTILITIES_SPEEDTESTDOTNET | true                                                                   | true                                                       | Toggle the speedtest.net feature                                                        |
| UTILITIES_TRACEROUTE       | true | true | Toggle Traceroute diagnostics |
| UTILITIES_MTR              | true | true | Toggle MTR diagnostics |
| UTILITIES_NEXTTRACE        | true | true | Toggle NextTrace diagnostics |
| UTILITIES_IPERF3          | true                                                                   | true                                                       | Toggle the iperf3 feature                                                               |
| UTILITIES_IPERF3_PORT_MIN | 30000                                                                  | 30000                                                      | iperf3 listen port range - from                                                         |
| UTILITIES_IPERF3_PORT_MAX | 31000                                                                  | 31000                                                      | iperf3 listen port range - to                                                           |
| SPONSOR_MESSAGE           | "Test message" or "/tmp/als_readme.md" or "http://some_host/114514.md" | ''                                                         | Show server sponsor message (support markdown file, required mapping file to container) |


## Features
- [x] HTML 5 Speed Test
- [x] Ping - IPv4 / IPv6
- [x] iPerf3 server
- [x] Streaming traffic graph
- [x] Speedtest.net Client
- [x] Native Ping / Traceroute / MTR / NextTrace UI with a restricted WebSocket API

Ping, Traceroute, MTR and NextTrace now run directly through Go networking libraries. The web terminal, `/session/:session/shell`, local `als --shell` entry and fake-shell command forwarding have been removed; `UTILITIES_FAKESHELL` has no effect. Both frontend and backend accept only valid IPv4, IPv6 or ASCII/Punycode DNS names, with server-owned probe limits.

Ping uses Go ICMP sockets and verifies the reply source, sequence and random payload. Traceroute/MTR/NextTrace import `github.com/nxtrace/NTrace-core/trace` v1.7.5. MTR streams complete statistics snapshots with at most 10 probes per hop. Route tests share one slot while Ping can run alongside them. ALS needs the corresponding socket permissions. Linux supports static builds; macOS requires CGO and system libpcap and is built natively in release CI, with no CLI fallback. iPerf3 and Speedtest.net retain their existing fixed-command integrations. See the [WebSocket protocol and deployment requirements](docs/diagnostics-websocket.md).

## License

Code is licensed under MIT Public License.

* If you wish to support my efforts, keep the "Powered by WIKIHOST Opensource - ALS" link intact.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=Indexyz/als&type=Date)](https://star-history.com/#Indexyz/als&Date)
