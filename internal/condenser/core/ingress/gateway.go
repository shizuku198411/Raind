package ingress

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"raind/internal/condenser/store/ism"
	"raind/internal/condenser/store/ssm"
	"raind/internal/condenser/utils"
)

const (
	defaultHTTPAddr  = ":7780"
	defaultHTTPSAddr = ":7443"
)

func NewGateway() *Gateway {
	return &Gateway{
		ismHandler: ism.NewIsmManager(ism.NewIsmStore(utils.IsmStorePath)),
		ssmHandler: ssm.NewSsmManager(ssm.NewSsmStore(utils.SsmStorePath)),
		tlsManager: NewTLSManager(),
		httpAddr:   ingressHTTPAddr(),
		httpsAddr:  ingressHTTPSAddr(),
	}
}

type Gateway struct {
	ismHandler ism.IsmHandler
	ssmHandler ssm.SsmHandler
	tlsManager *TLSManager
	httpAddr   string
	httpsAddr  string
}

func (g *Gateway) Start() error {
	errCh := make(chan error, 2)
	started := false

	if g.httpAddr != "" {
		started = true
		go func() {
			log.Printf("[*] ingress http gateway listening on %s", g.httpAddr)
			errCh <- http.ListenAndServe(g.httpAddr, g)
		}()
	}
	if g.httpsAddr != "" {
		started = true
		go func() {
			log.Printf("[*] ingress https gateway listening on %s", g.httpsAddr)
			srv := &http.Server{
				Addr:    g.httpsAddr,
				Handler: g,
				TLSConfig: &tls.Config{
					MinVersion:     tls.VersionTLS12,
					GetCertificate: g.getCertificate,
				},
			}
			errCh <- srv.ListenAndServeTLS("", "")
		}()
	}

	if !started {
		log.Printf("[*] ingress gateway disabled")
		return nil
	}
	return <-errCh
}

func (g *Gateway) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := normalizeHost(hello.ServerName)
	if host == "" {
		return nil, fmt.Errorf("ingress tls SNI is required")
	}
	if !g.isTLSHostEnabled(host) {
		return nil, fmt.Errorf("ingress tls host is not configured: %s", host)
	}
	return g.tlsManager.GetCertificate(hello)
}

func (g *Gateway) isTLSHostEnabled(host string) bool {
	ingresses, err := g.ismHandler.GetIngressList()
	if err != nil {
		return false
	}
	for _, in := range ingresses {
		for _, tlsHost := range in.TLSHosts {
			if normalizeHost(tlsHost) == host {
				return true
			}
		}
	}
	return false
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	match, err := g.resolve(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	target := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(match.clusterIP, strconv.Itoa(match.servicePort)),
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		log.Printf("ingress proxy failed: host=%s path=%s backend=%s err=%v", req.Host, req.URL.Path, target.Host, err)
		http.Error(rw, "ingress backend unavailable", http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, r)
}

type routeMatch struct {
	clusterIP   string
	servicePort int
	pathLen     int
}

func (g *Gateway) resolve(r *http.Request) (routeMatch, error) {
	ingresses, err := g.ismHandler.GetIngressList()
	if err != nil {
		return routeMatch{}, fmt.Errorf("ingress list failed: %w", err)
	}
	services, err := g.ssmHandler.GetServiceList()
	if err != nil {
		return routeMatch{}, fmt.Errorf("service list failed: %w", err)
	}

	host := normalizeHost(r.Host)
	path := r.URL.Path
	if path == "" {
		path = "/"
	}

	var matches []routeMatch
	for _, in := range ingresses {
		for _, rule := range in.Rules {
			if rule.Host != "" && strings.ToLower(rule.Host) != host {
				continue
			}
			for _, p := range rule.Paths {
				if !pathMatches(p, path) {
					continue
				}
				svc, ok := findService(services, in.Namespace, p.Backend.ServiceName)
				if !ok {
					return routeMatch{}, fmt.Errorf("ingress backend service not found: %s/%s", in.Namespace, p.Backend.ServiceName)
				}
				clusterIP := strings.TrimSpace(svc.ClusterIP)
				if clusterIP == "" {
					return routeMatch{}, fmt.Errorf("ingress backend service has no clusterIP: %s/%s", svc.Namespace, svc.Name)
				}
				servicePort, ok := resolveServicePort(svc, p.Backend.ServicePort)
				if !ok {
					return routeMatch{}, fmt.Errorf("ingress backend service port not found: %s/%s:%d", svc.Namespace, svc.Name, p.Backend.ServicePort)
				}
				matches = append(matches, routeMatch{clusterIP: clusterIP, servicePort: servicePort, pathLen: len(p.Path)})
			}
		}
	}
	if len(matches) == 0 {
		return routeMatch{}, fmt.Errorf("ingress route not found")
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].pathLen > matches[j].pathLen })
	return matches[0], nil
}

func ingressHTTPSAddr() string {
	if v := strings.TrimSpace(os.Getenv("RAIND_INGRESS_HTTPS_ADDR")); v != "" {
		return v
	}
	return defaultHTTPSAddr
}

func ingressHTTPAddr() string {
	if v := strings.TrimSpace(os.Getenv("RAIND_INGRESS_HTTP_ADDR")); v != "" {
		return v
	}
	return defaultHTTPAddr
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func pathMatches(rule ism.IngressPath, requestPath string) bool {
	switch rule.PathType {
	case "Exact":
		return requestPath == rule.Path
	case "", "Prefix":
		if rule.Path == "/" {
			return true
		}
		return requestPath == rule.Path || strings.HasPrefix(requestPath, strings.TrimRight(rule.Path, "/")+"/")
	default:
		return false
	}
}

func findService(services []ssm.ServiceInfo, namespace, name string) (ssm.ServiceInfo, bool) {
	for _, svc := range services {
		if svc.Namespace == namespace && svc.Name == name {
			return svc, true
		}
	}
	return ssm.ServiceInfo{}, false
}

func resolveServicePort(svc ssm.ServiceInfo, ingressPort int) (int, bool) {
	for _, p := range svc.Ports {
		if p.Port == ingressPort {
			return p.Port, true
		}
	}
	return 0, false
}
