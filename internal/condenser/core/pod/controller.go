package pod

import (
	"log"
	"raind/internal/condenser/core/container"
	"raind/internal/condenser/store/psm"
	"raind/internal/condenser/utils"
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
	if err := c.reconcileDeployments(); err != nil {
		return err
	}

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
		templateRefs := replicaSetTemplateRefs(replicaSets)
		for _, rs := range replicaSets {
			if shouldSkipReplicaSet(rs) {
				continue
			}
			var reconcileErr error
			podList := filterReplicaSetPods(rs, pods, templateRefs)
			podList, reconcileErr = c.cleanupStoppedManagedPods(podList)
			if reconcileErr != nil {
				c.recordReplicaSetReconcileError(rs, reconcileErr)
				continue
			}
			current := len(podList)
			if current < rs.Spec.Replicas {
				for i := 0; i < rs.Spec.Replicas-current; i++ {
					name := rs.Spec.Name + "-" + utils.NewUlid()[:8]
					podId, err := c.podHandler.CreateFromTemplate(rs.Spec.TemplateId, name)
					if err != nil {
						log.Printf("pod controller recreate failed: templateId=%s err=%v", rs.Spec.TemplateId, err)
						reconcileErr = err
						break
					}
					if err := c.psmHandler.UpdatePodOwner(podId, psm.OwnerKindReplicaSet, rs.ReplicaSetId); err != nil {
						log.Printf("pod controller owner update failed: podId=%s err=%v", podId, err)
						reconcileErr = err
						break
					}
					if _, err := c.podHandler.Start(podId); err != nil {
						log.Printf("pod controller start failed: podId=%s err=%v", podId, err)
						reconcileErr = err
						if podInfo, getErr := c.psmHandler.GetPodById(podId); getErr == nil {
							if delErr := c.deletePod(podInfo); delErr != nil {
								log.Printf("pod controller cleanup failed: podId=%s err=%v", podId, delErr)
							}
						}
					}
				}
			} else if current > rs.Spec.Replicas {
				excess := current - rs.Spec.Replicas
				for i := 0; i < excess; i++ {
					if err := c.deletePod(podList[i]); err != nil {
						log.Printf("pod controller delete failed: podId=%s err=%v", podList[i].PodId, err)
						reconcileErr = err
						break
					}
				}
			}
			if reconcileErr != nil {
				c.recordReplicaSetReconcileError(rs, reconcileErr)
				continue
			}
			for _, p := range podList {
				if p.StoppedByUser {
					continue
				}

				// The infra container owns the pod namespaces. If it is stopped or
				// removed while the pod state is still running, member containers may
				// keep running but the pod cannot safely reuse the old namespace paths.
				// Recreate the whole pod from its template instead of only restarting
				// member containers.
				infraState, err := c.getPodInfraState(p.PodId)
				if err != nil {
					log.Printf("pod controller infra check failed: podId=%s err=%v", p.PodId, err)
					reconcileErr = err
					continue
				}
				if infraState != psm.ContainerStateRunning {
					if err := c.recreatePod(p); err != nil {
						log.Printf("pod controller recreate failed: podId=%s err=%v", p.PodId, err)
						reconcileErr = err
					}
					continue
				}

				if p.State == psm.PodStateDegraded {
					// Infra is running, so only recover member containers.
					if _, err := c.podHandler.Start(p.PodId); err != nil {
						log.Printf("pod controller start failed: podId=%s err=%v", p.PodId, err)
						reconcileErr = err
					}
					continue
				}
				if p.State == psm.PodStateStopped {
					if _, err := c.podHandler.Start(p.PodId); err != nil {
						log.Printf("pod controller start failed: podId=%s err=%v", p.PodId, err)
						reconcileErr = err
					}
					continue
				}
				if p.State == psm.PodStateCreated {
					if _, err := c.podHandler.Start(p.PodId); err != nil {
						log.Printf("pod controller start failed: podId=%s err=%v", p.PodId, err)
						reconcileErr = err
						if err := c.recreatePod(p); err != nil {
							log.Printf("pod controller recreate failed: podId=%s err=%v", p.PodId, err)
						}
					}
				}
			}
			if reconcileErr != nil {
				c.recordReplicaSetReconcileError(rs, reconcileErr)
				continue
			}
			c.clearReplicaSetReconcileStatus(rs)
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
			if p.State == psm.PodStateCreated {
				hasActive = true
				break
			}
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
			if p.State == psm.PodStateDegraded && !p.StoppedByUser && degradedPodId == "" {
				degradedPodId = p.PodId
			}
			if p.State != psm.PodStateStopped {
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

func replicaSetTemplateRefs(replicaSets []psm.ReplicaSetInfo) map[string]int {
	refs := make(map[string]int, len(replicaSets))
	for _, rs := range replicaSets {
		refs[rs.Spec.TemplateId]++
	}
	return refs
}

func filterReplicaSetPods(rs psm.ReplicaSetInfo, pods []psm.PodInfo, templateRefs map[string]int) []psm.PodInfo {
	out := make([]psm.PodInfo, 0, len(pods))
	for _, p := range pods {
		if p.OwnerKind == psm.OwnerKindReplicaSet || p.OwnerId != "" {
			if p.OwnerKind == psm.OwnerKindReplicaSet && p.OwnerId == rs.ReplicaSetId {
				out = append(out, p)
			}
			continue
		}
		if p.TemplateId == rs.Spec.TemplateId && templateRefs[rs.Spec.TemplateId] == 1 && strings.HasPrefix(p.Name, rs.Spec.Name+"-") {
			out = append(out, p)
		}
	}
	return out
}

func shouldSkipReplicaSet(rs psm.ReplicaSetInfo) bool {
	return !rs.NextReconcileAt.IsZero() && time.Now().Before(rs.NextReconcileAt)
}

func (c *PodController) recordReplicaSetReconcileError(rs psm.ReplicaSetInfo, err error) {
	if err == nil {
		return
	}
	attempt := rs.ReconcileAttempt + 1
	delay := time.Duration(1<<min(attempt-1, 6)) * c.interval
	if delay <= 0 {
		delay = 5 * time.Second
	}
	if delay > time.Minute {
		delay = time.Minute
	}
	if statusErr := c.psmHandler.UpdateReplicaSetReconcileStatus(rs.ReplicaSetId, attempt, err.Error(), time.Now().Add(delay)); statusErr != nil {
		log.Printf("pod controller reconcile status update failed: replicaSetId=%s err=%v", rs.ReplicaSetId, statusErr)
	}
}

func (c *PodController) clearReplicaSetReconcileStatus(rs psm.ReplicaSetInfo) {
	if rs.ReconcileAttempt == 0 && rs.LastReconcileError == "" && rs.NextReconcileAt.IsZero() {
		return
	}
	if err := c.psmHandler.ClearReplicaSetReconcileStatus(rs.ReplicaSetId); err != nil {
		log.Printf("pod controller reconcile status clear failed: replicaSetId=%s err=%v", rs.ReplicaSetId, err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *PodController) cleanupStoppedManagedPods(pods []psm.PodInfo) ([]psm.PodInfo, error) {
	active := make([]psm.PodInfo, 0, len(pods))
	for _, p := range pods {
		if !p.StoppedByUser {
			active = append(active, p)
			continue
		}
		if err := c.deletePod(p); err != nil {
			log.Printf("pod controller delete stopped managed pod failed: podId=%s err=%v", p.PodId, err)
			return active, err
		}
	}
	return active, nil
}

func (c *PodController) reconcileDeployments() error {
	deployments, err := c.psmHandler.GetDeploymentList()
	if err != nil {
		return err
	}
	if len(deployments) == 0 {
		return nil
	}

	replicaSets, err := c.psmHandler.GetReplicaSetList()
	if err != nil {
		return err
	}
	replicaSetsById := make(map[string]psm.ReplicaSetInfo, len(replicaSets))
	for _, rs := range replicaSets {
		replicaSetsById[rs.ReplicaSetId] = rs
	}

	for _, deploy := range deployments {
		if deploy.Spec.ReplicaSetId == "" {
			replicaSetId := utils.NewUlid()
			if err := c.psmHandler.StoreReplicaSet(replicaSetId, psm.ReplicaSetSpec{
				Name:       deploy.Spec.Name,
				Namespace:  deploy.Spec.Namespace,
				Replicas:   deploy.Spec.Replicas,
				TemplateId: deploy.Spec.TemplateId,
				Selector:   deploy.Spec.Selector,
			}); err != nil {
				return err
			}
			if err := c.psmHandler.UpdateDeploymentReplicaSet(deploy.DeploymentId, replicaSetId); err != nil {
				if rbErr := c.psmHandler.RemoveReplicaSet(replicaSetId); rbErr != nil {
					log.Printf("deployment replicasets rollback failed: replicaSetId=%s err=%v", replicaSetId, rbErr)
				}
				return err
			}
			continue
		}

		rs, ok := replicaSetsById[deploy.Spec.ReplicaSetId]
		if !ok {
			if err := c.psmHandler.UpdateDeploymentReplicaSet(deploy.DeploymentId, ""); err != nil {
				return err
			}
			continue
		}
		if rs.Spec.Replicas != deploy.Spec.Replicas {
			if err := c.psmHandler.UpdateReplicaSetReplicas(rs.ReplicaSetId, deploy.Spec.Replicas); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *PodController) isPodInfraDown(podId string) (bool, error) {
	state, err := c.getPodInfraState(podId)
	if err != nil {
		return false, err
	}
	return state != psm.ContainerStateRunning, nil
}

func (c *PodController) getPodInfraState(podId string) (string, error) {
	containers, err := c.containerHandler.GetContainersByPodId(podId)
	if err != nil {
		return "", err
	}
	if len(containers) == 0 {
		return "missing", nil
	}
	runningInfra := 0
	stoppedInfra := 0
	for _, cinfo := range containers {
		if strings.HasPrefix(cinfo.Name, utils.PodInfraContainerNamePrefix) {
			if cinfo.State == psm.ContainerStateRunning {
				runningInfra++
				continue
			}
			stoppedInfra++
		}
	}
	if runningInfra == 1 && stoppedInfra == 0 {
		return psm.ContainerStateRunning, nil
	}
	if runningInfra > 1 {
		return "duplicate", nil
	}
	if stoppedInfra > 0 {
		return psm.ContainerStateStopped, nil
	}
	return "missing", nil
}

func (c *PodController) recreatePod(podInfo psm.PodInfo) error {
	containers, err := c.containerHandler.GetContainersByPodId(podInfo.PodId)
	if err != nil {
		return err
	}
	for _, cinfo := range containers {
		if cinfo.State == psm.ContainerStateRunning {
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
	if podInfo.OwnerKind != "" || podInfo.OwnerId != "" {
		if err := c.psmHandler.UpdatePodOwner(newPodId, podInfo.OwnerKind, podInfo.OwnerId); err != nil {
			return err
		}
	}
	_, err = c.podHandler.Start(newPodId)
	return err
}

func (c *PodController) deletePod(podInfo psm.PodInfo) error {
	if err := c.psmHandler.UpdatePodStoppedByUser(podInfo.PodId, true); err != nil {
		return err
	}
	containers, err := c.containerHandler.GetContainersByPodId(podInfo.PodId)
	if err != nil {
		return err
	}
	for _, cinfo := range containers {
		if cinfo.State == psm.ContainerStateRunning {
			_, _ = c.containerHandler.Stop(container.ServiceStopModel{ContainerId: cinfo.ContainerId})
		}
		_, _ = c.containerHandler.Delete(container.ServiceDeleteModel{ContainerId: cinfo.ContainerId})
	}
	return c.psmHandler.RemovePod(podInfo.PodId)
}
