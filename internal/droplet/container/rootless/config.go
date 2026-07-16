package rootless

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
)

type IDMapPolicy struct {
	mode    string
	uidBase int
	gidBase int
	mapSize int
	rootUID int
	rootGID int
}

func ConfigFromSpec(containerSpec spec.Spec) (spec.RootlessConfigObject, bool) {
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
	rootless.Mode = ModeOrDefault(rootless.Mode)
	return rootless, true
}

func IsSpec(containerSpec spec.Spec) bool {
	_, ok := ConfigFromSpec(containerSpec)
	return ok
}

func IsNonInitialUserNamespace(uidMap string) bool {
	for _, line := range strings.Split(uidMap, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		containerID, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil || containerID != 0 {
			continue
		}
		hostID, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return false
		}
		return hostID != 0
	}
	return false
}

func UserNamespaceDiffersFromInit(selfUIDMap string, initUIDMap string) bool {
	selfUIDMap = strings.TrimSpace(selfUIDMap)
	initUIDMap = strings.TrimSpace(initUIDMap)
	return selfUIDMap != "" && initUIDMap != "" && selfUIDMap != initUIDMap
}

func CurrentUserNamespaceDiffersFromInit() bool {
	selfUIDMap, err := os.ReadFile("/proc/self/uid_map")
	if err != nil {
		return false
	}
	initUIDMap, err := os.ReadFile("/proc/1/uid_map")
	if err != nil {
		return false
	}
	return UserNamespaceDiffersFromInit(string(selfUIDMap), string(initUIDMap))
}

func ModeOrDefault(mode string) string {
	switch mode {
	case "", spec.RootlessModeShiftedRoot:
		return spec.RootlessModeShiftedRoot
	case spec.RootlessModeLoginRoot:
		return spec.RootlessModeLoginRoot
	default:
		return spec.RootlessModeShiftedRoot
	}
}

func IDMapPolicyFromConfig(cfg spec.RootlessConfigObject) IDMapPolicy {
	uidBase, gidBase, mapSize := rootlessIDMapConfig()
	policy := IDMapPolicy{
		mode:    ModeOrDefault(cfg.Mode),
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
		policy.rootUID, policy.rootGID = policy.HostRootID()
	}
	return policy
}

func (p IDMapPolicy) HostRootID() (uid int, gid int) {
	if p.mode == spec.RootlessModeLoginRoot {
		return p.rootUID, p.rootGID
	}
	return p.uidBase, p.gidBase
}

func (p IDMapPolicy) MapUID(path string, uid int) (int, error) {
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

func (p IDMapPolicy) MapGID(path string, gid int) (int, error) {
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

func HostRootID(cfg spec.RootlessConfigObject) (uid int, gid int) {
	return IDMapPolicyFromConfig(cfg).HostRootID()
}

func NewIDMapPolicy(mode string, uidBase int, gidBase int, mapSize int, rootUID int, rootGID int) IDMapPolicy {
	return IDMapPolicy{
		mode:    ModeOrDefault(mode),
		uidBase: uidBase,
		gidBase: gidBase,
		mapSize: mapSize,
		rootUID: rootUID,
		rootGID: rootGID,
	}
}

func (p IDMapPolicy) Mode() string {
	return p.mode
}

func (p IDMapPolicy) UIDBase() int {
	return p.uidBase
}

func (p IDMapPolicy) GIDBase() int {
	return p.gidBase
}

func (p IDMapPolicy) MapSize() int {
	return p.mapSize
}

func (p IDMapPolicy) RootUID() int {
	return p.rootUID
}

func (p IDMapPolicy) RootGID() int {
	return p.rootGID
}
