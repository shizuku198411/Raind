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

type Plan struct {
	Config      spec.RootlessConfigObject
	Enabled     bool
	Nested      bool
	Policy      IDMapPolicy
	HostRootUID int
	HostRootGID int
}

func ConfigFromSpec(containerSpec spec.Spec) (spec.RootlessConfigObject, bool) {
	return ConfigFromAnnotation(containerSpec.Annotations)
}

func ConfigFromAnnotation(annotation spec.AnnotationObject) (spec.RootlessConfigObject, bool) {
	if annotation.Rootless == "" {
		return spec.RootlessConfigObject{}, false
	}
	var rootless spec.RootlessConfigObject
	if err := utils.StringToJson(annotation.Rootless, &rootless); err != nil {
		return spec.RootlessConfigObject{}, false
	}
	if !rootless.Enabled {
		return spec.RootlessConfigObject{}, false
	}
	rootless.Mode = ModeOrDefault(rootless.Mode)
	return rootless, true
}

func PlanFromSpec(containerSpec spec.Spec) Plan {
	cfg, ok := ConfigFromSpec(containerSpec)
	if !ok {
		return Plan{}
	}
	return PlanFromConfig(cfg)
}

func PlanFromConfig(cfg spec.RootlessConfigObject) Plan {
	if !cfg.Enabled {
		return Plan{}
	}
	cfg.Mode = ModeOrDefault(cfg.Mode)
	policy := IDMapPolicyFromConfig(cfg)
	hostRootUID, hostRootGID := policy.HostRootID()
	return Plan{
		Config:      cfg,
		Enabled:     true,
		Nested:      CurrentUserNamespaceDiffersFromInit(),
		Policy:      policy,
		HostRootUID: hostRootUID,
		HostRootGID: hostRootGID,
	}
}

func IsSpec(containerSpec spec.Spec) bool {
	_, ok := ConfigFromSpec(containerSpec)
	return ok
}

func IsAnnotation(annotation spec.AnnotationObject) bool {
	_, ok := ConfigFromAnnotation(annotation)
	return ok
}

func (p Plan) ShouldPrepareHostResources() bool {
	return !p.Enabled || !p.Nested
}

func (p Plan) ShouldPrepareHostOwnedFiles() bool {
	return p.Enabled && !p.Nested
}

func (p Plan) ShouldPrejoinNamespaces(hasJoinTargets bool) bool {
	return p.Enabled && hasJoinTargets
}

func ChownToHostRoot(path string, plan Plan) error {
	if !plan.ShouldPrepareHostOwnedFiles() {
		return nil
	}
	return os.Chown(path, plan.HostRootUID, plan.HostRootGID)
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
