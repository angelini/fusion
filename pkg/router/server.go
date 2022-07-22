package router

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"
)

const (
	PROXY_REQUEST_TIMEOUT = 5 * time.Second
)

func StartServer(ctx context.Context, log *zap.Logger, port int) error {
	client := &http.Client{
		Timeout: PROXY_REQUEST_TIMEOUT,
	}

	http.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			httpErr(log, resp, err, "failed to read proxy request body")
			return
		}

		// FIXME
		url := "127.0.0.1:3333"

		proxyReq, err := http.NewRequest(req.Method, url, bytes.NewReader(body))
		if err != nil {
			httpErr(log, resp, err, "failed to create proxy request")
			return
		}

		proxyReq.Header = make(http.Header)
		copyHeader(proxyReq.Header, req.Header)

		proxyResp, err := client.Do(proxyReq)
		if err != nil {
			httpErr(log, resp, err, "failed to proxy request")
			return
		}
		defer proxyResp.Body.Close()

		copyHeader(resp.Header(), proxyResp.Header)
		io.Copy(resp, proxyResp.Body)
	})

	return http.ListenAndServe("127.0.0.1:"+strconv.Itoa(port), nil)
}

func copyHeader(dest, src http.Header) {
	for key, value := range src {
		for _, nested := range value {
			dest.Add(key, nested)
		}
	}
}

func httpErr(log *zap.Logger, resp http.ResponseWriter, err error, message string) {
	log.Error(message, zap.Error(err))
	http.Error(resp, err.Error(), http.StatusInternalServerError)
}
