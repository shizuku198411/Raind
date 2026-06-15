package container

import (
	"fmt"
	"os"

	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
)

type rootlessIDMapPolicy struct {
	mode    string
	uidBase int
	gidBase int
	mapSize int
	rootUID int
	rootGID int
}

func rootlessConfigFromSpec(containerSpec spec.Spec) (spec.RootlessConfigObject, bool) {
	if containerSpec.Annotations.Rootless == "" {
		return spec.RootlessConfigObject{}, false
	}
	var rootless spec.RootlessConfigObject
	if err := utils.StringToJson(containerSpec.Annotations.Rootless, &rootless); err != nil {
		return spec.RootlessConfigObject{}, false
	}
	if !rootless.Enabled {
		return spec.RootlessConfigObject{}, false
	}
	rootless.Mode = rootlessModeOrDefault(rootless.Mode)
	return rootless, true
}

func isRootlessSpec(containerSpec spec.Spec) bool {
	_, ok := rootlessConfigFromSpec(containerSpec)
	return ok
}

func rootlessModeOrDefault(mode string) string {
	switch mode {
	case "", spec.RootlessModeShiftedRoot:
		return spec.RootlessModeShiftedRoot
	case spec.RootlessModeLoginRoot:
		return spec.RootlessModeLoginRoot
	default:
		return spec.RootlessModeShiftedRoot
	}
}

func rootlessIDMapPolicyFromConfig(cfg spec.RootlessConfigObject) rootlessIDMapPolicy {
	uidBase, gidBase, mapSize := rootlessIDMapConfig()
	policy := rootlessIDMapPolicy{
		mode:    rootlessModeOrDefault(cfg.Mode),
		uidBase: uidBase,
		gidBase: gidBase,
		mapSize: mapSize,
		rootUID: cfg.HostRootUID,
		rootGID: cfg.HostRootGID,
	}
	if policy.mode == spec.RootlessModeLoginRoot {
		if policy.rootUID <= 0 {
			policy.rootUID = os.Getuid()
		}
		if policy.rootGID <= 0 {
			policy.rootGID = os.Getgid()
		}
	} else {
		policy.rootUID, policy.rootGID = policy.hostRootID()
	}
	return policy
}

func (p rootlessIDMapPolicy) hostRootID() (uid int, gid int) {
	if p.mode == spec.RootlessModeLoginRoot {
		return p.rootUID, p.rootGID
	}
	return p.uidBase, p.gidBase
}

func (p rootlessIDMapPolicy) mapUID(path string, uid int) (int, error) {
	if uid < 0 || uid >= p.mapSize {
		return 0, fmt.Errorf("uid outside rootless map: path=%s uid=%d map_size=%d", path, uid, p.mapSize)
	}
	if p.mode == spec.RootlessModeLoginRoot {
		if uid == 0 {
			return p.rootUID, nil
		}
		return p.uidBase + uid - 1, nil
	}
	return p.uidBase + uid, nil
}

func (p rootlessIDMapPolicy) mapGID(path string, gid int) (int, error) {
	if gid < 0 || gid >= p.mapSize {
		return 0, fmt.Errorf("gid outside rootless map: path=%s gid=%d map_size=%d", path, gid, p.mapSize)
	}
	if p.mode == spec.RootlessModeLoginRoot {
		if gid == 0 {
			return p.rootGID, nil
		}
		return p.gidBase + gid - 1, nil
	}
	return p.gidBase + gid, nil
}

func rootlessHostRootID(cfg spec.RootlessConfigObject) (uid int, gid int) {
	return rootlessIDMapPolicyFromConfig(cfg).hostRootID()
}
