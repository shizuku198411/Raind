package networkpolicy

import (
	"log"
	"raind/internal/condenser/core/policy"
	"sync"
	"time"
)

func NewNetworkPolicyController() *NetworkPolicyController {
	return &NetworkPolicyController{
		service:       NewService(),
		policyHandler: policy.NewwServicePolicy(),
		interval:      2 * time.Second,
	}
}

type NetworkPolicyController struct {
	service       *Service
	policyHandler policy.PolicyServiceHandler
	interval      time.Duration
	mu            sync.Mutex
}

func (c *NetworkPolicyController) Start() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := c.reconcileOnce(); err != nil {
			log.Printf("networkpolicy controller reconcile failed: %v", err)
		}
	}
}

func (c *NetworkPolicyController) reconcileOnce() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	changed, err := c.service.ReconcileAll()
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if err := c.policyHandler.CommitPolicy(); err != nil {
		return err
	}
	log.Printf("networkpolicy controller committed policy updates")
	return nil
}
