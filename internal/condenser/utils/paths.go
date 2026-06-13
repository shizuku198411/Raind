package utils

const (
	RootDir          = "/etc/raind"
	AuditLogDir      = "/etc/raind/log/"
	ContainerRootDir = "/etc/raind/container"
	ImageRootDir     = "/etc/raind/image"
	LayerRootDir     = "/etc/raind/image/layers"
	VarLogDir        = "/var/log/raind"

	StoreDir      = "/etc/raind/store"
	IpamStorePath = "/etc/raind/store/ipam.json"
	CsmStorePath  = "/etc/raind/store/csm.json"
	PsmStorePath  = "/etc/raind/store/psm.json"
	IlmStorePath  = "/etc/raind/store/ilm.json"
	SsmStorePath  = "/etc/raind/store/ssm.json"
	NpmStorePath  = "/etc/raind/store/npm.json"
	BsmStorePath  = "/etc/raind/store/bsm.json"
	NsmStorePath  = "/etc/raind/store/nsm.json"

	CgroupRuntimeDir         = "/sys/fs/cgroup/raind"
	CgroupSubtreeControlPath = "/sys/fs/cgroup/raind/cgroup.subtree_control"

	CertDir                = "/etc/raind/cert"
	CliCertDir             = "/etc/raind/cli"
	PublicCertPath         = "/etc/raind/cert/raind.crt"
	PrivateKeyPath         = "/etc/raind/cert/raind.key"
	CliPublicCertPath      = "/etc/raind/cli/ca.crt"
	CliClientCertPath      = "/etc/raind/cli/client.crt"
	CliClientKeyPath       = "/etc/raind/cli/client.key"
	ClientIssuerCACertPath = "/etc/raind/cert/raindClientCA.crt"
	ClientIssuerCAKeyPath  = "/etc/raind/cert/raindClientCA.key"
	ClientCertPath         = "/etc/raind/cert/raindClient.crt"
	ClientKeyPath          = "/etc/raind/cert/raindClient.key"
	HookClientCertPath     = "/etc/raind/cert/raindHookClient.crt"
	HookClientKeyPath      = "/etc/raind/cert/raindHookClient.key"

	UlogPath        = "/var/log/ulog/raind.jsonl"
	AuditLogPath    = "/var/log/raind/raind_audit.jsonl"
	EnrichedLogPath = "/var/log/raind/raind_netflow.jsonl"
	DnsLogPath      = "/var/log/raind/raind_dns.jsonl"
	MetricsLogPath  = "/var/log/raind/raind_metrics.jsonl"

	RaindGroupName = "raind"

	PodInfraImage               = "registry.k8s.io/pause:3.9"
	PodInfraContainerNamePrefix = "condenser-pod-infra-"
)
