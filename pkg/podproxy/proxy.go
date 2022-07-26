package podproxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/angelini/fusion/internal/pb"
	"go.uber.org/zap"
)

const (
	NAMESPACE             = "fusion"
	PROXY_REQUEST_TIMEOUT = 10 * time.Second
)

var (
	managerHostname      = fmt.Sprintf("fusion-manager-service.%s.svc.cluster.local:80", NAMESPACE)
	errMissingNameHeader = errors.New("missing header: X-Fusion-Sandbox-Name")
)

// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true, // canonicalized version of "TE"
	"Trailers":            true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

func StartProxy(ctx context.Context, log *zap.Logger, port int) error {
	grpcClient, err := NewClient(ctx, log, managerHostname)
	if err != nil {
		return fmt.Errorf("cannot connect grpc client to %v: %w", managerHostname, err)
	}

	httpClient := &http.Client{
		Timeout: PROXY_REQUEST_TIMEOUT,
	}

	http.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		log.Info("incoming request", zap.String("url", req.URL.String()), zap.Strings("sandbox-name", req.Header["X-Fusion-Sandbox-Name"]))

		sandboxNames, ok := req.Header["X-Fusion-Sandbox-Name"]
		if !ok || len(sandboxNames) == 0 {
			httpErr(log, resp, errMissingNameHeader, "failed to read sandbox name")
			return
		}

		sandboxName := sandboxNames[0]
		hostname := fmt.Sprintf("%s.%s.svc.cluster.local", sandboxName, NAMESPACE)

		_, err := net.LookupIP(hostname)
		if err != nil {
			_, err = grpcClient.BootSandbox(ctx, &pb.BootSandboxRequest{
				Key: sandboxName,
			})
			if err != nil {
				httpErr(log, resp, err, "failed to boot sandbox")
				return
			}
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			httpErr(log, resp, err, "failed to read proxy request body")
			return
		}

		url := fmt.Sprintf("http://%s%s", hostname, req.URL.String())
		proxyReq, err := http.NewRequest(req.Method, url, bytes.NewReader(body))
		if err != nil {
			httpErr(log, resp, err, "failed to create proxy request")
			return
		}

		proxyReq.Header = make(http.Header)
		copyHeader(proxyReq.Header, req.Header, true)

		remoteHost, _, err := net.SplitHostPort(req.RemoteAddr)
		if err == nil {
			appendHostToXForwardHeader(req.Header, remoteHost)
		}

		proxyResp, err := httpClient.Do(proxyReq)
		if err != nil {
			httpErr(log, resp, err, "failed to proxy request")
			return
		}
		defer proxyResp.Body.Close()

		copyHeader(resp.Header(), proxyResp.Header, false)
		resp.WriteHeader(proxyResp.StatusCode)
		io.Copy(resp, proxyResp.Body)
	})

	return http.ListenAndServe(":"+strconv.Itoa(port), nil)
}

func copyHeader(dest, src http.Header, skipHopHeaders bool) {
	for key, value := range src {
		if skipHopHeaders {
			if _, ok := hopHeaders[key]; ok {
				continue
			}
		}

		for _, nested := range value {
			dest.Add(key, nested)
		}
	}
}

func appendHostToXForwardHeader(header http.Header, host string) {
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

func httpErr(log *zap.Logger, resp http.ResponseWriter, err error, message string) {
	log.Error(message, zap.Error(err))
	http.Error(resp, err.Error(), http.StatusInternalServerError)
}
