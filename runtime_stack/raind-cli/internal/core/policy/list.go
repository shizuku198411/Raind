package policy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	httpclient "raind/internal/core/client"
	"strconv"
	"strings"
	"text/tabwriter"
)

func NewServicePolicyList() *ServicePolicyList {
	return &ServicePolicyList{}
}

type ServicePolicyList struct{}

func (s *ServicePolicyList) List(param ListRequestModel) error {
	fmt.Println("FLAG: [*] - Applied, [+] - Apply next commit, [-] - Remove next commit, [ ] - Not applied\n")

	var chainName string
	switch param.ChainName {
	case "ew":
		chainName = "RAIND-EW"
		if err := s.requestGetList(chainName, true); err != nil {
			return err
		}

	case "ns-obs":
		chainName = "RAIND-NS-OBS"
		if err := s.requestGetList(chainName, true); err != nil {
			return err
		}

	case "ns-enf":
		chainName = "RAIND-NS-ENF"
		if err := s.requestGetList(chainName, true); err != nil {
			return err
		}

	default:
		if err := s.requestGetList("RAIND-EW", false); err != nil {
			return err
		}
		fmt.Println("\n============================")
		if err := s.requestGetList("RAIND-NS-OBS", false); err != nil {
			return err
		}
		if err := s.requestGetList("RAIND-NS-ENF", false); err != nil {
			return err
		}
	}

	return nil
}

func (s *ServicePolicyList) requestGetList(chainName string, chainFlag bool) error {
	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}
	httpClient.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/policies/%s", chainName),
		nil,
	)
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel ListResponseModel

	if !httpClient.IsStatusOk(resp) {
		decodeErr := json.NewDecoder(resp.Body).Decode(&respModel)
		if decodeErr != nil {
			return fmt.Errorf("decode response: %w", decodeErr)
		}
		return fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}

	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	s.printPolicyList(chainName, respModel.Data.Mode, respModel.Data.Policies, chainFlag)

	return nil
}

func (s *ServicePolicyList) printPolicyList(chainName string, mode string, PolicyList []PolicyModel, chainFlag bool) {
	if !chainFlag {
		if strings.Contains(mode, "observe") && chainName == "RAIND-NS-ENF" {
			return
		}
		if strings.Contains(mode, "enforce") && chainName == "RAIND-NS-OBS" {
			return
		}
	}

	w := tabwriter.NewWriter(
		os.Stdout,
		0,
		0,
		2,
		' ',
		tabwriter.DiscardEmptyColumns,
	)

	flagMap := map[string]string{
		"before_commit":      "[+]",
		"remove_next_commit": "[-]",
		"applied":            "[*]",
		"unresolved":         "[ ]",
	}

	// type
	var policyType string
	switch chainName {
	case "RAIND-EW":
		policyType = "East-West"
	case "RAIND-NS-OBS", "RAIND-NS-ENF":
		policyType = "North-South"
	default:
		policyType = "unknown"
	}
	fmt.Printf("POLICY TYPE : %s\n", policyType)
	// mode
	if strings.Contains(mode, "_next_commit") {
		mode = strings.Split(mode, "_")[0] + " (Next commit)"
	}
	fmt.Printf("CURRENT MODE: %s\n", mode)

	if chainName == "RAIND-EW" {
		fmt.Fprintln(w, "\nFLAG\tPOLICY ID\tSRC CONTAINER\tDST CONTAINER\tPROTOCOL\tDST PORT\tACTION\tCOMMENT\tREASON")
	} else {
		fmt.Fprintln(w, "\nFLAG\tPOLICY ID\tSRC CONTAINER\tDST ADDR\tPROTOCOL\tDST PORT\tACTION\tCOMMENT\tREASON")
	}

	// helper
	parseDport := func(port int) string {
		if port == 0 {
			return "*"
		}
		return strconv.Itoa(port)
	}

	// data
	for _, p := range PolicyList {
		flag := flagMap[p.Status]
		id := p.Id
		src := p.Source.ContainerName
		var dst string
		if chainName == "RAIND-EW" {
			dst = p.Destination.ContainerName
		} else {
			dst = p.Destination.Address
		}
		protocol := p.Protocol
		dport := parseDport(p.DestPort)
		var action string
		if chainName == "RAIND-EW" || chainName == "RAIND-NS-ENF" {
			action = "ALLOW"
		} else {
			action = "DENY"
		}
		comment := p.Comment
		reason := p.Reason

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", flag, id, src, dst, protocol, dport, action, comment, reason)
	}

	switch chainName {
	case "RAIND-EW":
		fmt.Fprintln(w, "  >> DENY ALL EAST-WEST TRAFFIC <<")
	case "RAIND-NS-OBS":
		fmt.Fprintln(w, "  >> ALLOW ALL NORTH-SOUTH TRAFFIC <<")
	case "RAIND-NS-ENF":
		fmt.Fprintln(w, "  >> DENY ALL NORTH-SOUTH TRAFFIC <<")
	}

	w.Flush()
}
