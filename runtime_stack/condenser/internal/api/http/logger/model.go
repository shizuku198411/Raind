package logger

import "net/url"

type Logger interface {
	Write(event Event)
}

type Event struct {
	TS            string `json:"generated_ts"`
	EventId       string `json:"event_id"`
	CorrelationId string `json:"correlation_id,omitempty"`
	Severity      string `json:"severity"`

	Actor Actor `json:"actor"`

	Action string `json:"action,omitempty"`
	Target Target `json:"target,omitempty"`

	Request Request `json:"request"`
	Result  Result  `json:"result"`

	Runtime Runtime `json:"runtime"`

	Extra map[string]any `json:"extra,omitempty"`
}

type Actor struct {
	SPIFFEId        string `json:"spiffe_id,omitempty"`
	CertFingerprint string `json:"certt_fingerprint,omitempty"`
	PeerIp          string `json:"peer_ip,omitempty"`
}

type Target struct {
	// container
	ContainerId   string   `json:"container_id,omitempty"`
	ContainerName string   `json:"container_name,omitempty"`
	ImageRef      string   `json:"image_ref,omitempty"`
	Command       []string `json:"command,omitempty"`
	Port          []string `json:"port,omitempty"`
	Mount         []string `json:"mount,omitempty"`
	Network       string   `json:"network,omitempty"`
	Tty           bool     `json:"tty,omitempty"`

	// policy
	PolicyId    string `json:"policy_id,omitempty"`
	ChainName   string `json:"chain,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	DestPort    int    `json:"dport,omitempty"`
	Comment     string `json:"comment,omitempty"`

	// pki
	CommonName string     `json:"common_name,omitempty"`
	SANURIs    []*url.URL `json:"san_uri,omitempty"`
}

type Request struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Host   string `json:"host,omitempty"`
}

type Result struct {
	Status    string `json:"status"`
	Code      int    `json:"code"`
	Reasone   string `json:"reason,omitempty"`
	Bytes     int    `json:"bytes,omitempty"`
	LatencyMs int64  `json:"latence_ms,omitempty"`
}

type Runtime struct {
	Component string `json:"component,omitempty"`
	Node      string `json:"node,omitempty"`
}

type ctxKey int

var Severity = map[int]string{
	0: "information",
	1: "low",
	2: "medium",
	3: "high",
	4: "critical",
}

const (
	SEV_INFO     = 0
	SEV_LOW      = 1
	SEV_MEDIUM   = 2
	SEV_HIGH     = 3
	SEV_CRITICAL = 4
)

type Rule struct {
	Method   string
	Pattern  string
	Action   string
	Severity int
}

var rules = []Rule{
	// container
	{"GET", "/v1/containers", "container.list", SEV_INFO},
	{"GET", "/v1/containers/{containerId}", "container.info", SEV_INFO},
	{"POST", "/v1/containers", "container.create", SEV_MEDIUM},
	{"POST", "/v1/containers/{containerId}/actions/start", "container.start", SEV_MEDIUM},
	{"POST", "/v1/containers/{containerId}/actions/stop", "container.stop", SEV_MEDIUM},
	{"POST", "/v1/containers/{containerId}/actions/exec", "container.exec", SEV_HIGH},
	{"DELETE", "/v1/containers/{containerId}/actions/delete", "container.delete", SEV_HIGH},

	// websocket
	{"GET", "/v1/containers/{containerId}/attach", "ws.attach", SEV_HIGH},
	{"GET", "/v1/containers/{containerId}/exec/attach", "ws.exec.attach", SEV_HIGH},

	// hook
	{"POST", "/v1/hooks/droplet", "hook.apply", SEV_MEDIUM},

	// image
	{"GET", "/v1/images", "image.list", SEV_INFO},
	{"POST", "/v1/images", "image.pull", SEV_MEDIUM},
	{"POST", "/v1/images/build", "image.build", SEV_HIGH},
	{"DELETE", "/v1/images", "image.remove", SEV_HIGH},

	// policy
	{"GET", "/v1/policies/{chain}", "policy.list", SEV_INFO},
	{"POST", "/v1/policies", "policy.add", SEV_MEDIUM},
	{"POST", "/v1/policies/commit", "policy.commit", SEV_CRITICAL},
	{"POST", "/v1/policies/revert", "policy.revert", SEV_MEDIUM},
	{"POST", "/v1/policies/ns/mode", "policy.mode.change", SEV_CRITICAL},
	{"DELETE", "/v1/policies/{policyId}", "policy.delete", SEV_MEDIUM},

	// pki
	{"POST", "/v1/pki/sign", "pki.sign", SEV_HIGH},
}

var actionSeverity = map[string]int{
	"hook.createRuntime":   SEV_MEDIUM,
	"hook.createContainer": SEV_MEDIUM,
	"hook.poststart":       SEV_MEDIUM,
	"hook.stopContainer":   SEV_MEDIUM,
	"hook.poststop":        SEV_MEDIUM,
}
