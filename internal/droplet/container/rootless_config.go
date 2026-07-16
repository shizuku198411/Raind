package container

import (
	"raind/internal/droplet/container/rootless"
	"raind/internal/droplet/spec"
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
	return rootless.ConfigFromSpec(containerSpec)
}

func rootlessPlanFromSpec(containerSpec spec.Spec) rootless.Plan {
	return rootless.PlanFromSpec(containerSpec)
}

func isRootlessSpec(containerSpec spec.Spec) bool {
	return rootless.IsSpec(containerSpec)
}

func isNonInitialUserNamespace(uidMap string) bool {
	return rootless.IsNonInitialUserNamespace(uidMap)
}

func userNamespaceDiffersFromInit(selfUIDMap string, initUIDMap string) bool {
	return rootless.UserNamespaceDiffersFromInit(selfUIDMap, initUIDMap)
}

func currentUserNamespaceDiffersFromInit() bool {
	return rootless.CurrentUserNamespaceDiffersFromInit()
}

func rootlessModeOrDefault(mode string) string {
	return rootless.ModeOrDefault(mode)
}

func rootlessIDMapPolicyFromConfig(cfg spec.RootlessConfigObject) rootlessIDMapPolicy {
	return rootlessIDMapPolicyFromRootless(rootless.IDMapPolicyFromConfig(cfg))
}

func (p rootlessIDMapPolicy) hostRootID() (uid int, gid int) {
	return p.toRootless().HostRootID()
}

func (p rootlessIDMapPolicy) mapUID(path string, uid int) (int, error) {
	return p.toRootless().MapUID(path, uid)
}

func (p rootlessIDMapPolicy) mapGID(path string, gid int) (int, error) {
	return p.toRootless().MapGID(path, gid)
}

func rootlessHostRootID(cfg spec.RootlessConfigObject) (uid int, gid int) {
	return rootless.HostRootID(cfg)
}

func (p rootlessIDMapPolicy) toRootless() rootless.IDMapPolicy {
	return rootless.NewIDMapPolicy(p.mode, p.uidBase, p.gidBase, p.mapSize, p.rootUID, p.rootGID)
}

func rootlessIDMapPolicyFromRootless(p rootless.IDMapPolicy) rootlessIDMapPolicy {
	return rootlessIDMapPolicy{
		mode:    p.Mode(),
		uidBase: p.UIDBase(),
		gidBase: p.GIDBase(),
		mapSize: p.MapSize(),
		rootUID: p.RootUID(),
		rootGID: p.RootGID(),
	}
}
