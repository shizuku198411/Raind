package pod

import (
	"condenser/internal/core/container"
	"condenser/internal/store/psm"
	"condenser/internal/utils"
	"log"
	"strings"
	"time"
)

func NewPodController() *PodController {
	return &PodController{
		psmHandler:       psm.NewPsmManager(psm.NewPsmStore(utils.PsmStorePath)),
		podHandler:       NewPodService(),
		containerHandler: container.NewContaierService(),
		interval:         5 * time.Second,
	}
}

type PodController struct {
	psmHandler       psm.PsmHandler
	podHandler       PodServiceHandler
	containerHandler container.ContainerServiceHandler
	interval         time.Duration
}

func (c *PodController) Start() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := c.reconcileOnce(); err != nil {
			log.Printf("pod controller reconcile failed: %v", err)
		}
	}
}

func (c *PodController) reconcileOnce() error {
	replicaSets, err := c.psmHandler.GetReplicaSetList()
	if err != nil {
		return err
	}

	templates, err := c.psmHandler.GetPodTemplateList()
	if err != nil {
		return err
	}
	if len(templates) == 0 && len(replicaSets) == 0 {
		return nil
	}

	pods, err := c.psmHandler.GetPodList()
	if err != nil {
		return err
	}

	// ReplicaSet reconcile
	if len(replicaSets) > 0 {
		podsByTemplate := make(map[string][]psm.PodInfo, len(templates))
		for _, p := range pods {
			if p.TemplateId == "" {
				continue
			}
			podsByTemplate[p.TemplateId] = append(podsByTemplate[p.TemplateId], p)
		}
		for _, rs := range replicaSets {
			podList := podsByTemplate[rs.Spec.TemplateId]
			current := len(podList)
			if current < rs.Spec.Replicas {
				for i := 0; i < rs.Spec.Replicas-current; i++ {
					name := rs.Spec.Name + "-" + utils.NewUlid()[:8]
					podId, err := c.podHandler.CreateFromTemplate(rs.Spec.TemplateId, name)
					if err != nil {
						log.Printf("pod controller recreate failed: templateId=%s err=%v", rs.Spec.TemplateId, err)
						break
					}
					if _, err := c.podHandler.Start(podId); err != nil {
						log.Printf("pod controller start failed: podId=%s err=%v", podId, err)
					}
				}
			} else if current > rs.Spec.Replicas {
				excess := current - rs.Spec.Replicas
				for i := 0; i < excess; i++ {
					if err := c.deletePod(podList[i]); err != nil {
						log.Printf("pod controller delete failed: podId=%s err=%v", podList[i].PodId, err)
					}
				}
			}
			for _, p := range podList {
				if p.StoppedByUser {
					continue
				}
				if p.State == "degraded" {
					infraState, err := c.getPodInfraState(p.PodId)
					if err != nil {
						log.Printf("pod controller infra check failed: podId=%s err=%v", p.PodId, err)
						continue
					}
					// If infra is not running, namespace continuity is broken.
					// Recreate pod from template (infra + members) to avoid stale ns path usage.
					if infraState != "running" {
						if err := c.recreatePod(p); err != nil {
							log.Printf("pod controller recreate failed: podId=%s err=%v", p.PodId, err)
						}
						continue
					}
					// Infra is running, so only recover member containers.
					if _, err := c.podHandler.Start(p.PodId); err != nil {
						log.Printf("pod controller start failed: podId=%s err=%v", p.PodId, err)
					}
					continue
				}
				if p.State == "stopped" {
					if _, err := c.podHandler.Start(p.PodId); err != nil {
						log.Printf("pod controller start failed: podId=%s err=%v", p.PodId, err)
					}
				}
			}
		}
	}
	podsByTemplate := make(map[string][]psm.PodInfo, len(templates))
	for _, p := range pods {
		if p.TemplateId == "" {
			continue
		}
		podsByTemplate[p.TemplateId] = append(podsByTemplate[p.TemplateId], p)
	}

	for _, tpl := range templates {
		inUse, err := c.psmHandler.IsTemplateReferenced(tpl.TemplateId)
		if err != nil {
			log.Printf("pod controller template check failed: templateId=%s err=%v", tpl.TemplateId, err)
			continue
		}
		if inUse {
			continue
		}
		podList := podsByTemplate[tpl.TemplateId]
		if len(podList) == 0 {
			if _, err := c.podHandler.RecreateFromTemplate(tpl.TemplateId); err != nil {
				log.Printf("pod controller recreate failed: templateId=%s err=%v", tpl.TemplateId, err)
			}
			continue
		}

		var hasActive bool
		var degradedPodId string
		var stoppedPodId string
		for _, p := range podList {
			if !p.StoppedByUser {
				infraDown, err := c.isPodInfraDown(p.PodId)
				if err != nil {
					log.Printf("pod controller infra check failed: podId=%s err=%v", p.PodId, err)
				} else if infraDown {
					if err := c.recreatePod(p); err != nil {
						log.Printf("pod controller recreate failed: podId=%s err=%v", p.PodId, err)
					}
					hasActive = true
					break
				}
			}
			if p.State == "degraded" && !p.StoppedByUser && degradedPodId == "" {
				degradedPodId = p.PodId
			}
			if p.State != "stopped" {
				hasActive = true
				break
			}
			if p.StoppedByUser {
				continue
			}
			if stoppedPodId == "" {
				stoppedPodId = p.PodId
			}
		}
		if degradedPodId != "" {
			if _, err := c.podHandler.Start(degradedPodId); err != nil {
				log.Printf("pod controller start failed: podId=%s err=%v", degradedPodId, err)
			}
			continue
		}
		if hasActive || stoppedPodId == "" {
			continue
		}
		if _, err := c.podHandler.Start(stoppedPodId); err != nil {
			log.Printf("pod controller start failed: podId=%s err=%v", stoppedPodId, err)
		}
	}

	return nil
}

func (c *PodController) isPodInfraDown(podId string) (bool, error) {
	state, err := c.getPodInfraState(podId)
	if err != nil {
		return false, err
	}
	return state != "running", nil
}

func (c *PodController) getPodInfraState(podId string) (string, error) {
	containers, err := c.containerHandler.GetContainersByPodId(podId)
	if err != nil {
		return "", err
	}
	if len(containers) == 0 {
		return "missing", nil
	}
	for _, cinfo := range containers {
		if strings.HasPrefix(cinfo.Name, utils.PodInfraContainerNamePrefix) {
			if cinfo.State == "running" {
				return "running", nil
			}
			return "stopped", nil
		}
	}
	// infra missing
	return "missing", nil
}

func (c *PodController) recreatePod(podInfo psm.PodInfo) error {
	containers, err := c.containerHandler.GetContainersByPodId(podInfo.PodId)
	if err != nil {
		return err
	}
	for _, cinfo := range containers {
		if cinfo.State == "running" {
			_, _ = c.containerHandler.Stop(container.ServiceStopModel{ContainerId: cinfo.ContainerId})
		}
		_, _ = c.containerHandler.Delete(container.ServiceDeleteModel{ContainerId: cinfo.ContainerId})
	}
	if err := c.psmHandler.RemovePod(podInfo.PodId); err != nil {
		// ignore already removed
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
	}
	if podInfo.TemplateId == "" {
		return nil
	}
	newPodId, err := c.podHandler.RecreateFromTemplate(podInfo.TemplateId)
	if err != nil {
		return err
	}
	_, err = c.podHandler.Start(newPodId)
	return err
}

func (c *PodController) deletePod(podInfo psm.PodInfo) error {
	containers, err := c.containerHandler.GetContainersByPodId(podInfo.PodId)
	if err != nil {
		return err
	}
	for _, cinfo := range containers {
		if cinfo.State == "running" {
			_, _ = c.containerHandler.Stop(container.ServiceStopModel{ContainerId: cinfo.ContainerId})
		}
		_, _ = c.containerHandler.Delete(container.ServiceDeleteModel{ContainerId: cinfo.ContainerId})
	}
	return c.psmHandler.RemovePod(podInfo.PodId)
}
