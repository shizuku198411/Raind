package netpol

import (
	"fmt"
	"time"
)

func NewManager(store *Store) *Manager {
	return &Manager{store: store}
}

type Manager struct {
	store *Store
}

func (m *Manager) StoreNetworkPolicy(networkPolicyId string, info NetworkPolicyInfo) error {
	return m.store.withLock(func(st *NetworkPolicyState) error {
		info.NetworkPolicyId = networkPolicyId
		if info.Namespace == "" {
			info.Namespace = "default"
		}
		if info.CreatedAt.IsZero() {
			info.CreatedAt = time.Now()
		}
		st.NetworkPolicies[networkPolicyId] = info
		return nil
	})
}

func (m *Manager) GetNetworkPolicyList() ([]NetworkPolicyInfo, error) {
	var list []NetworkPolicyInfo
	err := m.store.withRLock(func(st *NetworkPolicyState) error {
		for _, policy := range st.NetworkPolicies {
			list = append(list, policy)
		}
		return nil
	})
	return list, err
}

func (m *Manager) GetNetworkPolicyById(networkPolicyId string) (NetworkPolicyInfo, error) {
	var info NetworkPolicyInfo
	err := m.store.withRLock(func(st *NetworkPolicyState) error {
		policy, ok := st.NetworkPolicies[networkPolicyId]
		if !ok {
			return fmt.Errorf("networkPolicyId=%s not found", networkPolicyId)
		}
		info = policy
		return nil
	})
	return info, err
}

func (m *Manager) GetNetworkPolicyByName(name, namespace string) (NetworkPolicyInfo, error) {
	if namespace == "" {
		namespace = "default"
	}
	var info NetworkPolicyInfo
	err := m.store.withRLock(func(st *NetworkPolicyState) error {
		for _, policy := range st.NetworkPolicies {
			if policy.Name == name && policy.Namespace == namespace {
				info = policy
				return nil
			}
		}
		return fmt.Errorf("networkpolicy %s/%s not found", namespace, name)
	})
	return info, err
}

func (m *Manager) RemoveNetworkPolicy(networkPolicyId string) error {
	return m.store.withLock(func(st *NetworkPolicyState) error {
		if _, ok := st.NetworkPolicies[networkPolicyId]; !ok {
			return fmt.Errorf("networkPolicyId=%s not found", networkPolicyId)
		}
		delete(st.NetworkPolicies, networkPolicyId)
		return nil
	})
}

func (m *Manager) IsNameAlreadyUsed(name, namespace string) bool {
	if namespace == "" {
		namespace = "default"
	}
	var used bool
	_ = m.store.withRLock(func(st *NetworkPolicyState) error {
		for _, policy := range st.NetworkPolicies {
			if policy.Name == name && policy.Namespace == namespace {
				used = true
				return nil
			}
		}
		return nil
	})
	return used
}
