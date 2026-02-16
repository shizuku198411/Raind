package policy

import (
	"condenser/internal/utils"
	"slices"
	"strconv"
	"strings"
)

func NewIptablesManager() *IptablesManager {
	return &IptablesManager{
		commandFactory: utils.NewCommandFactory(),
	}
}

type IptablesManager struct {
	commandFactory utils.CommandFactory
}

func (h *IptablesManager) CreateChain(chainName string) error {
	// check if chain already exist
	check := h.commandFactory.Command("iptables", "-L", chainName)
	if err := check.Run(); err == nil {
		// clear chain
		clear := h.commandFactory.Command("iptables", "-F", chainName)
		if err := clear.Run(); err != nil {
			return err
		}
		return nil
	}

	// create chain
	create := h.commandFactory.Command("iptables", "-N", chainName)
	if err := create.Run(); err != nil {
		return err
	}

	return nil
}

func (h *IptablesManager) InsertForwardRule(chainName string) error {
	// check if forward rule already exist
	check := h.commandFactory.Command("iptables", "-C", "FORWARD", "-j", chainName)
	if err := check.Run(); err == nil {
		// rule already exist
		return nil
	}

	// insert rule
	insertRule := h.commandFactory.Command("iptables", "-I", "FORWARD", "1", "-j", chainName)
	if err := insertRule.Run(); err != nil {
		return err
	}

	return nil
}

func (h *IptablesManager) AddRuleToChain(chainName string, ruleModel RuleModel, action string) error {
	ruleParam := []string{chainName, "-j", action}
	if ruleModel.Conntrack {
		ruleParam = slices.Concat(ruleParam, []string{"-m", "conntrack"})
	}
	if len(ruleModel.Ctstate) > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--ctstate", strings.Join(ruleModel.Ctstate, ",")})
	}
	if ruleModel.Physdev {
		ruleParam = slices.Concat(ruleParam, []string{"-m", "physdev"})
	}
	if ruleModel.PhysdevIsBridge {
		ruleParam = slices.Concat(ruleParam, []string{"--physdev-is-bridged"})
	}
	if ruleModel.InputDev != "" {
		ruleParam = slices.Concat(ruleParam, []string{"-i", ruleModel.InputDev})
	}
	if ruleModel.OutputDev != "" {
		ruleParam = slices.Concat(ruleParam, []string{"-o", ruleModel.OutputDev})
	}
	if ruleModel.InputPhysdev != "" {
		ruleParam = slices.Concat(ruleParam, []string{"--physdev-in", ruleModel.InputPhysdev})
	}
	if ruleModel.OutputPhysdev != "" {
		ruleParam = slices.Concat(ruleParam, []string{"--physdev-out", ruleModel.OutputPhysdev})
	}
	if ruleModel.Protocol != "" {
		ruleParam = slices.Concat(ruleParam, []string{"-p", ruleModel.Protocol})
		if ruleModel.Protocol == "tcp" && ruleModel.Syn {
			ruleParam = slices.Concat(ruleParam, []string{"--syn"})
		}
	}
	if ruleModel.SourcePort > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--sport", strconv.Itoa(ruleModel.SourcePort)})
	}
	if ruleModel.DestPort > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--dport", strconv.Itoa(ruleModel.DestPort)})
	}
	if ruleModel.NflogGroup > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--nflog-group", strconv.Itoa(ruleModel.NflogGroup)})
	}
	if ruleModel.NflogPrefix != "" {
		ruleParam = slices.Concat(ruleParam, []string{"--nflog-prefix", ruleModel.NflogPrefix})
	}

	// check if rule already exist
	checkCmd := slices.Concat([]string{"iptables", "-C"}, ruleParam)
	check := h.commandFactory.Command(checkCmd[0], checkCmd[1:]...)
	if err := check.Run(); err == nil {
		// rule already exist
		return nil
	}

	// add rule
	addRuleCmd := slices.Concat([]string{"iptables", "-A"}, ruleParam)
	addRule := h.commandFactory.Command(addRuleCmd[0], addRuleCmd[1:]...)
	if err := addRule.Run(); err != nil {
		return err
	}
	return nil
}

func (h *IptablesManager) InsertRuleToChain(chainName string, ruleModel RuleModel, action string) error {
	ruleParam := []string{chainName, "-j", action}
	if ruleModel.Conntrack {
		ruleParam = slices.Concat(ruleParam, []string{"-m", "conntrack"})
	}
	if len(ruleModel.Ctstate) > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--ctstate", strings.Join(ruleModel.Ctstate, ",")})
	}
	if ruleModel.Physdev {
		ruleParam = slices.Concat(ruleParam, []string{"-m", "physdev"})
	}
	if ruleModel.PhysdevIsBridge {
		ruleParam = slices.Concat(ruleParam, []string{"--physdev-is-bridged"})
	}
	if ruleModel.InputDev != "" {
		ruleParam = slices.Concat(ruleParam, []string{"-i", ruleModel.InputDev})
	}
	if ruleModel.OutputDev != "" {
		ruleParam = slices.Concat(ruleParam, []string{"-o", ruleModel.OutputDev})
	}
	if ruleModel.InputPhysdev != "" {
		ruleParam = slices.Concat(ruleParam, []string{"--physdev-in", ruleModel.InputPhysdev})
	}
	if ruleModel.OutputPhysdev != "" {
		ruleParam = slices.Concat(ruleParam, []string{"--physdev-out", ruleModel.OutputPhysdev})
	}
	if ruleModel.Protocol != "" {
		ruleParam = slices.Concat(ruleParam, []string{"-p", ruleModel.Protocol})
	}
	if ruleModel.Source != "" {
		ruleParam = slices.Concat(ruleParam, []string{"-s", ruleModel.Source})
	}
	if ruleModel.SourcePort > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--sport", strconv.Itoa(ruleModel.SourcePort)})
	}
	if ruleModel.Destination != "" {
		ruleParam = slices.Concat(ruleParam, []string{"-d", ruleModel.Destination})
	}
	if ruleModel.DestPort > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--dport", strconv.Itoa(ruleModel.DestPort)})
	}
	if ruleModel.NflogGroup > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--nflog-group", strconv.Itoa(ruleModel.NflogGroup)})
	}
	if ruleModel.NflogPrefix != "" {
		ruleParam = slices.Concat(ruleParam, []string{"--nflog-prefix", ruleModel.NflogPrefix})
	}

	// check if rule already exist
	checkCmd := slices.Concat([]string{"iptables", "-C"}, ruleParam)
	check := h.commandFactory.Command(checkCmd[0], checkCmd[1:]...)
	if err := check.Run(); err == nil {
		// rule already exist
		return nil
	}

	// add rule
	addRuleCmd := slices.Concat([]string{"iptables", "-I"}, ruleParam)
	addRule := h.commandFactory.Command(addRuleCmd[0], addRuleCmd[1:]...)
	if err := addRule.Run(); err != nil {
		return err
	}
	return nil
}
