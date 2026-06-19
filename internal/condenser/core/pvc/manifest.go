package pvc

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"raind/internal/condenser/store/vsm"

	"gopkg.in/yaml.v3"
)

const ReclaimPolicyAnnotation = "raind.dev/reclaimPolicy"

var quantityPattern = regexp.MustCompile(`^([1-9][0-9]*)([KMGT]i?|)$`)

type Manifest struct {
	Name             string
	Namespace        string
	Annotations      map[string]string
	AccessModes      []string
	RequestedStorage string
	RequestedBytes   uint64
	StorageClassName string
	VolumeMode       string
	ReclaimPolicy    string
}

type manifestMeta struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace"`
	Annotations map[string]string `yaml:"annotations"`
}

type pvcManifest struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   manifestMeta `yaml:"metadata"`
	Spec       struct {
		AccessModes      []string `yaml:"accessModes"`
		StorageClassName string   `yaml:"storageClassName"`
		VolumeMode       string   `yaml:"volumeMode"`
		Resources        struct {
			Requests map[string]string `yaml:"requests"`
		} `yaml:"resources"`
	} `yaml:"spec"`
}

func DecodeK8sPVCManifest(body []byte) (Manifest, error) {
	dec := yaml.NewDecoder(bytes.NewReader(body))
	for {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return Manifest{}, err
		}
		if len(raw) == 0 {
			continue
		}
		kind, _ := raw["kind"].(string)
		if kind == "" {
			return Manifest{}, fmt.Errorf("kind is required")
		}
		if kind != "PersistentVolumeClaim" {
			return Manifest{}, fmt.Errorf("unsupported kind: %s", kind)
		}
		rawBytes, err := yaml.Marshal(raw)
		if err != nil {
			return Manifest{}, err
		}
		var manifest pvcManifest
		if err := yaml.Unmarshal(rawBytes, &manifest); err != nil {
			return Manifest{}, err
		}
		return buildManifest(manifest)
	}
	return Manifest{}, fmt.Errorf("persistentvolumeclaim manifest not found")
}

func buildManifest(pm pvcManifest) (Manifest, error) {
	if pm.Metadata.Name == "" {
		return Manifest{}, fmt.Errorf("persistentvolumeclaim name is required")
	}
	if pm.Metadata.Namespace == "" {
		pm.Metadata.Namespace = "default"
	}
	accessModes := pm.Spec.AccessModes
	if len(accessModes) == 0 {
		return Manifest{}, fmt.Errorf("persistentvolumeclaim accessModes is required")
	}
	for _, mode := range accessModes {
		if mode != "ReadWriteOnce" {
			return Manifest{}, fmt.Errorf("unsupported persistentvolumeclaim accessMode: %s", mode)
		}
	}
	requested := pm.Spec.Resources.Requests["storage"]
	if requested == "" {
		return Manifest{}, fmt.Errorf("persistentvolumeclaim resources.requests.storage is required")
	}
	bytes, err := ParseStorageQuantity(requested)
	if err != nil {
		return Manifest{}, err
	}
	reclaimPolicy := vsm.PVCReclaimRetain
	if pm.Metadata.Annotations != nil && pm.Metadata.Annotations[ReclaimPolicyAnnotation] != "" {
		reclaimPolicy = pm.Metadata.Annotations[ReclaimPolicyAnnotation]
	}
	if reclaimPolicy != vsm.PVCReclaimRetain && reclaimPolicy != vsm.PVCReclaimDelete {
		return Manifest{}, fmt.Errorf("unsupported persistentvolumeclaim reclaim policy: %s", reclaimPolicy)
	}
	volumeMode := pm.Spec.VolumeMode
	if volumeMode == "" {
		volumeMode = "Filesystem"
	}
	if volumeMode != "Filesystem" {
		return Manifest{}, fmt.Errorf("unsupported persistentvolumeclaim volumeMode: %s", volumeMode)
	}
	return Manifest{
		Name:             pm.Metadata.Name,
		Namespace:        pm.Metadata.Namespace,
		Annotations:      pm.Metadata.Annotations,
		AccessModes:      append([]string{}, accessModes...),
		RequestedStorage: requested,
		RequestedBytes:   bytes,
		StorageClassName: pm.Spec.StorageClassName,
		VolumeMode:       volumeMode,
		ReclaimPolicy:    reclaimPolicy,
	}, nil
}

func ParseStorageQuantity(value string) (uint64, error) {
	value = strings.TrimSpace(value)
	matches := quantityPattern.FindStringSubmatch(value)
	if matches == nil {
		return 0, fmt.Errorf("invalid storage quantity: %s", value)
	}
	n, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid storage quantity: %s", value)
	}
	multiplier := uint64(1)
	switch matches[2] {
	case "":
		multiplier = 1
	case "K":
		multiplier = 1000
	case "M":
		multiplier = 1000 * 1000
	case "G":
		multiplier = 1000 * 1000 * 1000
	case "T":
		multiplier = 1000 * 1000 * 1000 * 1000
	case "Ki":
		multiplier = 1024
	case "Mi":
		multiplier = 1024 * 1024
	case "Gi":
		multiplier = 1024 * 1024 * 1024
	case "Ti":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("invalid storage quantity unit: %s", matches[2])
	}
	if n > ^uint64(0)/multiplier {
		return 0, fmt.Errorf("storage quantity overflows uint64: %s", value)
	}
	return n * multiplier, nil
}
