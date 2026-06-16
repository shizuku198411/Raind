package http

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	bottleHandler "raind/internal/condenser/api/http/bottle"
	certHandler "raind/internal/condenser/api/http/cert"
	containerHandler "raind/internal/condenser/api/http/container"
	hookHandler "raind/internal/condenser/api/http/hook"
	imageHandler "raind/internal/condenser/api/http/image"
	ingressHandler "raind/internal/condenser/api/http/ingress"
	"raind/internal/condenser/api/http/logger"
	logHandler "raind/internal/condenser/api/http/logs"
	namespaceHandler "raind/internal/condenser/api/http/namespace"
	networkHandler "raind/internal/condenser/api/http/network"
	podHandler "raind/internal/condenser/api/http/pod"
	policyHandler "raind/internal/condenser/api/http/policy"
	securityProfileHandler "raind/internal/condenser/api/http/securityprofile"
	serviceHandler "raind/internal/condenser/api/http/service"
	websocketHandler "raind/internal/condenser/api/http/websocket"
	_ "raind/internal/condenser/docs"
	"raind/internal/condenser/utils"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"

	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// @title Condenser API
// @version 1.0
// @description High-level container runtime API for Raind stack
// @BasePath /
// @schemes http

func NewSwaggerRouter() *chi.Mux {
	r := chi.NewRouter()

	// middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// == swagger ==
	r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))

	return r
}

func NewApiRouter() *chi.Mux {
	r := chi.NewRouter()
	containerHandler := containerHandler.NewRequestHandler()
	bottleHandler := bottleHandler.NewRequestHandler()
	imageHandler := imageHandler.NewRequestHandler()
	networkHandler := networkHandler.NewRequestHandler()
	socketHandler := websocketHandler.NewRequestHandler()
	execSocketHandler := websocketHandler.NewExecRequestHandler()
	policyHandler := policyHandler.NewRequestHandler()
	logHandler := logHandler.NewRequestHandler()
	podHandler := podHandler.NewRequestHandler()
	serviceHandler := serviceHandler.NewRequestHandler()
	namespaceHandler := namespaceHandler.NewRequestHandler()
	ingressHandler := ingressHandler.NewRequestHandler()
	securityProfileHandler := securityProfileHandler.NewRequestHandler()

	// middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// LOGGER
	node, _ := os.Hostname()
	r.Use(logger.LoggerMiddleware(
		logger.JsonLineLogger{Out: openAuditLog()},
		"condenser",
		node,
	))
	// SPIFFE
	r.Use(RequireCLIIdentity)

	// == v1 ==
	// == bottles ==
	r.With(RequireCLIScope(ScopeBottleWrite)).Post("/v1/bottle", bottleHandler.RegisterBottle)     // register bottle
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/bottle", bottleHandler.GetBottleList)              // get bottle list
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/bottle/{bottleId}", bottleHandler.GetBottleDetail) // get bottle detail
	r.With(RequireCLIScope(ScopeBottleWrite)).Post("/v1/bottle/{bottleId}/actions/{action}", bottleHandler.ActionBottle)

	// == containers ==
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/containers", containerHandler.GetContainerList)                                      // get container list
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/containers/{containerId}", containerHandler.GetContainerById)                        // get container status by id
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/containers/{containerId}/log", containerHandler.GetContainerLog)                     // get container log
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/containers/{containerId}/logpath", containerHandler.GetContainerLogPath)             // get container log path
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/containers/{containerId}/stats", containerHandler.GetContainerStats)                 // get container stats
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/containers/{containerId}/spec", containerHandler.GetContainerSpec)                   // get container config spec
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/containers/stats", containerHandler.ListContainerStats)                              // get container stats list
	r.With(RequireCLIScope(ScopeContainerWrite)).Post("/v1/containers", containerHandler.CreateContainer)                            // create container
	r.With(RequireCLIScope(ScopeContainerWrite)).Post("/v1/containers/{containerId}/actions/start", containerHandler.StartContainer) // start container
	r.With(RequireCLIScope(ScopeContainerWrite)).Post("/v1/containers/{containerId}/actions/stop", containerHandler.StopContainer)   // stop container
	r.With(RequireCLIScope(ScopeContainerExec)).Post("/v1/containers/{containerId}/actions/exec", containerHandler.ExecContainer)    // exec container
	r.With(RequireCLIScope(ScopeContainerWrite)).Delete("/v1/containers/{containerId}/actions/delete", containerHandler.DeleteContainer)

	// == security profiles ==
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/security/profiles", securityProfileHandler.ListSecurityProfiles)
	r.With(RequireCLIScope(ScopeContainerWrite)).Post("/v1/security/profiles", securityProfileHandler.RegisterSecurityProfile)
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/security/profiles/{name}", securityProfileHandler.ShowSecurityProfile)
	r.With(RequireCLIScope(ScopeContainerWrite)).Delete("/v1/security/profiles/{name}", securityProfileHandler.DeleteSecurityProfile)

	// == resource ==
	r.With(RequireCLIScope(ScopeResourceWrite)).Post("/v1/resource/apply", podHandler.ApplyPodYaml) // apply yaml
	r.With(RequireCLIScope(ScopeResourceWrite)).Post("/v1/resource/delete", podHandler.DeleteResourceYaml)

	// == pods ==
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/pods", podHandler.GetPodList)                               // list pods
	r.With(RequireCLIScope(ScopeResourceWrite)).Post("/v1/pods", podHandler.CreatePod)                      // create pod sandbox
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/pods/{podId}", podHandler.GetPodById)                       // get pod sandbox detail
	r.With(RequireCLIScope(ScopeResourceWrite)).Post("/v1/pods/{podId}/actions/start", podHandler.StartPod) // start pod sandbox
	r.With(RequireCLIScope(ScopeResourceWrite)).Post("/v1/pods/{podId}/actions/stop", podHandler.StopPod)   // stop pod sandbox
	r.With(RequireCLIScope(ScopeResourceWrite)).Delete("/v1/pods/{podId}", podHandler.RemovePod)            // remove pod sandbox

	// == replicasets ==
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/replicasets", podHandler.GetReplicaSetList)                                      // list replicaset
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/replicasets/{replicaSetId}", podHandler.GetReplicaSetById)                       // get replicaset detail
	r.With(RequireCLIScope(ScopeResourceWrite)).Post("/v1/replicasets/{replicaSetId}/actions/scale", podHandler.ScaleReplicaSet) // scale replicaset
	r.With(RequireCLIScope(ScopeResourceWrite)).Delete("/v1/replicasets/{replicaSetId}", podHandler.RemoveReplicaSet)            // remove replicaset

	// == deployments ==
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/deployments", podHandler.GetDeploymentList)                                      // list deployment
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/deployments/{deploymentId}", podHandler.GetDeploymentById)                       // get deployment detail
	r.With(RequireCLIScope(ScopeResourceWrite)).Post("/v1/deployments/{deploymentId}/actions/scale", podHandler.ScaleDeployment) // scale deployment
	r.With(RequireCLIScope(ScopeResourceWrite)).Delete("/v1/deployments/{deploymentId}", podHandler.RemoveDeployment)            // remove deployment

	// == images ==
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/images", imageHandler.GetImageList)            // get image list
	r.With(RequireCLIScope(ScopeImageWrite)).Post("/v1/images", imageHandler.PullImage)        // pull image
	r.With(RequireCLIScope(ScopeImageWrite)).Post("/v1/images/build", imageHandler.BuildImage) // build image
	r.With(RequireCLIScope(ScopeImageWrite)).Delete("/v1/images", imageHandler.RemoveImage)    // remove image
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/images/status", imageHandler.GetImageStatus)
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/images/fs", imageHandler.GetImageFsInfo)

	// == services ==
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/services", serviceHandler.GetServiceList)             // list services
	r.With(RequireCLIScope(ScopeResourceWrite)).Post("/v1/services", serviceHandler.CreateService)    // create service
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/services/{serviceId}", serviceHandler.GetServiceById) // get service detail
	r.With(RequireCLIScope(ScopeResourceWrite)).Delete("/v1/services/{serviceId}", serviceHandler.RemoveService)

	// == ingresses ==
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/ingresses", ingressHandler.GetIngressList) // list ingress

	// == namespaces ==
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/namespaces", namespaceHandler.GetNamespaceList)
	r.With(RequireCLIScope(ScopeResourceWrite)).Post("/v1/namespaces", namespaceHandler.CreateNamespace)
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/namespaces/{name}", namespaceHandler.GetNamespace)
	r.With(RequireCLIScope(ScopeResourceWrite)).Delete("/v1/namespaces/{name}/actions/delete", namespaceHandler.DeleteNamespace)

	// == network ==
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/networks", networkHandler.GetNetworkList)        // list network
	r.With(RequireCLIScope(ScopeNetworkWrite)).Post("/v1/networks", networkHandler.CreateBridge) // create network
	r.With(RequireCLIScope(ScopeNetworkWrite)).Delete("/v1/networks/{bridge}/actions/delete", networkHandler.DeleteBridge)

	// == websocket ==
	r.With(RequireCLIScope(ScopeContainerAttach)).Get("/v1/containers/{containerId}/attach", socketHandler.ServeHTTP)
	r.With(RequireCLIScope(ScopeContainerAttach)).Get("/v1/containers/{containerId}/exec/attach", execSocketHandler.ServeHTTP)

	// == policy ==
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/policies/{chain}", policyHandler.GetPolicyList)        // get policy
	r.With(RequireCLIScope(ScopePolicyWrite)).Post("/v1/policies", policyHandler.AddPolicy)            // add policy
	r.With(RequireCLIScope(ScopePolicyWrite)).Post("/v1/policies/commit", policyHandler.CommitPolicy)  // commit policy
	r.With(RequireCLIScope(ScopePolicyWrite)).Post("/v1/policies/revert", policyHandler.RevertPolicy)  // revert policy
	r.With(RequireCLIScope(ScopePolicyWrite)).Post("/v1/policies/ns/mode", policyHandler.ChangeNSMode) // change NS mode
	r.With(RequireCLIScope(ScopePolicyWrite)).Delete("/v1/policies/{policyId}", policyHandler.RemovePolicy)

	// == logs ==
	r.With(RequireCLIScope(ScopeRead)).Get("/v1/logs/netflow", logHandler.GetNetflowLog) // get netflow log

	return r
}

func NewHookRouter() *chi.Mux {
	r := chi.NewRouter()
	hookHandler := hookHandler.NewRequestHandler()

	// middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// LOGGER
	node, _ := os.Hostname()
	r.Use(logger.LoggerMiddleware(
		logger.JsonLineLogger{Out: openAuditLog()},
		"condenser",
		node,
	))
	// SPIFFE
	r.Use(RequireSPIFFE("spiffe://raind/container"))

	// == hook ==
	r.Post("/v1/hooks/droplet", hookHandler.ApplyHook)

	return r
}

func NewCARouter() *chi.Mux {
	r := chi.NewRouter()
	certHandler := certHandler.NewRequestHandler()

	// middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// SPIFFE
	r.Use(RequireSPIFFE("spiffe://raind/droplet/"))
	// LOGGER
	node, _ := os.Hostname()
	r.Use(logger.LoggerMiddleware(
		logger.JsonLineLogger{Out: openAuditLog()},
		"condenser",
		node,
	))

	// == CA ==
	r.Post("/v1/pki/sign", certHandler.SignCSRHandler)

	return r
}

func RequireSPIFFE(prefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
				http.Error(w, "client certificate required", http.StatusUnauthorized)
				return
			}
			// validate
			cert := r.TLS.PeerCertificates[0]
			if len(cert.URIs) == 0 {
				http.Error(w, "client certificate URI SAN required", http.StatusForbidden)
				return
			}
			for _, spiffeId := range cert.URIs {
				if strings.HasPrefix(spiffeId.String(), prefix) {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}
}

type cliIdentityContextKey struct{}

const (
	ScopeRead            = "read"
	ScopeBottleWrite     = "bottle.write"
	ScopeContainerWrite  = "container.write"
	ScopeContainerExec   = "container.exec"
	ScopeContainerAttach = "container.attach"
	ScopeResourceWrite   = "resource.write"
	ScopeImageWrite      = "image.write"
	ScopeNetworkWrite    = "network.write"
	ScopePolicyWrite     = "policy.write"
	scopeAll             = "*"
)

type CLIIdentity struct {
	SPIFFE string
	Role   string
	Scopes map[string]struct{}
}

func (i CLIIdentity) HasScope(scope string) bool {
	if i.Role == "admin" {
		return true
	}
	if _, ok := i.Scopes[scopeAll]; ok {
		return true
	}
	if _, ok := i.Scopes[scope]; ok {
		return true
	}
	return false
}

func RequireCLIIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			http.Error(w, "client certificate required", http.StatusUnauthorized)
			return
		}

		cert := r.TLS.PeerCertificates[0]
		if len(cert.URIs) == 0 {
			http.Error(w, "client certificate URI SAN required", http.StatusForbidden)
			return
		}

		for _, uri := range cert.URIs {
			identity, err := parseCLIIdentity(uri.String())
			if err == nil {
				ctx := context.WithValue(r.Context(), cliIdentityContextKey{}, identity)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		http.Error(w, "forbidden", http.StatusForbidden)
	})
}

func RequireCLIScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := CLIIdentityFromContext(r.Context())
			if !ok {
				http.Error(w, "client identity required", http.StatusUnauthorized)
				return
			}
			if !identity.HasScope(scope) {
				http.Error(w, "insufficient scope", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func CLIIdentityFromContext(ctx context.Context) (CLIIdentity, bool) {
	identity, ok := ctx.Value(cliIdentityContextKey{}).(CLIIdentity)
	return identity, ok
}

func parseCLIIdentity(raw string) (CLIIdentity, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return CLIIdentity{}, err
	}
	if u.Scheme != "spiffe" || u.Host != "raind" {
		return CLIIdentity{}, errors.New("invalid cli spiffe trust domain")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "cli" || parts[1] == "" {
		return CLIIdentity{}, errors.New("invalid cli spiffe path")
	}

	role := parts[1]
	scopes := map[string]struct{}{}
	for _, scopePart := range parts[2:] {
		for _, scope := range strings.Split(scopePart, ",") {
			scope = strings.TrimSpace(scope)
			if scope != "" {
				scopes[scope] = struct{}{}
			}
		}
	}
	for _, scope := range defaultScopesForRole(role) {
		scopes[scope] = struct{}{}
	}

	return CLIIdentity{
		SPIFFE: raw,
		Role:   role,
		Scopes: scopes,
	}, nil
}

func defaultScopesForRole(role string) []string {
	switch role {
	case "admin":
		return []string{scopeAll}
	case "read":
		return []string{ScopeRead}
	default:
		return nil
	}
}

func openAuditLog() *os.File {
	fd, err := os.OpenFile(utils.AuditLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		log.Fatal("open audit log file failed")
	}
	return fd
}
