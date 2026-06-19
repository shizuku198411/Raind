package sec

import (
	"fmt"
	"time"
)

const SecretTypeOpaque = "Opaque"

func NewSecManager(secStore *SecStore) *SecManager {
	return &SecManager{secStore: secStore}
}

type SecManager struct {
	secStore *SecStore
}

func (m *SecManager) StoreSecret(secretId string, spec SecretInfo) error {
	return m.secStore.withLock(func(st *SecretState) error {
		spec.SecretId = secretId
		spec.CreatedAt = time.Now()
		if spec.Namespace == "" {
			spec.Namespace = "default"
		}
		if spec.Type == "" {
			spec.Type = SecretTypeOpaque
		}
		if spec.Data == nil {
			spec.Data = map[string]string{}
		}
		st.Secrets[secretId] = spec
		return nil
	})
}

func (m *SecManager) GetSecretList() ([]SecretInfo, error) {
	var list []SecretInfo
	err := m.secStore.withRLock(func(st *SecretState) error {
		for _, s := range st.Secrets {
			list = append(list, s)
		}
		return nil
	})
	return list, err
}

func (m *SecManager) GetSecretById(secretId string) (SecretInfo, error) {
	var info SecretInfo
	err := m.secStore.withRLock(func(st *SecretState) error {
		s, ok := st.Secrets[secretId]
		if !ok {
			return fmt.Errorf("secretId=%s not found", secretId)
		}
		info = s
		return nil
	})
	return info, err
}

func (m *SecManager) GetSecretByName(name, namespace string) (SecretInfo, error) {
	if namespace == "" {
		namespace = "default"
	}
	var info SecretInfo
	err := m.secStore.withRLock(func(st *SecretState) error {
		for _, s := range st.Secrets {
			if s.Name == name && s.Namespace == namespace {
				info = s
				return nil
			}
		}
		return fmt.Errorf("secret %s/%s not found", namespace, name)
	})
	return info, err
}

func (m *SecManager) RemoveSecret(secretId string) error {
	return m.secStore.withLock(func(st *SecretState) error {
		if _, ok := st.Secrets[secretId]; !ok {
			return fmt.Errorf("secretId=%s not found", secretId)
		}
		delete(st.Secrets, secretId)
		return nil
	})
}

func (m *SecManager) IsNameAlreadyUsed(name, namespace string) bool {
	if namespace == "" {
		namespace = "default"
	}
	var used bool
	_ = m.secStore.withRLock(func(st *SecretState) error {
		for _, s := range st.Secrets {
			if s.Name == name && s.Namespace == namespace {
				used = true
				return nil
			}
		}
		return nil
	})
	return used
}
