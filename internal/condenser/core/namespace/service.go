package namespace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"raind/internal/condenser/core/network"
	"raind/internal/condenser/store/ipam"
	"raind/internal/condenser/store/nsm"
	"raind/internal/condenser/store/psm"
	"raind/internal/condenser/store/ssm"
	"raind/internal/condenser/utils"
)

func NewNamespaceService() *NamespaceService {
	return &NamespaceService{
		nsmHandler:     nsm.NewNsmManager(nsm.NewNsmStore(utils.NsmStorePath)),
		psmHandler:     psm.NewPsmManager(psm.NewPsmStore(utils.PsmStorePath)),
		ssmHandler:     ssm.NewSsmManager(ssm.NewSsmStore(utils.SsmStorePath)),
		ipamHandler:    ipam.NewIpamManager(ipam.NewIpamStore(utils.IpamStorePath)),
		networkHandler: network.NewNetworkService(),
	}
}

type NamespaceService struct {
	nsmHandler     nsm.NsmHandler
	psmHandler     psm.PsmHandler
	ssmHandler     ssm.SsmHandler
	ipamHandler    ipam.IpamHandler
	networkHandler network.NetworkServiceHandler
}

var namespaceNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

func (s *NamespaceService) Create(param ServiceCreateModel) (NamespaceInfo, error) {
	name := strings.TrimSpace(param.Name)
	if err := validateNamespaceName(name); err != nil {
		return NamespaceInfo{}, err
	}
	if name == nsm.DefaultNamespace {
		return NamespaceInfo{}, fmt.Errorf("namespace already exists: %s", name)
	}

	networkName := strings.TrimSpace(param.Network)
	autoNetwork := networkName == ""
	if autoNetwork {
		networkName = bridgeNameForNamespace(name)
		if err := s.networkHandler.CreateNewNetwork(network.ServiceNewNetworkModel{Bridge: networkName}); err != nil {
			return NamespaceInfo{}, err
		}
	} else if _, err := s.ipamHandler.GetBridgeAddr(networkName); err != nil {
		return NamespaceInfo{}, fmt.Errorf("network not found: %s", networkName)
	}

	info := nsm.NamespaceInfo{
		Name:        name,
		Network:     networkName,
		NetworkAuto: autoNetwork,
		Labels:      param.Labels,
		Annotations: param.Annotations,
		CreatedAt:   time.Now(),
	}
	if err := s.nsmHandler.StoreNamespace(info); err != nil {
		if autoNetwork {
			_ = s.networkHandler.RemoveNetwork(network.ServiceRemoveNetworkModel{Bridge: networkName})
		}
		return NamespaceInfo{}, err
	}
	return s.enrich(info)
}

func (s *NamespaceService) Remove(param ServiceRemoveModel) (string, error) {
	name := strings.TrimSpace(param.Name)
	if name == "" {
		return "", fmt.Errorf("namespace name is required")
	}
	if name == nsm.DefaultNamespace {
		return "", fmt.Errorf("default namespace cannot be removed")
	}
	info, err := s.nsmHandler.GetNamespace(name)
	if err != nil {
		return "", err
	}
	enriched, err := s.enrich(info)
	if err != nil {
		return "", err
	}
	if !enriched.Resources.isEmpty() {
		return "", fmt.Errorf("namespace %s is not empty", name)
	}
	if info.NetworkAuto && info.Network != "" {
		if err := s.networkHandler.RemoveNetwork(network.ServiceRemoveNetworkModel{Bridge: info.Network}); err != nil {
			return "", err
		}
	}
	if err := s.nsmHandler.RemoveNamespace(name); err != nil {
		return "", err
	}
	return name, nil
}

func (s *NamespaceService) Get(name string) (NamespaceInfo, error) {
	info, err := s.nsmHandler.GetNamespace(strings.TrimSpace(name))
	if err != nil {
		return NamespaceInfo{}, err
	}
	return s.enrich(info)
}

func (s *NamespaceService) List() ([]NamespaceInfo, error) {
	namespaces, err := s.nsmHandler.GetNamespaceList()
	if err != nil {
		return nil, err
	}
	out := make([]NamespaceInfo, 0, len(namespaces))
	for _, ns := range namespaces {
		info, err := s.enrich(ns)
		if err != nil {
			return nil, err
		}
		out = append(out, info)
	}
	return out, nil
}

func (s *NamespaceService) ResolveNetwork(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		name = nsm.DefaultNamespace
	}
	info, err := s.nsmHandler.GetNamespace(name)
	if err != nil {
		return "", err
	}
	if info.Network == "" {
		return nsm.DefaultNamespaceNetwork, nil
	}
	return info.Network, nil
}

func (s *NamespaceService) enrich(info nsm.NamespaceInfo) (NamespaceInfo, error) {
	counts, err := s.resourceCounts(info.Name, info.Network)
	if err != nil {
		return NamespaceInfo{}, err
	}
	return NamespaceInfo{
		Name:        info.Name,
		Network:     info.Network,
		NetworkAuto: info.NetworkAuto,
		Labels:      info.Labels,
		Annotations: info.Annotations,
		CreatedAt:   info.CreatedAt,
		Resources:   counts,
	}, nil
}

func (s *NamespaceService) resourceCounts(namespaceName, networkName string) (ResourceCounts, error) {
	var counts ResourceCounts
	pods, err := s.psmHandler.GetPodList()
	if err != nil {
		return counts, err
	}
	for _, p := range pods {
		if p.Namespace == namespaceName {
			counts.Pods++
		}
	}
	services, err := s.ssmHandler.GetServiceList()
	if err != nil {
		return counts, err
	}
	for _, svc := range services {
		if svc.Namespace == namespaceName {
			counts.Services++
		}
	}
	replicaSets, err := s.psmHandler.GetReplicaSetList()
	if err != nil {
		return counts, err
	}
	for _, rs := range replicaSets {
		if rs.Spec.Namespace == namespaceName {
			counts.ReplicaSets++
		}
	}
	deployments, err := s.psmHandler.GetDeploymentList()
	if err != nil {
		return counts, err
	}
	for _, deploy := range deployments {
		if deploy.Spec.Namespace == namespaceName {
			counts.Deployments++
		}
	}
	networks, err := s.ipamHandler.GetNetworkList()
	if err != nil {
		return counts, nil
	}
	for _, n := range networks {
		if n.Interface == networkName {
			counts.Allocations = n.NumContainers
			break
		}
	}
	return counts, nil
}

func validateNamespaceName(name string) error {
	if name == "" {
		return fmt.Errorf("namespace name is required")
	}
	if len(name) > 63 {
		return fmt.Errorf("namespace name must be 63 characters or less")
	}
	if !namespaceNamePattern.MatchString(name) {
		return fmt.Errorf("invalid namespace name: %s", name)
	}
	return nil
}

func bridgeNameForNamespace(name string) string {
	sum := sha256.Sum256([]byte(name))
	return "rns" + hex.EncodeToString(sum[:])[:12]
}

func (r ResourceCounts) isEmpty() bool {
	return r.Pods == 0 &&
		r.Services == 0 &&
		r.ReplicaSets == 0 &&
		r.Deployments == 0 &&
		r.Allocations == 0
}
