package policy

import (
	"condenser/internal/store/csm"
	"condenser/internal/store/ipam"
	"condenser/internal/store/npm"
	"condenser/internal/utils"
	"fmt"
	"strings"
)

func NewwServicePolicy() *ServicePolicy {
	return &ServicePolicy{
		ipamHandler:     ipam.NewIpamManager(ipam.NewIpamStore(utils.IpamStorePath)),
		npmHandler:      npm.NewNpmManager(npm.NewNpmStore(utils.NpmStorePath)),
		npmStoreHandler: npm.NewNpmStore(utils.NpmStorePath),
		csmHandler:      csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),

		iptablesHandler: NewIptablesManager(),
	}
}

type ServicePolicy struct {
	ipamHandler     ipam.IpamHandler
	npmHandler      npm.NpmHandler
	npmStoreHandler npm.NpmStoreHandler
	csmHandler      csm.CsmHandler

	iptablesHandler IptablesHandler
}

func (s *ServicePolicy) AddUserPolicy(param ServiceAddPolicyModel) (string, error) {
	policyId := utils.NewUlid()
	status := "before_commit"

	// resolve container id/name
	var (
		srcHost npm.HostInfo
		dstHost npm.HostInfo
	)

	if param.ChainName == "RAIND-EW" {
		_, srcContainerName := s.resolveContainerNameAndInfo(param.Source)
		_, dstContainerName := s.resolveContainerNameAndInfo(param.Destination)
		srcHost = npm.HostInfo{
			ContainerName: srcContainerName,
		}
		dstHost = npm.HostInfo{
			ContainerName: dstContainerName,
		}
	} else {
		_, srcContainerName := s.resolveContainerNameAndInfo(param.Source)
		srcHost = npm.HostInfo{
			ContainerName: srcContainerName,
		}
		dstHost = npm.HostInfo{
			Address: param.Destination,
		}
	}

	if err := s.npmHandler.AddPolicy(
		param.ChainName,
		npm.Policy{
			Id:          policyId,
			Status:      status,
			Source:      srcHost,
			Destination: dstHost,
			Protocol:    param.Protocol,
			DestPort:    param.DestPort,
			Comment:     param.Comment,
		},
	); err != nil {
		return "", err
	}
	return policyId, nil
}

func (s *ServicePolicy) RemoveUserPolicy(param ServiceRemovePolicyModel) error {
	// get policy chain
	chainName, err := s.npmHandler.GetPolicyChain(param.Id)
	if err != nil {
		return err
	}

	// update status
	// set status: remove_next_commit, reason: user operation
	if err := s.npmHandler.UpdateStatus(
		chainName,
		param.Id,
		"remove_next_commit",
		"user operation",
	); err != nil {
		return err
	}
	return nil
}

func (s *ServicePolicy) CommitPolicy() error {
	// re-build using current policy setting
	// 1. fix ns mode
	if err := s.FixNSMode(); err != nil {
		return err
	}
	// 2. predefined policyy
	if err := s.BuildPredefinedPolicy(); err != nil {
		return err
	}
	// 3. user policy
	if err := s.BuildUserPolicy(); err != nil {
		return err
	}
	// 4. baclup
	if err := s.npmStoreHandler.Backup(); err != nil {
		return err
	}
	return nil
}

func (s *ServicePolicy) RevertPolicy() error {
	if err := s.npmStoreHandler.Revert(); err != nil {
		return err
	}
	return nil
}

func (s *ServicePolicy) GetPolicyList(param ServiceListModel) PolicyListModel {
	var (
		mode     string
		total    int
		policies []PolicyInfo
	)

	switch param.Chain {
	case "RAIND-EW":
		mode = s.npmHandler.GetEWMode()
		policyList := s.npmHandler.GetEWPolicyList()
		total = len(policyList)

		for _, p := range policyList {
			policies = append(policies, PolicyInfo{
				Id:     p.Id,
				Status: p.Status,
				Reason: p.Reason,
				Source: HostInfo{
					ContainerName: p.Source.ContainerName,
				},
				Destination: HostInfo{
					ContainerName: p.Destination.ContainerName,
				},
				Protocol: p.Protocol,
				DestPort: p.DestPort,
				Comment:  p.Comment,
			})
		}

	case "RAIND-NS-OBS":
		mode = s.npmHandler.GetNSMode()
		policyList := s.npmHandler.GetNSObsPolicyList()
		total = len(policyList)

		for _, p := range policyList {
			policies = append(policies, PolicyInfo{
				Id:     p.Id,
				Status: p.Status,
				Reason: p.Reason,
				Source: HostInfo{
					ContainerName: p.Source.ContainerName,
				},
				Destination: HostInfo{
					Address: p.Destination.Address,
				},
				Protocol: p.Protocol,
				DestPort: p.DestPort,
				Comment:  p.Comment,
			})
		}

	case "RAIND-NS-ENF":
		mode = s.npmHandler.GetNSMode()
		policyList := s.npmHandler.GetNSEnfPolicyList()
		total = len(policyList)

		for _, p := range policyList {
			policies = append(policies, PolicyInfo{
				Id:     p.Id,
				Status: p.Status,
				Reason: p.Reason,
				Source: HostInfo{
					ContainerName: p.Source.ContainerName,
				},
				Destination: HostInfo{
					Address: p.Destination.Address,
				},
				Protocol: p.Protocol,
				DestPort: p.DestPort,
				Comment:  p.Comment,
			})
		}

	default:
		return PolicyListModel{}
	}

	return PolicyListModel{
		Mode:          mode,
		PoliciesTotal: total,
		Policies:      policies,
	}
}

func (s *ServicePolicy) ChangeNSMode(mode string) error {
	if mode != "enforce" && mode != "observe" {
		return fmt.Errorf("invalid mode: %s", mode)
	}
	// check current mode
	current := s.npmHandler.GetNSMode()
	if current == mode {
		return fmt.Errorf("NS Mode already set in: %s", current)
	}
	// set next commit flag
	mode = mode + "_next_commit"
	if err := s.npmHandler.ChangeNSMode(mode); err != nil {
		return err
	}
	return nil
}

func (s *ServicePolicy) resolveContainerNameAndInfo(str string) (string, string) {
	var (
		containerId   string
		containerName string
	)
	got, err := s.csmHandler.GetContainerIdByName(str)
	if err == nil {
		containerName = str
		containerId = got
	} else {
		got, err := s.csmHandler.GetContainerNameById(str)
		if err == nil {
			containerName = got
			containerId = str
		} else {
			// assuming the source is the container name to be built in the future,
			// set value as contianer name with empty container id
			containerId = ""
			containerName = str
		}
	}
	return containerId, containerName
}

func (s *ServicePolicy) FixNSMode() error {
	current := s.npmHandler.GetNSMode()
	if current == "observe" || current == "enforce" {
		return nil
	}
	if strings.Contains(current, "_next_commit") {
		mode := strings.Split(current, "_")[0]
		if err := s.npmHandler.ChangeNSMode(mode); err != nil {
			return err
		}
	}
	return nil
}

func (s *ServicePolicy) BuildPredefinedPolicy() error {
	// raind default chains
	// 1. RAIND-ROOT
	//      manage all Raind Rules
	// 2. RAIND-EW
	//      manage East-West(contaier-to-container) traffic rules
	// 3. RAIND-NS-OBS
	//      manage North-West(container-to-external) traffic rules
	//      this chain is all accept and log traffic default
	// 4. RAIND-NS-ENF
	//      manage North-West(container-to-external) traffic rules
	//      this chain is all denay and allow explicit

	// == setup predefined rules ==//
	networkList, err := s.ipamHandler.GetNetworkList()
	if err != nil {
		return err
	}

	// ns mode (enforce:true or observe:false)
	enforce := s.npmHandler.IsNsEnforce()
	// logging mode
	loggingEw := s.npmHandler.GetEWLogging()
	loggingNs := s.npmHandler.GetNSLogging()

	// 1. create chain
	if err := s.createChain(); err != nil {
		return err
	}

	// 2. insert chain to forward
	if err := s.insertForward(); err != nil {
		return err
	}

	// 3. build RAIND-ROOT chain
	if err := s.buildRaindRootChain(enforce); err != nil {
		return err
	}

	// 4. build RAIND-EW chain
	if err := s.buildRaindEWChain(networkList, loggingEw); err != nil {
		return err
	}

	if enforce {
		// 5. build RAIND-NS-ENF chain
		if err := s.buildRaindNSEnforceChain(networkList, loggingNs); err != nil {
			return err
		}
	} else {
		// 5. build RAIND-NS-OBS chain
		if err := s.buildRaindNSObserveChain(networkList, loggingNs); err != nil {
			return err
		}
	}

	return nil
}

func (s *ServicePolicy) createChain() error {
	chains := []string{
		"RAIND-ROOT",
		"RAIND-EW",
		"RAIND-NS-OBS",
		"RAIND-NS-ENF",
	}
	for _, c := range chains {
		if err := s.iptablesHandler.CreateChain(c); err != nil {
			return err
		}
	}
	return nil
}

func (s *ServicePolicy) insertForward() error {
	if err := s.iptablesHandler.InsertForwardRule("RAIND-ROOT"); err != nil {
		return err
	}
	return nil
}

func (s *ServicePolicy) buildRaindRootChain(enforce bool) error {
	chainName := "RAIND-ROOT"
	// 1. allow return traffic (ESTABLISHED,RELATED)
	if err := s.iptablesHandler.AddRuleToChain(
		chainName,
		RuleModel{
			Conntrack: true,
			Ctstate:   []string{"ESTABLISHED", "RELATED"},
		},
		"ACCEPT",
	); err != nil {
		return err
	}

	// 2. forward to RAIND-EW
	if err := s.iptablesHandler.AddRuleToChain(
		chainName,
		RuleModel{},
		"RAIND-EW",
	); err != nil {
		return err
	}

	if enforce {
		// 3. forward to RAIND-NS-ENF
		if err := s.iptablesHandler.AddRuleToChain(
			chainName,
			RuleModel{},
			"RAIND-NS-ENF",
		); err != nil {
			return err
		}
	} else {
		// 3. forward to RAIND-NS-OBS
		if err := s.iptablesHandler.AddRuleToChain(
			chainName,
			RuleModel{},
			"RAIND-NS-OBS",
		); err != nil {
			return err
		}
	}

	// 4. other: return
	if err := s.iptablesHandler.AddRuleToChain(
		chainName,
		RuleModel{},
		"RETURN",
	); err != nil {
		return err
	}

	return nil
}

func (s *ServicePolicy) buildRaindEWChain(networkList []ipam.NetworkList, nflog bool) error {
	chainName := "RAIND-EW"

	// 1. if NFLOG mode is enabled, add NFLOG entry
	if nflog {
		for _, n := range networkList {
			if err := s.iptablesHandler.AddRuleToChain(
				chainName,
				RuleModel{
					Conntrack:       true,
					Ctstate:         []string{"NEW"},
					Physdev:         true,
					PhysdevIsBridge: true,
					InputDev:        n.Interface,
					OutputDev:       n.Interface,
					NflogGroup:      10,
					NflogPrefix:     "RAIND-EW-DENY,id=predefined",
				},
				"NFLOG",
			); err != nil {
				return err
			}
		}
	}

	// 2. deny all inter-container traffic
	for _, n := range networkList {
		if err := s.iptablesHandler.AddRuleToChain(
			chainName,
			RuleModel{
				InputDev:  n.Interface,
				OutputDev: n.Interface,
			},
			"DROP",
		); err != nil {
			return err
		}
	}

	// 3. deny all inter-bridge traffic
	if nflog {
		if err := s.iptablesHandler.AddRuleToChain(
			chainName,
			RuleModel{
				Conntrack:   true,
				Ctstate:     []string{"NEW"},
				InputDev:    "raind+",
				OutputDev:   "raind+",
				NflogGroup:  10,
				NflogPrefix: "RAIND-EW-DENY,id=predefined",
			},
			"NFLOG",
		); err != nil {
			return err
		}
	}
	if err := s.iptablesHandler.AddRuleToChain(
		chainName,
		RuleModel{
			InputDev:  "raind+",
			OutputDev: "raind+",
		},
		"DROP",
	); err != nil {
		return err
	}

	// 4. other: return
	if err := s.iptablesHandler.AddRuleToChain(
		chainName,
		RuleModel{},
		"RETURN",
	); err != nil {
		return err
	}

	return nil
}

func (s *ServicePolicy) buildRaindNSObserveChain(networkList []ipam.NetworkList, nflog bool) error {
	chainName := "RAIND-NS-OBS"
	hostInterface, err := s.ipamHandler.GetDefaultInterface()
	if err != nil {
		return err
	}

	// 1. if NFLOG mode is enabled, add NFLOG entry
	for _, n := range networkList {
		if nflog {
			// from external to container
			// tcp
			if err := s.iptablesHandler.AddRuleToChain(
				chainName,
				RuleModel{
					Conntrack:   true,
					Ctstate:     []string{"NEW"},
					InputDev:    hostInterface,
					OutputDev:   n.Interface,
					Protocol:    "tcp",
					Syn:         true,
					NflogGroup:  11,
					NflogPrefix: "RAIND-NS-ALLOW,id=predefined",
				},
				"NFLOG",
			); err != nil {
				return err
			}
			// udp
			if err := s.iptablesHandler.AddRuleToChain(
				chainName,
				RuleModel{
					Conntrack:   true,
					Ctstate:     []string{"NEW"},
					InputDev:    hostInterface,
					OutputDev:   n.Interface,
					Protocol:    "udp",
					NflogGroup:  11,
					NflogPrefix: "RAIND-NS-ALLOW,id=predefined",
				},
				"NFLOG",
			); err != nil {
				return err
			}

			// from container to external
			if err := s.iptablesHandler.AddRuleToChain(
				chainName,
				RuleModel{
					Conntrack:   true,
					Ctstate:     []string{"NEW"},
					InputDev:    n.Interface,
					OutputDev:   hostInterface,
					NflogGroup:  11,
					NflogPrefix: "RAIND-NS-ALLOW,id=predefined",
				},
				"NFLOG",
			); err != nil {
				return err
			}
		}
		if err := s.iptablesHandler.AddRuleToChain(
			chainName,
			RuleModel{
				InputDev:  n.Interface,
				OutputDev: hostInterface,
			},
			"ACCEPT",
		); err != nil {
			return err
		}
	}

	// 2. other: return
	if err := s.iptablesHandler.AddRuleToChain(
		chainName,
		RuleModel{},
		"RETURN",
	); err != nil {
		return err
	}

	return nil
}

func (s *ServicePolicy) buildRaindNSEnforceChain(networkList []ipam.NetworkList, nflog bool) error {
	chainName := "RAIND-NS-ENF"
	hostInterface, err := s.ipamHandler.GetDefaultInterface()
	if err != nil {
		return err
	}

	// 1. if NFLOG mode is enabled, add NFLOG entry
	if nflog {
		for _, n := range networkList {
			if err := s.iptablesHandler.AddRuleToChain(
				chainName,
				RuleModel{
					Conntrack:   true,
					Ctstate:     []string{"NEW"},
					InputDev:    n.Interface,
					OutputDev:   hostInterface,
					NflogGroup:  12,
					NflogPrefix: "RAIND-NS-DENY,id=predefined",
				},
				"NFLOG",
			); err != nil {
				return err
			}
		}
	}

	// 2. allow container to external traffic
	for _, n := range networkList {
		if err := s.iptablesHandler.AddRuleToChain(
			chainName,
			RuleModel{
				InputDev:  n.Interface,
				OutputDev: hostInterface,
			},
			"DROP",
		); err != nil {
			return err
		}
	}

	// 3. other: return
	if err := s.iptablesHandler.AddRuleToChain(
		chainName,
		RuleModel{},
		"RETURN",
	); err != nil {
		return err
	}

	return nil
}

func (s *ServicePolicy) BuildUserPolicy() error {
	var err error = nil
	loggingEw := s.npmHandler.GetEWLogging()
	loggingNs := s.npmHandler.GetNSLogging()

	// 1. add RAIND-EW user policy
	ewPolicies := s.npmHandler.GetEWPolicyList()
	err = s.insertRaindEWUserPolicy(ewPolicies, loggingEw)

	// 2. add RAIND-NS-OBS user policy
	nsObsPolicies := s.npmHandler.GetNSObsPolicyList()
	err = s.insertRaindNSUserPolicy("RAIND-NS-OBS", nsObsPolicies, loggingNs)

	// 3. add RAIND-NS-ENF user policy
	nsEnfPolicies := s.npmHandler.GetNSEnfPolicyList()
	err = s.insertRaindNSUserPolicy("RAIND-NS-ENF", nsEnfPolicies, loggingNs)

	return err
}

func (s *ServicePolicy) insertRaindEWUserPolicy(policies []npm.Policy, logging bool) error {
	chainName := "RAIND-EW"
	for _, p := range policies {
		policyId := p.Id

		// check remove flag
		//   if status is "remove_next_commit", remove policy from store and not apply the policy
		if p.Status == "remove_next_commit" {
			// remove from store
			if err := s.npmHandler.RemovePolicy(p.Id); err != nil {
				return err
			}
			continue
		}

		// resolve container id/name
		srcContainerId, _ := s.resolveContainerNameAndInfo(p.Source.ContainerName)
		dstContainerId, _ := s.resolveContainerNameAndInfo(p.Destination.ContainerName)

		// resolve container veth
		srcVeth, err := s.ipamHandler.GetVethById(srcContainerId)
		if err != nil {
			// set status: unresolved, reason: contaner: <str> not found
			if err := s.npmHandler.UpdateStatus(
				chainName,
				p.Id,
				"unresolved",
				fmt.Sprintf("container: %s not found", p.Source.ContainerName),
			); err != nil {
				return err
			}
			continue
		}
		dstVeth, err := s.ipamHandler.GetVethById(dstContainerId)
		if err != nil {
			// set status: unresolved, reason: contaner: <str> not found
			if err := s.npmHandler.UpdateStatus(
				chainName,
				p.Id,
				"unresolved",
				fmt.Sprintf("container: %s not found", p.Destination.ContainerName),
			); err != nil {
				return err
			}
			continue
		}

		if err := s.iptablesHandler.InsertRuleToChain(
			chainName,
			RuleModel{
				Physdev:         true,
				PhysdevIsBridge: true,
				InputPhysdev:    srcVeth,
				OutputPhysdev:   dstVeth,
				Protocol:        p.Protocol,
				DestPort:        p.DestPort,
			},
			"ACCEPT",
		); err != nil {
			continue
		}
		if logging {
			if err := s.iptablesHandler.InsertRuleToChain(
				chainName,
				RuleModel{
					Conntrack:       true,
					Ctstate:         []string{"NEW"},
					Physdev:         true,
					PhysdevIsBridge: true,
					InputPhysdev:    srcVeth,
					OutputPhysdev:   dstVeth,
					Protocol:        p.Protocol,
					DestPort:        p.DestPort,
					NflogGroup:      10,
					NflogPrefix:     "RAIND-EW-ALLOW,id=" + policyId,
				},
				"NFLOG",
			); err != nil {
				continue
			}
		}

		// set status: applied
		if err := s.npmHandler.UpdateStatus(
			chainName,
			p.Id,
			"applied",
			"",
		); err != nil {
			continue
		}
	}

	return nil
}

func (s *ServicePolicy) insertRaindNSUserPolicy(chainName string, policies []npm.Policy, logging bool) error {
	for _, p := range policies {
		policyId := p.Id

		// check remove flag
		//   if status is "remove_next_commit", remove policy from store and not apply the policy
		if p.Status == "remove_next_commit" {
			// remove from store
			if err := s.npmHandler.RemovePolicy(p.Id); err != nil {
				return err
			}
			continue
		}

		// resolve container id/name
		srcContainerId, _ := s.resolveContainerNameAndInfo(p.Source.ContainerName)

		_, err := s.ipamHandler.GetVethById(srcContainerId)
		if err != nil {
			// set status: unresolved, reason: contaner: <str> not found
			if err := s.npmHandler.UpdateStatus(
				chainName,
				p.Id,
				"unresolved",
				fmt.Sprintf("container: %s not found", p.Source.ContainerName),
			); err != nil {
				return err
			}
			continue
		}
		_, bridgeDev, srcAddress, _ := s.ipamHandler.GetContainerAddress(srcContainerId)
		dstDev, _ := s.ipamHandler.GetDefaultInterface()

		var action string
		if chainName == "RAIND-NS-OBS" {
			action = "DROP"
		} else {
			action = "ACCEPT"
		}

		if err := s.iptablesHandler.InsertRuleToChain(
			chainName,
			RuleModel{
				InputDev:    bridgeDev,
				Source:      srcAddress,
				OutputDev:   dstDev,
				Destination: p.Destination.Address,
				Protocol:    p.Protocol,
				DestPort:    p.DestPort,
			},
			action,
		); err != nil {
			continue
		}

		if logging {
			var (
				nflogGroup  int
				nflogPrefix string
			)
			if chainName == "RAIND-NS-OBS" {
				nflogGroup = 11
				nflogPrefix = "RAIND-NS-DENY,id=" + policyId
			} else {
				nflogGroup = 12
				nflogPrefix = "RAIND-NS-ALLOW,id=" + policyId
			}
			if err := s.iptablesHandler.InsertRuleToChain(
				chainName,
				RuleModel{
					Conntrack:   true,
					Ctstate:     []string{"NEW"},
					InputDev:    bridgeDev,
					Source:      srcAddress,
					OutputDev:   dstDev,
					Destination: p.Destination.Address,
					Protocol:    p.Protocol,
					DestPort:    p.DestPort,
					NflogGroup:  nflogGroup,
					NflogPrefix: nflogPrefix,
				},
				"NFLOG",
			); err != nil {
				continue
			}
		}
		// set status: applied
		if err := s.npmHandler.UpdateStatus(
			chainName,
			p.Id,
			"applied",
			"",
		); err != nil {
			continue
		}
	}

	return nil
}
