package secret

import (
	"fmt"

	"raind/internal/condenser/store/sec"
	"raind/internal/condenser/utils"
)

func NewSecretService() *SecretService {
	return &SecretService{
		secHandler: sec.NewSecManager(sec.NewSecStore(utils.SecStorePath)),
	}
}

type SecretService struct {
	secHandler sec.SecHandler
}

func (s *SecretService) Create(manifest Manifest) (sec.SecretInfo, error) {
	if manifest.Name == "" {
		return sec.SecretInfo{}, fmt.Errorf("secret name is required")
	}
	if manifest.Namespace == "" {
		manifest.Namespace = "default"
	}
	if manifest.Type == "" {
		manifest.Type = sec.SecretTypeOpaque
	}
	if manifest.Type != sec.SecretTypeOpaque {
		return sec.SecretInfo{}, fmt.Errorf("unsupported secret type: %s", manifest.Type)
	}
	if s.secHandler.IsNameAlreadyUsed(manifest.Name, manifest.Namespace) {
		return sec.SecretInfo{}, fmt.Errorf("name already used by other secret")
	}
	secretId := utils.NewUlid()
	info := sec.SecretInfo{
		Name:      manifest.Name,
		Namespace: manifest.Namespace,
		Type:      manifest.Type,
		Data:      manifest.Data,
	}
	if err := s.secHandler.StoreSecret(secretId, info); err != nil {
		return sec.SecretInfo{}, err
	}
	return s.secHandler.GetSecretById(secretId)
}

func (s *SecretService) List(namespace string) ([]sec.SecretInfo, error) {
	list, err := s.secHandler.GetSecretList()
	if err != nil {
		return nil, err
	}
	if namespace == "" {
		return list, nil
	}
	out := make([]sec.SecretInfo, 0, len(list))
	for _, secret := range list {
		if secret.Namespace == namespace {
			out = append(out, secret)
		}
	}
	return out, nil
}

func (s *SecretService) Get(idOrName string, namespace string) (sec.SecretInfo, error) {
	if namespace != "" {
		return s.secHandler.GetSecretByName(idOrName, namespace)
	}
	if info, err := s.secHandler.GetSecretById(idOrName); err == nil {
		return info, nil
	}
	list, err := s.secHandler.GetSecretList()
	if err != nil {
		return sec.SecretInfo{}, err
	}
	var found []sec.SecretInfo
	for _, secret := range list {
		if secret.Name == idOrName {
			found = append(found, secret)
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	if len(found) > 1 {
		return sec.SecretInfo{}, fmt.Errorf("secret name %q exists in multiple namespaces; specify namespace", idOrName)
	}
	return sec.SecretInfo{}, fmt.Errorf("secret %q not found", idOrName)
}

func (s *SecretService) Remove(idOrName string, namespace string) (sec.SecretInfo, error) {
	info, err := s.Get(idOrName, namespace)
	if err != nil {
		return sec.SecretInfo{}, err
	}
	if err := s.secHandler.RemoveSecret(info.SecretId); err != nil {
		return sec.SecretInfo{}, err
	}
	return info, nil
}
