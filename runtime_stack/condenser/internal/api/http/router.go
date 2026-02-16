package http

import (
	_ "condenser/docs"
	"log"
	"net/http"
	"os"
	"strings"

	bottleHandler "condenser/internal/api/http/bottle"
	certHandler "condenser/internal/api/http/cert"
	containerHandler "condenser/internal/api/http/container"
	hookHandler "condenser/internal/api/http/hook"
	imageHandler "condenser/internal/api/http/image"
	"condenser/internal/api/http/logger"
	logHandler "condenser/internal/api/http/logs"
	networkHandler "condenser/internal/api/http/network"
	podHandler "condenser/internal/api/http/pod"
	policyHandler "condenser/internal/api/http/policy"
	serviceHandler "condenser/internal/api/http/service"
	websocketHandler "condenser/internal/api/http/websocket"
	"condenser/internal/utils"

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

	// middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// SPIFFE
	r.Use(RequireSPIFFE("spiffe://raind/cli/"))
	// LOGGER
	node, _ := os.Hostname()
	r.Use(logger.LoggerMiddleware(
		logger.JsonLineLogger{Out: openAuditLog()},
		"condenser",
		node,
	))

	// == v1 ==
	// == bottles ==
	r.Post("/v1/bottle", bottleHandler.RegisterBottle)            // register bottle
	r.Get("/v1/bottle", bottleHandler.GetBottleList)              // get bottle list
	r.Get("/v1/bottle/{bottleId}", bottleHandler.GetBottleDetail) // get bottle detail
	r.Post("/v1/bottle/{bottleId}/actions/{action}", bottleHandler.ActionBottle)

	// == containers ==
	r.Get("/v1/containers", containerHandler.GetContainerList)                                // get container list
	r.Get("/v1/containers/{containerId}", containerHandler.GetContainerById)                  // get container status by id
	r.Get("/v1/containers/{containerId}/log", containerHandler.GetContainerLog)               // get container log
	r.Get("/v1/containers/{containerId}/logpath", containerHandler.GetContainerLogPath)       // get container log path
	r.Get("/v1/containers/{containerId}/stats", containerHandler.GetContainerStats)           // get container stats
	r.Get("/v1/containers/stats", containerHandler.ListContainerStats)                        // get container stats list
	r.Post("/v1/containers", containerHandler.CreateContainer)                                // create container
	r.Post("/v1/containers/{containerId}/actions/start", containerHandler.StartContainer)     // start container
	r.Post("/v1/containers/{containerId}/actions/stop", containerHandler.StopContainer)       // stop container
	r.Post("/v1/containers/{containerId}/actions/exec", containerHandler.ExecContainer)       // exec container
	r.Delete("/v1/containers/{containerId}/actions/delete", containerHandler.DeleteContainer) // delete container

	// == resource ==
	r.Post("/v1/resource/apply", podHandler.ApplyPodYaml) // apply yaml
	r.Post("/v1/resource/delete", podHandler.DeleteResourceYaml)

	// == pods ==
	r.Get("/v1/pods", podHandler.GetPodList)                      // list pods
	r.Post("/v1/pods", podHandler.CreatePod)                      // create pod sandbox
	r.Get("/v1/pods/{podId}", podHandler.GetPodById)              // get pod sandbox detail
	r.Post("/v1/pods/{podId}/actions/start", podHandler.StartPod) // start pod sandbox
	r.Post("/v1/pods/{podId}/actions/stop", podHandler.StopPod)   // stop pod sandbox
	r.Delete("/v1/pods/{podId}", podHandler.RemovePod)            // remove pod sandbox

	// == replicasets ==
	r.Get("/v1/replicasets", podHandler.GetReplicaSetList)                             // list replicaset
	r.Get("/v1/replicasets/{replicaSetId}", podHandler.GetReplicaSetById)              // get replicaset detail
	r.Post("/v1/replicasets/{replicaSetId}/actions/scale", podHandler.ScaleReplicaSet) // scale replicaset
	r.Delete("/v1/replicasets/{replicaSetId}", podHandler.RemoveReplicaSet)            // remove replicaset

	// == images ==
	r.Get("/v1/images", imageHandler.GetImageList)      // get image list
	r.Post("/v1/images", imageHandler.PullImage)        // pull image
	r.Post("/v1/images/build", imageHandler.BuildImage) // build image
	r.Delete("/v1/images", imageHandler.RemoveImage)    // remove image
	r.Get("/v1/images/status", imageHandler.GetImageStatus)
	r.Get("/v1/images/fs", imageHandler.GetImageFsInfo)

	// == services ==
	r.Get("/v1/services", serviceHandler.GetServiceList)               // list services
	r.Post("/v1/services", serviceHandler.CreateService)               // create service
	r.Get("/v1/services/{serviceId}", serviceHandler.GetServiceById)   // get service detail
	r.Delete("/v1/services/{serviceId}", serviceHandler.RemoveService) // remove service

	// == network ==
	r.Get("/v1/networks", networkHandler.GetNetworkList)                          // list network
	r.Post("/v1/networks", networkHandler.CreateBridge)                           // create network
	r.Delete("/v1/networks/{bridge}/actions/delete", networkHandler.DeleteBridge) // delete network

	// == websocket ==
	r.Get("/v1/containers/{containerId}/attach", socketHandler.ServeHTTP)
	r.Get("/v1/containers/{containerId}/exec/attach", execSocketHandler.ServeHTTP)

	// == policy ==
	r.Get("/v1/policies/{chain}", policyHandler.GetPolicyList)      // get policy
	r.Post("/v1/policies", policyHandler.AddPolicy)                 // add policy
	r.Post("/v1/policies/commit", policyHandler.CommitPolicy)       // commit policy
	r.Post("/v1/policies/revert", policyHandler.RevertPolicy)       // revert policy
	r.Post("/v1/policies/ns/mode", policyHandler.ChangeNSMode)      // change NS mode
	r.Delete("/v1/policies/{policyId}", policyHandler.RemovePolicy) // remove policy

	// == logs ==
	r.Get("/v1/logs/netflow", logHandler.GetNetflowLog) // get netflow log

	return r
}

func NewHookRouter() *chi.Mux {
	r := chi.NewRouter()
	hookHandler := hookHandler.NewRequestHandler()

	// middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// SPIFFE
	r.Use(RequireSPIFFE("spiffe://raind/container"))
	// LOGGER
	node, _ := os.Hostname()
	r.Use(logger.LoggerMiddleware(
		logger.JsonLineLogger{Out: openAuditLog()},
		"condenser",
		node,
	))

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
			spiffeId := cert.URIs[0]
			if strings.HasPrefix(spiffeId.String(), prefix) {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}
}

func openAuditLog() *os.File {
	fd, err := os.OpenFile(utils.AuditLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		log.Fatal("open audit log file failed")
	}
	return fd
}
