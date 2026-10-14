// Package chunkwriter implements a Caddy HTTP handler that splits every
// response body write into fixed-size pieces and flushes after each one, so
// each piece becomes exactly one TLS record on the wire instead of whatever
// size upstream/proxy buffering happens to produce.
package chunkwriter

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddy.RegisterModule(ChunkWriter{})
	httpcaddyfile.RegisterHandlerDirective("chunk_writer", parseCaddyfile)
}

// ChunkWriter is a Caddy HTTP middleware. Caddyfile usage:
//
//	chunk_writer <size_in_bytes>
type ChunkWriter struct {
	ChunkSize int `json:"chunk_size,omitempty"`
}

func (ChunkWriter) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.chunk_writer",
		New: func() caddy.Module { return new(ChunkWriter) },
	}
}

func (cw *ChunkWriter) Provision(_ caddy.Context) error {
	if cw.ChunkSize <= 0 {
		cw.ChunkSize = 2048
	}
	return nil
}

func (cw *ChunkWriter) Validate() error {
	if cw.ChunkSize <= 0 {
		return fmt.Errorf("chunk_size must be a positive number of bytes, got %d", cw.ChunkSize)
	}
	return nil
}

func (cw *ChunkWriter) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	return next.ServeHTTP(&chunkedWriter{
		ResponseWriter: w,
		flush:          http.NewResponseController(w).Flush,
		size:           cw.ChunkSize,
	}, r)
}

// UnmarshalCaddyfile parses: chunk_writer [<size_in_bytes>]
func (cw *ChunkWriter) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		if d.NextArg() {
			size, err := strconv.Atoi(d.Val())
			if err != nil {
				return d.Errf("invalid chunk size %q: %v", d.Val(), err)
			}
			cw.ChunkSize = size
		}
		if d.NextArg() {
			return d.ArgErr()
		}
	}
	return nil
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	cw := new(ChunkWriter)
	err := cw.UnmarshalCaddyfile(h.Dispenser)
	return cw, err
}

type chunkedWriter struct {
	http.ResponseWriter
	flush func() error
	size  int
}

// Write splits p into fixed-size slices and flushes after each one, so each
// slice becomes its own TLS record instead of being re-batched by Go's
// internal response buffering (which defeats a plain http.Flusher type
// assertion when w is wrapped, e.g. by Caddy's own middleware).
func (cw *chunkedWriter) Write(p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		n := cw.size
		if n > len(p) {
			n = len(p)
		}
		written, err := cw.ResponseWriter.Write(p[:n])
		total += written
		if err != nil {
			return total, err
		}
		if err := cw.flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
			return total, err
		}
		p = p[n:]
	}
	return total, nil
}

var (
	_ caddy.Provisioner          = (*ChunkWriter)(nil)
	_ caddy.Validator            = (*ChunkWriter)(nil)
	_ caddyhttp.MiddlewareHandler = (*ChunkWriter)(nil)
	_ caddyfile.Unmarshaler      = (*ChunkWriter)(nil)
)
