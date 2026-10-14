# caddy-plugins

Small Caddy HTTP plugins. One Go module, one subdirectory per plugin — each
plugin is its own Go package and its own Caddy module, built into a custom
Caddy binary with `xcaddy`.

## Plugins

### chunkwriter

**Problem it solves:** some clients, such as embedded devices with a cellular
modem that terminates TLS on its own, have very little RAM for incoming data.
If the server sends large TLS records, the client's receive buffer gets
overwhelmed and the download stalls or fails.

Caddy terminates TLS with Go's standard `crypto/tls`, which has no option to
cap the record size below its maximum (~16 KB). Lowering the backend's own
chunk size does not reliably help: Caddy's reverse proxy re-buffers the
upstream response before writing it to the TLS connection, so small chunks
get coalesced back toward the maximum. `reverse_proxy`'s `flush_interval` does
not help either, because it is time-based rather than size-based.

**What it does:** wraps the `http.ResponseWriter` and splits every `Write()`
call into fixed-size slices, flushing after each one. Since `crypto/tls` maps
one `Write()` call to one TLS record, each slice becomes one TLS record of
that size, regardless of upstream buffering or timing.

**Caddyfile usage:**

```caddyfile
chunk_writer <size_in_bytes>
```

Place it before `reverse_proxy` inside a `route` block, so it wraps the
response writer that the proxy writes into:

```caddyfile
example.com {
	route {
		chunk_writer 2048
		reverse_proxy upstream:5000
	}
}
```

If the size is omitted, it defaults to 2048 bytes. Small values trade
throughput for smaller records: each slice is a separate write and flush.

## Building a Caddy binary

```dockerfile
# syntax=docker/dockerfile:1
FROM caddy:builder-alpine AS builder

RUN xcaddy build \
  --with github.com/oscarb-fossa/caddy-plugins/chunkwriter@<commit-or-tag>

FROM caddy:latest
COPY --from=builder /usr/bin/caddy /usr/bin/caddy
```

Add one `--with` line per plugin you want. Pin to a commit SHA or tag, not a
moving branch, so builds stay reproducible.

## Development

```bash
go mod tidy
go build ./...
```

## License

MIT, see [LICENSE](LICENSE).
