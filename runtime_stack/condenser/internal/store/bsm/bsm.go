package bsm

import (
	"fmt"
	"time"
)

func NewBsmManager(bsmStore *BsmStore) *BsmManager {
	return &BsmManager{
		bsmStore: bsmStore,
	}
}

type BsmManager struct {
	bsmStore *BsmStore
}

func (m *BsmManager) StoreBottle(bottleId string, bottleName string, services map[string]ServiceSpec, startOrder []string, policies []PolicyInfo) error {
	return m.bsmStore.withLock(func(st *BottleState) error {
		st.Bottles[bottleId] = BottleInfo{
			BottleId:   bottleId,
			BottleName: bottleName,
			Services:   services,
			StartOrder: startOrder,
			Containers: map[string]string{},
			Policies:   policies,
			CreatedAt:  time.Now(),
		}
		return nil
	})
}

func (m *BsmManager) RemoveBottle(bottleId string) error {
	return m.bsmStore.withLock(func(st *BottleState) error {
		for id, b := range st.Bottles {
			if b.BottleId == bottleId {
				delete(st.Bottles, id)
				return nil
			}
		}
		return fmt.Errorf("bottleId=%s not found", bottleId)
	})
}

func (m *BsmManager) UpdateBottleContainers(bottleId string, containers map[string]string) error {
	return m.bsmStore.withLock(func(st *BottleState) error {
		b, ok := st.Bottles[bottleId]
		if !ok {
			return fmt.Errorf("bottleId=%s not found", bottleId)
		}
		b.Containers = containers
		st.Bottles[bottleId] = b
		return nil
	})
}

func (m *BsmManager) UpdateBottleContainer(bottleId string, serviceName string, containerId string) error {
	return m.bsmStore.withLock(func(st *BottleState) error {
		b, ok := st.Bottles[bottleId]
		if !ok {
			return fmt.Errorf("bottleId=%s not found", bottleId)
		}
		if b.Containers == nil {
			b.Containers = map[string]string{}
		}
		b.Containers[serviceName] = containerId
		st.Bottles[bottleId] = b
		return nil
	})
}

func (m *BsmManager) UpdateBottleNetwork(bottleId string, network string, auto bool) error {
	return m.bsmStore.withLock(func(st *BottleState) error {
		b, ok := st.Bottles[bottleId]
		if !ok {
			return fmt.Errorf("bottleId=%s not found", bottleId)
		}
		b.Network = network
		b.NetworkAuto = auto
		st.Bottles[bottleId] = b
		return nil
	})
}

func (m *BsmManager) GetBottleList() ([]BottleInfo, error) {
	var bottleList []BottleInfo
	err := m.bsmStore.withRLock(func(st *BottleState) error {
		for _, b := range st.Bottles {
			bottleList = append(bottleList, b)
		}
		return nil
	})
	return bottleList, err
}

func (m *BsmManager) GetBottleById(bottleId string) (BottleInfo, error) {
	var bottleInfo BottleInfo
	err := m.bsmStore.withRLock(func(st *BottleState) error {
		for _, b := range st.Bottles {
			if b.BottleId != bottleId {
				continue
			}
			bottleInfo = b
			return nil
		}
		return fmt.Errorf("bottle: %s not found", bottleId)
	})
	return bottleInfo, err
}

func (m *BsmManager) GetBottleByName(bottleName string) (BottleInfo, error) {
	var bottleInfo BottleInfo
	err := m.bsmStore.withRLock(func(st *BottleState) error {
		for _, b := range st.Bottles {
			if b.BottleName != bottleName {
				continue
			}
			bottleInfo = b
			return nil
		}
		return fmt.Errorf("bottle: %s not found", bottleName)
	})
	return bottleInfo, err
}

func (m *BsmManager) IsNameAlreadyUsed(name string) bool {
	var result bool
	_ = m.bsmStore.withRLock(func(st *BottleState) error {
		for _, b := range st.Bottles {
			if b.BottleName != name {
				continue
			}
			result = true
			return nil
		}
		result = false
		return nil
	})
	return result
}

func (m *BsmManager) GetBottleIdByName(name string) (string, error) {
	var bottleId string
	err := m.bsmStore.withRLock(func(st *BottleState) error {
		for _, b := range st.Bottles {
			if b.BottleName != name {
				continue
			}
			bottleId = b.BottleId
			return nil
		}
		return fmt.Errorf("bottle: %s not found", name)
	})
	return bottleId, err
}

func (m *BsmManager) GetBottleNameById(bottleId string) (string, error) {
	var bottleName string
	err := m.bsmStore.withRLock(func(st *BottleState) error {
		for _, b := range st.Bottles {
			if b.BottleId != bottleId {
				continue
			}
			bottleName = b.BottleName
			return nil
		}
		return fmt.Errorf("bottle: %s not found", bottleId)
	})
	return bottleName, err
}

func (m *BsmManager) GetBottleIdAndName(str string) (id, name string, err error) {
	bottleId, getNameErr := m.GetBottleIdByName(str)
	bottleName, getIdErr := m.GetBottleNameById(str)
	if getNameErr == nil {
		return bottleId, str, nil
	}
	if getIdErr == nil {
		return str, bottleName, nil
	}
	return "", "", fmt.Errorf("bottle: %s not found", str)
}

func (m *BsmManager) ResolveBottleId(str string) (string, error) {
	var bottleId string
	bottleId, err := m.GetBottleIdByName(str)
	if err != nil {
		if _, err := m.GetBottleById(str); err != nil {
			return "", err
		}
		bottleId = str
	}
	return bottleId, nil
}

func (m *BsmManager) IsBottleExist(str string) bool {
	_, getNameErr := m.GetBottleIdByName(str)
	_, getIdErr := m.GetBottleNameById(str)
	if getNameErr != nil && getIdErr != nil {
		return false
	}
	return true
}
