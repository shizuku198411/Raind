package npm

import (
	"fmt"
)

func NewNpmManager(npmStrore *NpmStore) *NpmManager {
	return &NpmManager{
		npmStore: npmStrore,
	}
}

type NpmManager struct {
	npmStore *NpmStore
}

func (m *NpmManager) GetEWMode() string {
	var mode string
	_ = m.npmStore.withRLock(func(np *NetworkPolicy) error {
		mode = np.DefaultRule.EastWest.Mode
		return nil
	})
	return mode
}

func (m *NpmManager) GetNSMode() string {
	var mode string
	_ = m.npmStore.withRLock(func(np *NetworkPolicy) error {
		mode = np.DefaultRule.NorthSouth.Mode
		return nil
	})
	return mode
}

func (m *NpmManager) GetEWLogging() bool {
	var result bool
	_ = m.npmStore.withRLock(func(np *NetworkPolicy) error {
		result = np.DefaultRule.EastWest.Logging
		return nil
	})
	return result
}

func (m *NpmManager) GetNSLogging() bool {
	var result bool
	_ = m.npmStore.withRLock(func(np *NetworkPolicy) error {
		result = np.DefaultRule.NorthSouth.Logging
		return nil
	})
	return result
}

func (m *NpmManager) IsNsEnforce() bool {
	var result bool
	_ = m.npmStore.withRLock(func(np *NetworkPolicy) error {
		nsMode := np.DefaultRule.NorthSouth.Mode
		if nsMode == "enforce" {
			result = true
			return nil
		}
		result = false
		return nil
	})
	return result
}

func (m *NpmManager) GetEWPolicyList() []Policy {
	var policyList []Policy
	_ = m.npmStore.withRLock(func(np *NetworkPolicy) error {
		policyList = np.Policies.EastWestPolicy
		return nil
	})
	return policyList
}

func (m *NpmManager) GetNSObsPolicyList() []Policy {
	var policyList []Policy
	_ = m.npmStore.withRLock(func(np *NetworkPolicy) error {
		policyList = np.Policies.NorthSouthObservePolicy
		return nil
	})
	return policyList
}

func (m *NpmManager) GetNSEnfPolicyList() []Policy {
	var policyList []Policy
	_ = m.npmStore.withRLock(func(np *NetworkPolicy) error {
		policyList = np.Policies.NorthSouthEnforcePolicy
		return nil
	})
	return policyList
}

func (m *NpmManager) GetPolicyChain(policyId string) (string, error) {
	var chainName string
	err := m.npmStore.withRLock(func(np *NetworkPolicy) error {
		for _, p := range np.Policies.EastWestPolicy {
			if p.Id != policyId {
				continue
			}
			chainName = "RAIND-EW"
			return nil
		}
		for _, p := range np.Policies.NorthSouthObservePolicy {
			if p.Id != policyId {
				continue
			}
			chainName = "RAIND-NS-OBS"
			return nil
		}
		for _, p := range np.Policies.NorthSouthEnforcePolicy {
			if p.Id != policyId {
				continue
			}
			chainName = "RAIND-NS-ENF"
			return nil
		}
		return fmt.Errorf("ID: %s not found", policyId)
	})
	return chainName, err
}

func (m *NpmManager) AddPolicy(chainName string, policy Policy) error {
	return m.npmStore.withLock(func(np *NetworkPolicy) error {
		switch chainName {
		case "RAIND-EW":
			np.Policies.EastWestPolicy = append(np.Policies.EastWestPolicy, policy)
		case "RAIND-NS-OBS":
			np.Policies.NorthSouthObservePolicy = append(np.Policies.NorthSouthObservePolicy, policy)
		case "RAIND-NS-ENF":
			np.Policies.NorthSouthEnforcePolicy = append(np.Policies.NorthSouthEnforcePolicy, policy)
		default:
			return fmt.Errorf("Chain: %s is invalid", chainName)
		}
		return nil
	})
}

func (m *NpmManager) RemovePolicy(policyId string) error {
	return m.npmStore.withLock(func(np *NetworkPolicy) error {
		for i, p := range np.Policies.EastWestPolicy {
			if p.Id != policyId {
				continue
			}
			np.Policies.EastWestPolicy = append(np.Policies.EastWestPolicy[:i], np.Policies.EastWestPolicy[i+1:]...)
			return nil
		}
		for i, p := range np.Policies.NorthSouthObservePolicy {
			if p.Id != policyId {
				continue
			}
			np.Policies.NorthSouthObservePolicy = append(np.Policies.NorthSouthObservePolicy[:i], np.Policies.NorthSouthObservePolicy[i+1:]...)
			return nil
		}
		for i, p := range np.Policies.NorthSouthEnforcePolicy {
			if p.Id != policyId {
				continue
			}
			np.Policies.NorthSouthEnforcePolicy = append(np.Policies.NorthSouthEnforcePolicy[:i], np.Policies.NorthSouthEnforcePolicy[i+1:]...)
			return nil
		}
		return fmt.Errorf("ID: %s not found", policyId)
	})
}

func (m *NpmManager) UpdateStatus(chainName string, policyId string, status string, reason string) error {
	return m.npmStore.withLock(func(np *NetworkPolicy) error {
		switch chainName {
		case "RAIND-EW":
			for i, p := range np.Policies.EastWestPolicy {
				if p.Id != policyId {
					continue
				}
				p.Status = status
				p.Reason = reason
				np.Policies.EastWestPolicy[i] = p
				return nil
			}
		case "RAIND-NS-OBS":
			for i, p := range np.Policies.NorthSouthObservePolicy {
				if p.Id != policyId {
					continue
				}
				p.Status = status
				p.Reason = reason
				np.Policies.NorthSouthObservePolicy[i] = p
				return nil
			}
		case "RAIND-NS-ENF":
			for i, p := range np.Policies.NorthSouthEnforcePolicy {
				if p.Id != policyId {
					continue
				}
				p.Status = status
				p.Reason = reason
				np.Policies.NorthSouthEnforcePolicy[i] = p
				return nil
			}
		}
		return fmt.Errorf("ID: %s not found", policyId)
	})
}

func (m *NpmManager) ChangeNSMode(mode string) error {
	return m.npmStore.withLock(func(np *NetworkPolicy) error {
		np.DefaultRule.NorthSouth.Mode = mode
		return nil
	})
}
