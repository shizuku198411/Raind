package nsm

import (
	"fmt"
	"sort"
	"time"
)

func NewNsmManager(nsmStore *NsmStore) *NsmManager {
	return &NsmManager{
		nsmStore: nsmStore,
	}
}

type NsmManager struct {
	nsmStore *NsmStore
}

func (m *NsmManager) EnsureDefaultNamespace() error {
	return m.nsmStore.SetNamespaceState()
}

func (m *NsmManager) StoreNamespace(info NamespaceInfo) error {
	return m.nsmStore.withLock(func(st *NamespaceState) error {
		if info.Name == "" {
			return fmt.Errorf("namespace name is required")
		}
		if _, ok := st.Namespace[info.Name]; ok {
			return fmt.Errorf("namespace already exists: %s", info.Name)
		}
		if info.CreatedAt.IsZero() {
			info.CreatedAt = time.Now()
		}
		st.Namespace[info.Name] = info
		return nil
	})
}

func (m *NsmManager) GetNamespace(name string) (NamespaceInfo, error) {
	var info NamespaceInfo
	err := m.nsmStore.withRLock(func(st *NamespaceState) error {
		ns, ok := st.Namespace[name]
		if !ok {
			return fmt.Errorf("namespace not found: %s", name)
		}
		info = ns
		return nil
	})
	return info, err
}

func (m *NsmManager) GetNamespaceList() ([]NamespaceInfo, error) {
	var list []NamespaceInfo
	err := m.nsmStore.withRLock(func(st *NamespaceState) error {
		for _, ns := range st.Namespace {
			list = append(list, ns)
		}
		sort.Slice(list, func(i, j int) bool {
			return list[i].Name < list[j].Name
		})
		return nil
	})
	return list, err
}

func (m *NsmManager) RemoveNamespace(name string) error {
	return m.nsmStore.withLock(func(st *NamespaceState) error {
		if name == DefaultNamespace {
			return fmt.Errorf("default namespace cannot be removed")
		}
		if _, ok := st.Namespace[name]; !ok {
			return fmt.Errorf("namespace not found: %s", name)
		}
		delete(st.Namespace, name)
		return nil
	})
}

func (m *NsmManager) IsNamespaceExist(name string) bool {
	_, err := m.GetNamespace(name)
	return err == nil
}

func ensureDefaultNamespace(namespaces map[string]NamespaceInfo) {
	if _, ok := namespaces[DefaultNamespace]; ok {
		return
	}
	namespaces[DefaultNamespace] = NamespaceInfo{
		Name:        DefaultNamespace,
		Network:     DefaultNamespaceNetwork,
		NetworkAuto: false,
		CreatedAt:   time.Now(),
	}
}
