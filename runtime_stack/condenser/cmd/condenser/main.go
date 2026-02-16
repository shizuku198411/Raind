package main

import (
	httpapi "condenser/internal/api/http"
	"condenser/internal/core/cert"
	"condenser/internal/core/pod"
	"condenser/internal/core/service"
	"condenser/internal/dns"
	enrichedlog "condenser/internal/enriched_log"
	"condenser/internal/env"
	"condenser/internal/monitor"
	"condenser/internal/utils"
	"crypto/tls"
	"log"
	"net/http"
)

func main() {
	// == bootstrap ==
	bootstrap := env.NewBootstrapManager()
	if err := bootstrap.SetupRuntime(); err != nil {
		log.Fatal(err)
	}

	// == rest api ==
	clientCA, err := cert.LoadCertPoolFromFile(utils.ClientIssuerCACertPath)
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS13,
		ClientCAs:  clientCA,
		ClientAuth: tls.RequireAndVerifyClientCert,
	}

	// Management Server
	managementAddr := "127.0.0.1:7755"
	managementRouter := httpapi.NewApiRouter()
	if err != nil {
		log.Fatal(err)
	}
	managementSrv := &http.Server{
		Addr:      managementAddr,
		Handler:   managementRouter,
		TLSConfig: tlsCfg,
	}
	go func() {
		log.Printf("[*] management server listening on %s", managementAddr)
		if err := managementSrv.ListenAndServeTLS(utils.PublicCertPath, utils.PrivateKeyPath); err != nil {
			log.Fatal(err)
		}
	}()

	// Hook Server
	hookAddr := ":7756"
	hookRouter := httpapi.NewHookRouter()
	hookSrv := &http.Server{
		Addr:      hookAddr,
		Handler:   hookRouter,
		TLSConfig: tlsCfg,
	}
	go func() {
		log.Printf("[*] hook server listening on %s", hookAddr)
		if err := hookSrv.ListenAndServeTLS(utils.PublicCertPath, utils.PrivateKeyPath); err != nil {
			log.Fatal(err)
		}
	}()

	// CA Server
	caAddr := "127.0.0.1:7757"
	caRouter := httpapi.NewCARouter()
	caSrv := &http.Server{
		Addr:      caAddr,
		Handler:   caRouter,
		TLSConfig: tlsCfg,
	}
	go func() {
		log.Printf("[*] ca server listening on %s", caAddr)
		if err := caSrv.ListenAndServeTLS(utils.PublicCertPath, utils.PrivateKeyPath); err != nil {
			log.Fatal(err)
		}
	}()

	// Swagger
	swaggerAddr := ":7758"
	swaggerRouter := httpapi.NewSwaggerRouter()
	swaggerSrv := &http.Server{
		Addr:    swaggerAddr,
		Handler: swaggerRouter,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
		},
	}
	go func() {
		log.Printf("[*] swagger listening on %s", swaggerAddr)
		if err := swaggerSrv.ListenAndServeTLS(utils.PublicCertPath, utils.PrivateKeyPath); err != nil {
			log.Fatal(err)
		}
	}()

	// enriched logger
	go func() {
		log.Printf("[*] enrichement logger start")
		enLogger := enrichedlog.NewEnrichedLogHandler()
		enLogger.EnrichedLogger()
	}()

	// pod controller
	go func() {
		log.Printf("[*] pod controller start")
		pod.NewPodController().Start()
	}()

	// service controller
	go func() {
		log.Printf("[*] service controller start")
		service.NewServiceController().Start()
	}()

	// forwarder
	go func() {
		log.Printf("[*] dns proxy listening")
		dns.StartDnsProxy()
	}()

	// start monitoring
	log.Println("[*] Container Monitoring Start")
	containerMonitoring := monitor.NewContainerMonitor()
	containerMonitoring.Start()
}
