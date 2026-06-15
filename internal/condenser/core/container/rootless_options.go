package container

const (
	rootlessModeShiftedRoot = "shifted-root"
	rootlessModeLoginRoot   = "login-root"
)

func normalizeRootlessCreateParameter(param ServiceCreateModel) ServiceCreateModel {
	switch param.RootlessMode {
	case "":
		param.RootlessMode = rootlessModeShiftedRoot
	case rootlessModeShiftedRoot:
		// Keep the default mode unchanged. Rootless is enabled only when requested
		// explicitly, or when a non-default rootless mode is used.
	case rootlessModeLoginRoot:
		param.Rootless = true
	default:
		// Let droplet/spec own validation semantics for unknown modes. Do not
		// silently enable rootless for a mode that may simply be a typo.
	}
	return param
}
