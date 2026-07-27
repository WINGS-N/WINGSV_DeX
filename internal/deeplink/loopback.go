package deeplink

import (
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// LoopbackPort is the fixed port the callback listener binds on the loopback interface. A web
// page redirects the browser to http://127.0.0.1:<LoopbackPort>/callback?url=<link>. It is
// fixed (not ephemeral) because the redirect URL on the external site is static and cannot be
// told a per-run port. It is deliberately below 32768 to sit outside the Linux ephemeral range
// (32768-60999 default), so the kernel never pre-assigns it as an outbound source port and the
// bind does not fail intermittently. Loopback is reachable by any local process and any site via
// the user's browser, so a delivered link is never applied silently - the app confirms it.
const LoopbackPort = 27310

// StartLoopback binds the callback listener and hands every accepted import link to deliver.
// It returns a stop func. A bind failure (port already held by a foreign process) is returned
// so the caller can log it; the app still runs, only the loopback channel is unavailable.
func StartLoopback(deliver func(link string)) (stop func(), err error) {
	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(LoopbackPort))
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		link := strings.TrimSpace(r.URL.Query().Get("url"))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if !IsImportLink(link) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, pageError)
			return
		}
		if deliver != nil {
			deliver(link)
		}
		_, _ = io.WriteString(w, pageOK)
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}, nil
}

const pageOK = `<!doctype html><html lang="ru"><head><meta charset="utf-8">` +
	`<meta name="viewport" content="width=device-width,initial-scale=1">` +
	`<title>WINGS V</title><style>body{margin:0;height:100vh;display:flex;align-items:center;` +
	`justify-content:center;font-family:system-ui,sans-serif;background:#101014;color:#e8e8ea}` +
	`div{text-align:center;padding:24px}h1{font-size:18px;margin:0 0 8px}p{opacity:.6;margin:0}</style>` +
	`</head><body><div><h1>Ссылка передана в приложение</h1><p>Можно вернуться в WINGS V и закрыть эту вкладку.</p></div></body></html>`

const pageError = `<!doctype html><html lang="ru"><head><meta charset="utf-8"><title>WINGS V</title>` +
	`<style>body{margin:0;height:100vh;display:flex;align-items:center;justify-content:center;` +
	`font-family:system-ui,sans-serif;background:#101014;color:#e8e8ea}div{text-align:center;padding:24px}</style>` +
	`</head><body><div><h1>Неверная ссылка</h1><p>Приложение не распознало ссылку.</p></div></body></html>`
