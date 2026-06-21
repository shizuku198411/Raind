package ingress

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"raind/internal/condenser/store/ism"
	"raind/internal/condenser/store/ssm"
)

type fakeIsmHandler struct {
	ingresses []ism.IngressInfo
}

func (f fakeIsmHandler) StoreIngress(string, ism.IngressInfo) error { return nil }
func (f fakeIsmHandler) GetIngressList() ([]ism.IngressInfo, error) { return f.ingresses, nil }
func (f fakeIsmHandler) GetIngressById(string) (ism.IngressInfo, error) { return ism.IngressInfo{}, nil }
func (f fakeIsmHandler) RemoveIngress(string) error { return nil }
func (f fakeIsmHandler) IsNameAlreadyUsed(string, string) bool { return false }

type fakeSsmHandler struct {
	services []ssm.ServiceInfo
}

func (f fakeSsmHandler) StoreService(string, ssm.ServiceInfo) error { return nil }
func (f fakeSsmHandler) GetServiceList() ([]ssm.ServiceInfo, error) { return f.services, nil }
func (f fakeSsmHandler) GetServiceById(string) (ssm.ServiceInfo, error) { return ssm.ServiceInfo{}, nil }
func (f fakeSsmHandler) RemoveService(string) error { return nil }
func (f fakeSsmHandler) IsNameAlreadyUsed(string, string) bool { return false }

func TestGatewayPreservesExternalHostHeaderForBackend(t *testing.T) {
	var gotHost string
	var gotForwardedHost string
	var gotForwardedProto string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		gotForwardedHost = r.Header.Get("X-Forwarded-Host")
		gotForwardedProto = r.Header.Get("X-Forwarded-Proto")
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer backend.Close()

	backendHost, backendPort, err := net.SplitHostPort(strings.TrimPrefix(backend.URL, "http://"))
	require.NoError(t, err)
	port, err := strconv.Atoi(backendPort)
	require.NoError(t, err)

	g := &Gateway{
		ismHandler: fakeIsmHandler{ingresses: []ism.IngressInfo{{
			Name:      "wordpress",
			Namespace: "wordpress-mysql",
			Rules: []ism.IngressRule{{
				Host: "wordpress.raind.local",
				Paths: []ism.IngressPath{{
					Path:     "/",
					PathType: "Prefix",
					Backend: ism.IngressBackend{ServiceName: "wordpress", ServicePort: port},
				}},
			}},
		}}},
		ssmHandler: fakeSsmHandler{services: []ssm.ServiceInfo{{
			Name:      "wordpress",
			Namespace: "wordpress-mysql",
			Type:      ssm.ServiceTypeClusterIP,
			ClusterIP: backendHost,
			Ports:     []ssm.ServicePort{{Port: port, TargetPort: port, Protocol: "TCP"}},
		}}},
	}

	req := httptest.NewRequest(http.MethodGet, "http://wordpress.raind.local/wp-admin/", nil)
	req.Host = "wordpress.raind.local"
	rr := httptest.NewRecorder()

	g.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "wordpress.raind.local", gotHost)
	require.Equal(t, "wordpress.raind.local", gotForwardedHost)
	require.Equal(t, "http", gotForwardedProto)
}
