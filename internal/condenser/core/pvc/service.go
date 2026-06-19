package pvc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"raind/internal/condenser/store/psm"
	"raind/internal/condenser/store/vsm"
	"raind/internal/condenser/utils"
)

func NewService() *Service {
	return &Service{
		vsmHandler: vsm.NewManager(vsm.NewStore(utils.VsmStorePath)),
		psmHandler: psm.NewPsmManager(psm.NewPsmStore(utils.PsmStorePath)),
		fs:         utils.NewFilesystemExecutor(),
	}
}

type Service struct {
	vsmHandler vsm.Handler
	psmHandler psm.PsmHandler
	fs         utils.FilesystemHandler
}

func (s *Service) Create(manifest Manifest) (vsm.PersistentVolumeClaimInfo, error) {
	if manifest.Name == "" {
		return vsm.PersistentVolumeClaimInfo{}, fmt.Errorf("persistentvolumeclaim name is required")
	}
	if manifest.Namespace == "" {
		manifest.Namespace = "default"
	}
	if s.vsmHandler.IsNameAlreadyUsed(manifest.Name, manifest.Namespace) {
		return vsm.PersistentVolumeClaimInfo{}, fmt.Errorf("name already used by other persistentvolumeclaim")
	}
	pvcId := utils.NewUlid()
	hostPath := filepath.Join(utils.PVCVolumeRootDir, manifest.Namespace, pvcId)
	dataPath := filepath.Join(hostPath, "data")
	if err := s.ensureRuntimeOwnedPath(dataPath); err != nil {
		return vsm.PersistentVolumeClaimInfo{}, err
	}
	if err := s.fs.MkdirAll(dataPath, 0o750); err != nil {
		return vsm.PersistentVolumeClaimInfo{}, fmt.Errorf("create pvc data path: %w", err)
	}
	_ = s.fs.Chmod(hostPath, 0o750)
	_ = s.fs.Chmod(dataPath, 0o750)

	info := vsm.PersistentVolumeClaimInfo{
		Name:             manifest.Name,
		Namespace:        manifest.Namespace,
		AccessModes:      append([]string{}, manifest.AccessModes...),
		RequestedStorage: manifest.RequestedStorage,
		RequestedBytes:   manifest.RequestedBytes,
		StorageClassName: manifest.StorageClassName,
		VolumeMode:       manifest.VolumeMode,
		HostPath:         hostPath,
		DataPath:         dataPath,
		ReclaimPolicy:    manifest.ReclaimPolicy,
		Phase:            vsm.PVCPhaseBound,
	}
	if err := s.writeMetadata(hostPath, info); err != nil {
		_ = s.fs.RemoveAll(hostPath)
		return vsm.PersistentVolumeClaimInfo{}, err
	}
	if err := s.vsmHandler.StorePVC(pvcId, info); err != nil {
		_ = s.fs.RemoveAll(hostPath)
		return vsm.PersistentVolumeClaimInfo{}, err
	}
	return s.vsmHandler.GetPVCById(pvcId)
}

func (s *Service) List(namespace string) ([]vsm.PersistentVolumeClaimInfo, error) {
	list, err := s.vsmHandler.GetPVCList()
	if err != nil {
		return nil, err
	}
	if namespace != "" {
		filtered := make([]vsm.PersistentVolumeClaimInfo, 0, len(list))
		for _, pvc := range list {
			if pvc.Namespace == namespace {
				filtered = append(filtered, pvc)
			}
		}
		list = filtered
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Namespace != list[j].Namespace {
			return list[i].Namespace < list[j].Namespace
		}
		return list[i].Name < list[j].Name
	})
	return list, nil
}

func (s *Service) Get(idOrName, namespace string) (vsm.PersistentVolumeClaimInfo, error) {
	if namespace != "" {
		return s.vsmHandler.GetPVCByName(idOrName, namespace)
	}
	if info, err := s.vsmHandler.GetPVCById(idOrName); err == nil {
		return info, nil
	}
	list, err := s.vsmHandler.GetPVCList()
	if err != nil {
		return vsm.PersistentVolumeClaimInfo{}, err
	}
	var found []vsm.PersistentVolumeClaimInfo
	for _, info := range list {
		if info.Name == idOrName {
			found = append(found, info)
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	if len(found) > 1 {
		return vsm.PersistentVolumeClaimInfo{}, fmt.Errorf("persistentvolumeclaim name %q exists in multiple namespaces; specify namespace", idOrName)
	}
	return vsm.PersistentVolumeClaimInfo{}, fmt.Errorf("persistentvolumeclaim %q not found", idOrName)
}

func (s *Service) Remove(idOrName, namespace string) (vsm.PersistentVolumeClaimInfo, error) {
	info, err := s.Get(idOrName, namespace)
	if err != nil {
		return vsm.PersistentVolumeClaimInfo{}, err
	}
	if err := s.ensureNoRunningPodReference(info); err != nil {
		return vsm.PersistentVolumeClaimInfo{}, err
	}
	if err := s.vsmHandler.RemovePVC(info.PVCId); err != nil {
		return vsm.PersistentVolumeClaimInfo{}, err
	}
	if info.ReclaimPolicy == vsm.PVCReclaimDelete {
		if err := s.ensureRuntimeOwnedPath(info.HostPath); err != nil {
			return vsm.PersistentVolumeClaimInfo{}, err
		}
		if err := s.fs.RemoveAll(info.HostPath); err != nil {
			return vsm.PersistentVolumeClaimInfo{}, fmt.Errorf("remove pvc data path: %w", err)
		}
	}
	return info, nil
}

func (s *Service) writeMetadata(hostPath string, info vsm.PersistentVolumeClaimInfo) error {
	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return s.fs.WriteFile(filepath.Join(hostPath, "metadata.json"), b, 0o644)
}

func (s *Service) ensureRuntimeOwnedPath(path string) error {
	root, err := filepath.Abs(utils.PVCVolumeRootDir)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." || filepath.IsAbs(rel) {
		return fmt.Errorf("pvc path escapes runtime volume root: %s", path)
	}
	return nil
}

func (s *Service) ensureNoRunningPodReference(info vsm.PersistentVolumeClaimInfo) error {
	pods, err := s.psmHandler.GetPodList()
	if err != nil {
		return err
	}
	for _, pod := range pods {
		if pod.State != psm.PodStateRunning {
			continue
		}
		if podReferencesDataPath(s.psmHandler, pod, info.DataPath) {
			return fmt.Errorf("persistentvolumeclaim %s/%s is still used by running pod %s", info.Namespace, info.Name, pod.Name)
		}
	}
	return nil
}

func podReferencesDataPath(handler psm.PsmHandler, pod psm.PodInfo, dataPath string) bool {
	if pod.TemplateId == "" {
		return false
	}
	tpl, err := handler.GetPodTemplate(pod.TemplateId)
	if err != nil {
		return false
	}
	prefix := dataPath + ":"
	for _, container := range tpl.Spec.Containers {
		for _, mount := range container.Mount {
			if mount == dataPath || strings.HasPrefix(mount, prefix) {
				return true
			}
		}
	}
	return false
}
