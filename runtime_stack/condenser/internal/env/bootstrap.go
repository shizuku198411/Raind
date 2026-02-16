package env

import (
	"bufio"
	"condenser/internal/core/cert"
	"condenser/internal/core/network"
	"condenser/internal/core/policy"
	"condenser/internal/lsm"
	"condenser/internal/store/csm"
	"condenser/internal/store/ilm"
	"condenser/internal/store/ipam"
	"condenser/internal/store/npm"
	"condenser/internal/utils"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func NewBootstrapManager() *BootstrapManager {
	return &BootstrapManager{
		filesystemHandler: utils.NewFilesystemExecutor(),
		commandFactory:    utils.NewCommandFactory(),
		certHandler:       cert.NewCertManager(),
		networkHandler:    network.NewNetworkService(),
		policyHandler:     policy.NewwServicePolicy(),
		ipamStoreHandler:  ipam.NewIpamStore(utils.IpamStorePath),
		ipamHandler:       ipam.NewIpamManager(ipam.NewIpamStore(utils.IpamStorePath)),
		csmStoreHandler:   csm.NewCsmStore(utils.CsmStorePath),
		csmHandler:        csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),
		ilmStoreHandler:   ilm.NewIlmStore(utils.IlmStorePath),
		npmStoreHandler:   npm.NewNpmStore(utils.NpmStorePath),
		appArmorHandler:   lsm.NewAppArmorManager(),
	}
}

type BootstrapManager struct {
	filesystemHandler utils.FilesystemHandler
	commandFactory    utils.CommandFactory
	certHandler       cert.CertHandler
	networkHandler    network.NetworkServiceHandler
	policyHandler     policy.PolicyServiceHandler
	ipamStoreHandler  ipam.IpamStoreHandler
	ipamHandler       ipam.IpamHandler
	csmStoreHandler   csm.CsmStoreHandler
	csmHandler        csm.CsmHandler
	ilmStoreHandler   ilm.IlmStoreHandler
	npmStoreHandler   npm.NpmStoreHandler
	appArmorHandler   lsm.AppArmorHandler
}

func (m *BootstrapManager) SetupRuntime() error {
	// 1. create runtime directory
	if err := m.setupRuntimeDirectory(); err != nil {
		return err
	}

	// 3. setup IPAM (IP Address Managr)
	if err := m.setupIpam(); err != nil {
		return err
	}

	// 4. setup CSM (Container State Manager)
	if err := m.setupCsm(); err != nil {
		return err
	}

	// 5. setup ILM (Image Layer Manager)
	if err := m.setupIlm(); err != nil {
		return err
	}

	// 6. setup NPM (Network Policy Manager)
	if err := m.setupNpm(); err != nil {
		return err
	}

	// 2. setup cgroup
	if err := m.setupCgroup(); err != nil {
		return err
	}
	// 7. setup certificate
	if err := m.setupCertificate(); err != nil {
		return err
	}

	// 6. setup network
	if err := m.setupNetwork(); err != nil {
		return err
	}

	// 7. setup network policy
	if err := m.setupPolicy(); err != nil {
		return err
	}

	// 8. setup AppArmor
	if err := m.setupAppArmor(); err != nil {
		return err
	}

	return nil
}

func (m *BootstrapManager) setupRuntimeDirectory() error {
	dirs := []string{
		utils.ContainerRootDir,
		utils.ImageRootDir,
		utils.LayerRootDir,
		utils.StoreDir,
		utils.AuditLogDir,
		utils.CertDir,
	}
	for _, dir := range dirs {
		if err := m.filesystemHandler.MkdirAll(dir, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (m *BootstrapManager) setupCgroup() error {
	// 1. create cgroup runtime directory
	if err := m.setupCgroupDirectory(); err != nil {
		return err
	}

	// 2. enable controllers
	if err := m.enableCgroupControllers(); err != nil {
		return err
	}

	// 3. create existing container's directory
	if err := m.createContainerCgroup(); err != nil {
		return err
	}

	return nil
}

func (m *BootstrapManager) setupCgroupDirectory() error {
	dir := utils.CgroupRuntimeDir
	if err := m.filesystemHandler.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return nil
}

func (m *BootstrapManager) enableCgroupControllers() error {
	// get current enabled control
	enabled, err := m.readCgroupEnabledControllers()
	if err != nil {
		return nil
	}

	controllers := []string{
		"cpu",
		"memory",
		"pids",
		"io",
	}
	for _, c := range controllers {
		if enabled[c] {
			continue
		}
		if err := m.writeCgroupController("+" + c); err != nil {
			return err
		}
	}

	return nil
}

func (m *BootstrapManager) createContainerCgroup() error {
	containerList, err := m.csmHandler.GetContainerList()
	if err != nil {
		return err
	}
	for _, c := range containerList {
		if err := m.filesystemHandler.MkdirAll(filepath.Join(utils.CgroupRuntimeDir, c.ContainerId), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (m *BootstrapManager) readCgroupEnabledControllers() (map[string]bool, error) {
	subtreePath := utils.CgroupSubtreeControlPath
	f, err := m.filesystemHandler.Open(subtreePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	enabled := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		for _, name := range fields {
			enabled[name] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return enabled, nil
}

func (m *BootstrapManager) writeCgroupController(token string) error {
	subtreePath := utils.CgroupSubtreeControlPath
	f, err := m.filesystemHandler.OpenFile(subtreePath, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%s\n", token); err != nil {
		return err
	}
	return nil
}

func (m *BootstrapManager) setupIpam() error {
	return m.ipamStoreHandler.SetConfig()
}

func (m *BootstrapManager) setupCsm() error {
	return m.csmStoreHandler.SetContainerState()
}

func (m *BootstrapManager) setupIlm() error {
	return m.ilmStoreHandler.SetConfig()
}

func (m *BootstrapManager) setupNpm() error {
	return m.npmStoreHandler.SetNetworkPolicy()
}

func (m *BootstrapManager) setupAppArmor() error {
	if err := m.appArmorHandler.EnsureRaindDefaultProfile(); err != nil {
		// if apparmor setting failed, runtime ignore apparmor setting
		return nil
	}
	return nil
}

func (m *BootstrapManager) setupNetwork() error {
	// 1. create bridge interface
	if err := m.createBridgeInterface(); err != nil {
		return err
	}

	// 2. setup masquerade
	if err := m.createMasqueradeRule(); err != nil {
		return err
	}

	// 3. setup protect rule
	if err := m.createManagementProtectRule(); err != nil {
		return err
	}

	// 4. setup dns proxy
	if err := m.setupDnsProxy(); err != nil {
		return err
	}

	return nil
}

func (m *BootstrapManager) createBridgeInterface() error {
	networkList, err := m.ipamHandler.GetNetworkList()
	if err != nil {
		return err
	}

	for _, n := range networkList {
		if err := m.networkHandler.CreateBridgeInterface(n.Interface, n.Address); err != nil {
			return err
		}
	}
	return nil
}

func (m *BootstrapManager) createMasqueradeRule() error {
	hostInterface, err := m.ipamHandler.GetDefaultInterface()
	if err != nil {
		return err
	}
	runtimeSubnet, err := m.ipamHandler.GetRuntimeSubnet()
	if err != nil {
		return err
	}

	if err := m.networkHandler.CreateMasqueradeRule(runtimeSubnet, hostInterface); err != nil {
		return err
	}

	return nil
}

func (m *BootstrapManager) createManagementProtectRule() error {
	runtimeSubnet, err := m.ipamHandler.GetRuntimeSubnet()
	if err != nil {
		return err
	}
	hostAddr, err := m.ipamHandler.GetDefaultInterfaceAddr()
	if err != nil {
		return err
	}
	hostAddr = strings.Split(hostAddr, "/")[0]

	// allow rule for hook traffic: container -> host:7756
	if err := m.networkHandler.InsertInputRule(
		1,
		network.InputRuleModel{
			SourceAddr: runtimeSubnet,
			DestAddr:   hostAddr,
			Protocol:   "tcp",
			DestPort:   7756,
		},
		"ACCEPT",
	); err != nil {
		return err
	}

	// drop rule for management traffic: container -> host:7755
	if err := m.networkHandler.InsertInputRule(
		2,
		network.InputRuleModel{
			SourceAddr: runtimeSubnet,
			DestAddr:   hostAddr,
			Protocol:   "tcp",
			DestPort:   7755,
		},
		"DROP",
	); err != nil {
		return err
	}

	return nil
}

func (m *BootstrapManager) setupDnsProxy() error {
	proxyIf, proxyAddr, _, err := m.ipamHandler.GetDnsProxyInfo()
	if err != nil {
		return err
	}
	// 1. create dns proxy interface
	if err := m.createDnsProxyInterface(proxyIf, proxyAddr); err != nil {
		return err
	}
	// 2. create redirect from container:53 to proxy:1053
	if err := m.createRedirectDnsRule(proxyAddr); err != nil {
		return err
	}
	return nil
}

func (m *BootstrapManager) createDnsProxyInterface(ifname string, addr string) error {
	if err := m.networkHandler.CreateBridgeInterface(ifname, addr); err != nil {
		return err
	}
	return nil
}

func (m *BootstrapManager) createRedirectDnsRule(addr string) error {
	networkList, err := m.ipamHandler.GetNetworkList()
	if err != nil {
		return err
	}
	for _, n := range networkList {
		if err := m.networkHandler.CreateRedirectDnsTrafficRule(n.Interface, addr); err != nil {
			return err
		}
	}
	return nil
}

func (m *BootstrapManager) setupPolicy() error {
	// 1. setup predefined policy
	if err := m.policyHandler.BuildPredefinedPolicy(); err != nil {
		return err
	}
	// 2. setup user defined policy
	if err := m.policyHandler.BuildUserPolicy(); err != nil {
		return err
	}
	return nil
}

func (m *BootstrapManager) setupCertificate() error {
	// 1. server cert
	hostaddr, err := m.ipamHandler.GetDefaultInterfaceAddr()
	if err != nil {
		return err
	}
	hostaddr = strings.SplitN(hostaddr, "/", 2)[0]

	err = m.certHandler.EnsureSelfSignedCert(
		utils.PublicCertPath,
		utils.PrivateKeyPath,
		cert.CertConfig{
			CommonName: "raind",
			DNSNames: []string{
				"localhost",
			},
			IPAddresses: []net.IP{
				net.ParseIP("127.0.0.1"),
				net.ParseIP(hostaddr),
			},
			ValidFor: 5 * 365 * 24 * time.Hour, // 5 years
		},
	)
	if err != nil {
		return err
	}

	// 2. client CA
	err = m.certHandler.EnsureClientCACert(
		utils.ClientIssuerCACertPath,
		utils.ClientIssuerCAKeyPath,
		cert.CertConfig{
			CommonName: "raind client issuer",
			ValidFor:   5 * 365 * 24 * time.Hour, // 5 years
		},
	)
	if err != nil {
		return err
	}

	// 3. client cert
	err = m.certHandler.IssueClientCert(
		utils.ClientCertPath,
		utils.ClientKeyPath,
		utils.ClientIssuerCACertPath,
		utils.ClientIssuerCAKeyPath,
		cert.ClientCertConfig{
			CommonName: "raind-client",
			SpiiffeId:  "spiffe://raind/cli/admin",
			ValidFor:   1 * 365 * 24 * time.Hour, // 1 year
		},
	)
	if err != nil {
		return err
	}

	// 4. csr request client cert
	err = m.certHandler.IssueClientCert(
		utils.HookClientCertPath,
		utils.HookClientKeyPath,
		utils.ClientIssuerCACertPath,
		utils.ClientIssuerCAKeyPath,
		cert.ClientCertConfig{
			CommonName: "raind-hook-client",
			SpiiffeId:  "spiffe://raind/droplet/container",
			ValidFor:   1 * 365 * 24 * time.Hour, // 1 year
		},
	)
	if err != nil {
		return err
	}

	return nil
}
