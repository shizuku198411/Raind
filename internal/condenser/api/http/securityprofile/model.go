package securityprofile

import core "raind/internal/condenser/core/securityprofile"

type ListSecurityProfileResponse struct {
	Profiles []core.ProfileSummary `json:"profiles"`
}

type ShowSecurityProfileResponse struct {
	Profile core.SecurityProfile `json:"profile"`
}
