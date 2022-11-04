package podproxy

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/angelini/fusion/internal/pb"
	"github.com/o1egl/paseto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	PROXY_REQUEST_TIMEOUT = 10 * time.Second
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

type Proxy struct {
	log        *zap.Logger
	namespace  string
	managerUri string
	port       int
	publicKey  ed25519.PublicKey

	httpClient    *http.Client
	managerClient pb.ManagerClient
}

func NewProxy(log *zap.Logger, namespace, managerUri string, port int, publicKey ed25519.PublicKey) (*Proxy, error) {
	connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(connectCtx, managerUri, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("cannot connect to grpc server %v: %w", managerUri, err)
	}

	managerClient := pb.NewManagerClient(conn)

	httpClient := http.Client{
		Timeout: PROXY_REQUEST_TIMEOUT,
	}

	return &Proxy{
		log:        log,
		namespace:  namespace,
		managerUri: managerUri,
		port:       port,
		publicKey:  publicKey,

		httpClient:    &httpClient,
		managerClient: managerClient,
	}, nil
}

func (p *Proxy) Start(ctx context.Context) error {
	http.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		p.log.Info("incoming request", zap.String("url", req.URL.String()), zap.Strings("project", req.Header["X-Fusion-Project"]))

		project, err := readProject(req.Header)
		if err != nil {
			p.httpErr(resp, err, "failed to parse project header")
			return
		}

		valid, err := verifyAuthorization(req.Header, project, p.publicKey)
		if err != nil {
			p.httpErr(resp, err, "failed to validate auth header")
			return
		}
		if !valid {
			resp.WriteHeader(http.StatusForbidden)
			return
		}

		hostname := fmt.Sprintf("s-%d.%s.svc.cluster.local", project, p.namespace)

		_, err = net.LookupIP(hostname)
		if err != nil {
			_, err = p.managerClient.BootSandbox(ctx, &pb.BootSandboxRequest{
				Project: project,
			})
			if err != nil {
				p.httpErr(resp, err, "failed to boot sandbox")
				return
			}
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			p.httpErr(resp, err, "failed to read proxy request body")
			return
		}

		url := fmt.Sprintf("http://%s%s", hostname, req.URL.String())
		proxyReq, err := http.NewRequest(req.Method, url, bytes.NewReader(body))
		if err != nil {
			p.httpErr(resp, err, "failed to create proxy request")
			return
		}

		proxyReq.Header = make(http.Header)
		copyHeader(proxyReq.Header, req.Header, true)

		remoteHost, _, err := net.SplitHostPort(req.RemoteAddr)
		if err == nil {
			appendHostToXForwardHeader(req.Header, remoteHost)
		}

		proxyResp, err := p.httpClient.Do(proxyReq)
		if err != nil {
			p.httpErr(resp, err, "failed to proxy request")
			return
		}
		defer proxyResp.Body.Close()

		copyHeader(resp.Header(), proxyResp.Header, false)
		resp.WriteHeader(proxyResp.StatusCode)
		io.Copy(resp, proxyResp.Body)
	})

	return http.ListenAndServe(":"+strconv.Itoa(p.port), nil)
}

func (p *Proxy) httpErr(resp http.ResponseWriter, err error, message string) {
	p.log.Error(message, zap.Error(err))
	http.Error(resp, err.Error(), http.StatusInternalServerError)
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

func readProject(header http.Header) (int64, error) {
	projects, ok := header["X-Fusion-Project"]
	if !ok || len(projects) == 0 {
		return -1, fmt.Errorf("failed to read project header")
	}

	project, err := strconv.ParseInt(projects[0], 10, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to parse project header %v: %w", projects[0], err)
	}

	return project, nil
}

func verifyAuthorization(header http.Header, project int64, publicKey ed25519.PublicKey) (bool, error) {
	auths, ok := header["Authorization"]
	if !ok || len(auths) == 0 {
		return false, fmt.Errorf("failed to read authorization header")
	}

	reg := regexp.MustCompile("[Bb]earer (.+)")
	matches := reg.FindStringSubmatch(auths[0])
	if len(matches) != 2 {
		return false, fmt.Errorf("invalid authorization header %v", auths[0])
	}

	var payload paseto.JSONToken
	var footer string

	token := matches[1]
	v2 := paseto.NewV2()

	err := v2.Verify(token, publicKey, &payload, &footer)
	if len(matches) != 2 {
		return false, fmt.Errorf("cannot verify authorization header: %w", err)
	}

	return payload.Subject == strconv.FormatInt(project, 10) || payload.Subject == "admin", nil
}
