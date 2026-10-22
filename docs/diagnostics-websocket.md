# Native network diagnostics WebSocket API

The Looking Glass UI uses `GET /session/:session/diagnostics` for Ping, Traceroute, MTR and NextTrace. All four execute inside ALS using Go networking libraries. There is no diagnostic executable lookup, shell, PTY, command argument forwarding or CLI fallback on any supported platform.

The former `/session/:session/shell` endpoint, local `als --shell` entry and fake-shell command forwarding have been removed. `UTILITIES_FAKESHELL` no longer enables a terminal. iPerf3 and Speedtest.net retain their existing fixed-command integrations; they are separate from this diagnostic API.

## Session and connection

Keep `GET /session` (SSE) open and read its `SessionId` event. Use that ID in the WebSocket URL. Closing the SSE session also cancels its diagnostics. Browser connections must have the same origin host as the server; reverse proxies must preserve the external `Host` and forward the WebSocket upgrade. Use `wss:` over HTTPS. Requests without a live session are rejected before upgrade.

Each connection runs one test. Send a text JSON start message within 10 seconds:

```json
{"type":"start","tool":"nexttrace","target":"example.com","family":"auto"}
```

- `tool`: exactly `ping`, `traceroute`, `mtr` or `nexttrace`.
- `target`: a bare IPv4, IPv6 or ASCII/Punycode DNS name.
- `family`: `auto` (default), `4` or `6`. Auto prefers IPv4 for dual-stack domains. IPv4-mapped IPv6 is normalized to IPv4.
- No additional fields, commands, arguments, executable paths, stdin, arbitrary protocols or ports are accepted.

Stop with:

```json
{"type":"stop"}
```

Disconnecting also cancels the test. Cancellation closes sockets, stops probes and joins their workers before releasing the run slot. Every application message after start other than `stop` ends the test with a protocol error.

## Target validation

Frontend and backend enforce the same rules and share fixtures in `ui/src/helper/diagnostic-targets.json`:

- IPv4: four decimal octets, 0–255, without leading zeroes.
- IPv6: full or compressed, including an embedded valid IPv4 suffix; no zones or brackets.
- DNS: at least two labels, each 1–63 ASCII letters/digits/hyphens, with no leading/trailing hyphen. At most 253 characters excluding an optional trailing dot. The last label contains a letter. Internationalized names must use Punycode.
- Whitespace, control characters, URLs, ports, CIDRs, wildcard/underscore labels and CLI flags are rejected.

The backend resolves a validated domain once with a five-second timeout and passes a literal IP to the Go probe engine. These are syntax rules, not a public-address allowlist: private and loopback targets are valid.

## Engines and permissions

| Tool | Implementation | Fixed probe policy |
| --- | --- | --- |
| Ping | Go `golang.org/x/net/icmp` sockets | 10 echo requests, one-second minimum interval, two-second reply deadline |
| Traceroute | NextTrace v1.7.5 `TracerouteWithContext`, GeoIP disabled | ICMP, at most 30 hops and three attempts per hop |
| MTR | NextTrace v1.7.5 `RunMTR` per-hop scheduler, GeoIP disabled | ICMP, at most 30 hops and 10 probes per hop; live statistics |
| NextTrace | NextTrace v1.7.5 `TracerouteWithContext`, official GeoIP provider | ICMP, at most 30 hops and three attempts per hop |

Ping checks the reply type, code, source IP, sequence and a random per-run payload. Raw ICMP also checks the identifier; datagram ping sockets may have their identifier rewritten by the kernel. Unrelated, malformed and stale packets are ignored within the original reply deadline. Ping tries an ICMP datagram socket first, then a raw socket if opening it fails. Send errors terminate the run rather than retrying indefinitely.

**The ALS process needs OS permission to open the required sockets.** On Linux, unprivileged Ping depends on the host's `ping_group_range`; raw ICMP route diagnostics require `CAP_NET_RAW` or equivalent privileges. Permissions on separately installed CLI programs do not grant permissions to ALS. Initialization failures return an API error without terminating the server.

Linux builds with `CGO_ENABLED=0` and requires no diagnostic executables. macOS builds require `CGO_ENABLED=1`, an Apple C toolchain and the system libpcap SDK: the upstream trace package imports libpcap even when ICMP is selected. Release CI builds macOS natively on Intel and ARM64 runners. **There is no macOS command fallback.**

## Server messages

Application messages are JSON text. WebSocket ping control frames are sent every 15 seconds; browsers reply automatically.

```json
{"type":"started","tool":"ping","target":"example.com","address":"192.0.2.1"}
{"type":"row","row":{"seq":1,"host":"192.0.2.1","ttl":56,"latency_ms":12.3}}
{"type":"output","line":"from 192.0.2.1: icmp_seq=1 ttl=56 time=12.30 ms"}
{"type":"done","status":"completed","exit_code":0}
```

| Tool | Row identity | Fields |
| --- | --- | --- |
| Ping | `seq` | `host`, `ttl`, `latency_ms`, or `timeout: true` |
| Traceroute | `hop` + `host` | `rtts_ms`, `timeouts` |
| MTR | `hop` + `host`, inside a complete snapshot | `loss_percent`, `sent`, `last_ms`, `avg_ms`, `best_ms`, `worst_ms`, `stddev_ms` |
| NextTrace | `hop` + `host` | `rtts_ms`, `timeouts`, `asn`, `location`, `owner` when available |

Ping/trace `row` messages upsert a row with the same identity. Multiple responders at the same hop remain separate rows.

MTR sends **complete replacement snapshots**, approximately once per second and at completion. Replace the current MTR table with `rows` (or an empty array if omitted). Do not merge snapshots: a previously unanswered hop can acquire a responder, and a changed path can remove later hops.

```json
{"type":"snapshot","rows":[{"hop":1,"host":"192.0.2.1","loss_percent":0,"sent":2,"last_ms":3,"avg_ms":2,"best_ms":1,"worst_ms":3,"stddev_ms":1}]}
```

MTR latency fields are omitted for rows with no replies. Statistics describe each responder row from the upstream aggregator. NextTrace GeoIP uses its official v3 service, or v4 when the corresponding token is configured. Traceroute and MTR do not contact GeoIP providers.

`output` events contain generated text summaries or errors, never CLI stdout or terminal input. The UI renders them as plain text. `done.status` is `completed`, `stopped`, `timeout` or `error`. Completion does not imply reachability; lost probes are valid measurements. `exit_code` is retained for protocol compatibility: `0` when finished, `-1` on failure/cancellation; no OS child process exists.

```json
{"type":"error","code":"invalid_target","message":"Enter a bare IPv4, IPv6 or ASCII/Punycode domain name."}
```

Errors include `invalid_request`, `invalid_target`, `invalid_family`, `invalid_tool`, `disabled`, `resolve_failed`, `busy`, `execution_failed`, `output_limit` and `protocol_error`. Invalid starts close after the error. Runtime failures send an error followed by `done`. Connection/session capacity failures return HTTP 429 before upgrade; route-engine contention returns `error` with `code: busy` after upgrade.

## Resource limits and configuration

- One connection/test per session; at most four server-wide, including sockets awaiting start. Traceroute/MTR/NextTrace share one route-probe slot to isolate the upstream ICMP identifiers and process-global GeoIP transport. Ping can run alongside a route test.
- A 120-second overall deadline; each route probe waits at most one second, with at most six in flight. Ping has at most 10 sequential two-second reply waits.
- Incoming messages: 1 KiB. Encoded outgoing events: 256 KiB total, 16 KiB per event except MTR snapshots (64 KiB). WebSocket writes time out after five seconds. Identical MTR previews are suppressed.
- NextTrace GeoIP calls are limited to two seconds each; teardown closes the transport and drains active calls before another route test starts. Its CLI signal subscription is released so ALS owns process signals.
- Route tests reject `NEXTTRACE_DEVMODE=true`: the upstream CLI developer mode deliberately panics on some network errors and is unsuitable inside the ALS server.
- `UTILITIES_PING`, `UTILITIES_TRACEROUTE`, `UTILITIES_MTR`, `UTILITIES_NEXTTRACE` default to `true`; setting one to `false` disables that tool in both API and UI.
- The legacy `/method/ping?ip=...` SSE integration uses the same native Ping runner, target validation, session capacity and cancellation rules.
- Docker images no longer build/install a separate NextTrace, iputils, traceroute or MTR package. The fixed-command iPerf3 and Speedtest.net features are retained.

## Browser example

```js
const source = new EventSource('./session')
source.addEventListener('SessionId', ({ data: sessionId }) => {
  const url = new URL(`./session/${encodeURIComponent(sessionId)}/diagnostics`, location.href)
  url.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const socket = new WebSocket(url)
  socket.onopen = () => socket.send(JSON.stringify({
    type: 'start', tool: 'mtr', target: 'example.com', family: '4'
  }))
  socket.onmessage = ({ data }) => {
    const event = JSON.parse(data)
    if (event.type === 'snapshot') console.table(event.rows || [])
    else console.log(event)
  }
  // socket.send(JSON.stringify({ type: 'stop' }))
  // Close source and socket when leaving the page.
})
```
