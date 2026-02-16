package bottle

import (
	apimodel "condenser/internal/api/http/utils"
	"condenser/internal/core/bottle"
	"condenser/internal/core/container"
	"condenser/internal/core/policy"
	"condenser/internal/store/bsm"
	"condenser/internal/utils"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		serviceHandler:   bottle.NewBottleService(),
		bsmHandler:       bsm.NewBsmManager(bsm.NewBsmStore(utils.BsmStorePath)),
		policyHandler:    policy.NewwServicePolicy(),
		containerHandler: container.NewContaierService(),
	}
}

type RequestHandler struct {
	serviceHandler   bottle.BottleServiceHandler
	bsmHandler       bsm.BsmHandler
	policyHandler    policy.PolicyServiceHandler
	containerHandler container.ContainerServiceHandler
}

// RegisterBottle godoc
// @Summary register a bottle
// @Description register a bottle from yaml spec
// @Tags bottles
// @Accept text/plain
// @Produce json
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/bottle [post]
func (h *RequestHandler) RegisterBottle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid body: "+err.Error(), nil)
		return
	}

	spec, err := h.serviceHandler.DecodeSpec(body)
	if err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid yaml: "+err.Error(), nil)
		return
	}

	order, err := h.serviceHandler.BuildStartOrder(spec)
	if err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid dependency: "+err.Error(), nil)
		return
	}

	if h.bsmHandler.IsNameAlreadyUsed(spec.Bottle.Name) {
		apimodel.RespondFail(w, http.StatusBadRequest, "name already used by other bottle", nil)
		return
	}

	bottleId := utils.NewUlid()[:12]
	policies, err := h.applyPolicies(spec.Bottle.Name, spec.Services, spec.Policies)
	if err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "policy failed: "+err.Error(), nil)
		return
	}

	if err := h.bsmHandler.StoreBottle(bottleId, spec.Bottle.Name, toStoreServices(spec.Services), order, policies); err != nil {
		h.rollbackPolicies(extractPolicyIds(policies))
		apimodel.RespondFail(w, http.StatusInternalServerError, "store failed: "+err.Error(), nil)
		return
	}

	if _, err := h.serviceHandler.Create(bottleId); err != nil {
		_ = h.bsmHandler.RemoveBottle(bottleId)
		h.rollbackPolicies(extractPolicyIds(policies))
		apimodel.RespondFail(w, http.StatusInternalServerError, "create containers failed: "+err.Error(), nil)
		return
	}

	services := make([]string, 0, len(spec.Services))
	for name := range spec.Services {
		services = append(services, name)
	}
	slices.Sort(services)

	apimodel.RespondSuccess(w, http.StatusCreated, "bottle registered", RegisterBottleResponse{
		BottleId:   bottleId,
		BottleName: spec.Bottle.Name,
		Services:   services,
		StartOrder: order,
	})
}

// GetBottleList godoc
// @Summary list bottles
// @Description list bottles
// @Tags bottles
// @Produce json
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/bottle [get]
func (h *RequestHandler) GetBottleList(w http.ResponseWriter, r *http.Request) {
	list, err := h.bsmHandler.GetBottleList()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "list failed: "+err.Error(), nil)
		return
	}
	res := make([]BottleSummary, 0, len(list))
	for _, b := range list {
		status := h.buildBottleStatus(b)
		res = append(res, BottleSummary{
			BottleId:     b.BottleId,
			BottleName:   b.BottleName,
			ServiceCount: len(b.Services),
			Status:       status,
		})
	}
	apimodel.RespondSuccess(w, http.StatusOK, "bottle list", GetBottleListResponse{Bottles: res})
}

// GetBottleDetail godoc
// @Summary get bottle detail
// @Description get bottle detail by id or name
// @Tags bottles
// @Param bottleId path string true "Bottle ID or Name"
// @Produce json
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/bottle/{bottleId} [get]
func (h *RequestHandler) GetBottleDetail(w http.ResponseWriter, r *http.Request) {
	bottleId := chi.URLParam(r, "bottleId")
	if bottleId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing bottleId", nil)
		return
	}

	resolvedId, err := h.bsmHandler.ResolveBottleId(bottleId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusNotFound, "bottle not found", nil)
		return
	}

	info, err := h.bsmHandler.GetBottleById(resolvedId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusNotFound, "bottle not found", nil)
		return
	}

	containerStates, err := h.buildBottleContainers(info.BottleName, info.Containers)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "container status failed: "+err.Error(), nil)
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "bottle detail", GetBottleResponse{
		Bottle: BottleDetail{
			BottleId:   info.BottleId,
			BottleName: info.BottleName,
			Services:   toApiServices(info.Services, info.Network, info.NetworkAuto),
			StartOrder: info.StartOrder,
			Containers: containerStates,
			Policies:   toApiPolicies(info.BottleName, info.Policies),
			Network:    info.Network,
			NetworkAuto: info.NetworkAuto,
			CreatedAt:  info.CreatedAt.Format(time.RFC3339Nano),
		},
	})
}

// ActionBottle godoc
// @Summary perform bottle action
// @Description start/stop/remove bottle
// @Tags bottles
// @Param bottleId path string true "Bottle ID or Name"
// @Param action path string true "Action"
// @Produce json
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/bottle/{bottleId}/actions/{action} [post]
func (h *RequestHandler) ActionBottle(w http.ResponseWriter, r *http.Request) {
	bottleId := chi.URLParam(r, "bottleId")
	action := chi.URLParam(r, "action")
	if bottleId == "" || action == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing bottleId or action", nil)
		return
	}

	var err error
	switch action {
	case "start":
		_, err = h.serviceHandler.Start(bottleId)
	case "stop":
		_, err = h.serviceHandler.Stop(bottleId)
	case "delete":
		_, err = h.serviceHandler.Delete(bottleId)
	default:
		apimodel.RespondFail(w, http.StatusBadRequest, "unknown action", nil)
		return
	}
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), nil)
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "bottle action done", ActionBottleResponse{Id: bottleId})
}

func toStoreServices(services map[string]bottle.ServiceSpec) map[string]bsm.ServiceSpec {
	out := make(map[string]bsm.ServiceSpec, len(services))
	for name, svc := range services {
		out[name] = bsm.ServiceSpec{
			Image:     svc.Image,
			Command:   svc.Command,
			Env:       svc.Env,
			Ports:     svc.Ports,
			Mount:     svc.Mount,
			Network:   svc.Network,
			Tty:       svc.Tty,
			DependsOn: svc.DependsOn,
		}
	}
	return out
}

func toApiServices(services map[string]bsm.ServiceSpec, bottleNetwork string, auto bool) map[string]BottleServiceSpec {
	out := make(map[string]BottleServiceSpec, len(services))
	for name, svc := range services {
		networkName := svc.Network
		if networkName == "" && auto && bottleNetwork != "" {
			networkName = bottleNetwork
		}
		out[name] = BottleServiceSpec{
			Image:     svc.Image,
			Command:   svc.Command,
			Env:       svc.Env,
			Ports:     svc.Ports,
			Mount:     svc.Mount,
			Network:   networkName,
			Tty:       svc.Tty,
			DependsOn: svc.DependsOn,
		}
	}
	return out
}

func toApiPolicies(bottleName string, policies []bsm.PolicyInfo) []BottlePolicyInfo {
	if len(policies) == 0 {
		return nil
	}
	out := make([]BottlePolicyInfo, 0, len(policies))
	for _, p := range policies {
		out = append(out, BottlePolicyInfo{
			Id:          p.Id,
			Type:        p.Type,
			Source:      normalizePolicyEndpoint(bottleName, p.Source),
			Destination: normalizePolicyEndpoint(bottleName, p.Destination),
			Protocol:    p.Protocol,
			DestPort:    p.DestPort,
			Comment:     p.Comment,
		})
	}
	return out
}

func (h *RequestHandler) applyPolicies(bottleName string, services map[string]bottle.ServiceSpec, policies []bottle.PolicySpec) ([]bsm.PolicyInfo, error) {
	if len(policies) == 0 {
		return nil, nil
	}
	var applied []string
	var stored []bsm.PolicyInfo
	for _, p := range policies {
		chainName, err := mapPolicyChain(p.Type)
		if err != nil {
			h.rollbackPolicies(applied)
			return nil, err
		}
		src := resolvePolicyEndpoint(bottleName, services, p.Source)
		dst := resolvePolicyEndpoint(bottleName, services, p.Destination)
		if err := validatePolicyDestination(chainName, dst); err != nil {
			h.rollbackPolicies(applied)
			return nil, err
		}
		id, err := h.policyHandler.AddUserPolicy(policy.ServiceAddPolicyModel{
			ChainName:   chainName,
			Source:      src,
			Destination: dst,
			Protocol:    p.Protocol,
			DestPort:    p.DestPort,
			Comment:     p.Comment,
		})
		if err != nil {
			h.rollbackPolicies(applied)
			return nil, err
		}
		applied = append(applied, id)
		stored = append(stored, bsm.PolicyInfo{
			Id:          id,
			Type:        p.Type,
			Source:      src,
			Destination: dst,
			Protocol:    p.Protocol,
			DestPort:    p.DestPort,
			Comment:     p.Comment,
		})
	}
	// commit
	if err := h.policyHandler.CommitPolicy(); err != nil {
		return nil, err
	}
	return stored, nil
}

func (h *RequestHandler) rollbackPolicies(ids []string) {
	for _, id := range ids {
		_ = h.policyHandler.RemoveUserPolicy(policy.ServiceRemovePolicyModel{Id: id})
	}
	_ = h.policyHandler.CommitPolicy()
}

func mapPolicyChain(policyType string) (string, error) {
	switch policyType {
	case "east-west":
		return "RAIND-EW", nil
	case "north-south_observe":
		return "RAIND-NS-OBS", nil
	case "north-south_enforce":
		return "RAIND-NS-ENF", nil
	default:
		return "", fmt.Errorf("unknown policy type: %s", policyType)
	}
}

func validatePolicyDestination(chainName string, dest string) error {
	isIP := isIPAddress(dest)
	switch chainName {
	case "RAIND-EW":
		if isIP {
			return fmt.Errorf("east-west destination must be container name or id")
		}
	case "RAIND-NS-OBS", "RAIND-NS-ENF":
		if !isIP {
			return fmt.Errorf("north-south destination must be ip address")
		}
	}
	return nil
}

func isIPAddress(s string) bool {
	if ip := net.ParseIP(s); ip != nil {
		return true
	}
	if strings.Contains(s, "/") {
		if _, _, err := net.ParseCIDR(s); err == nil {
			return true
		}
	}
	return false
}

func resolvePolicyEndpoint(bottleName string, services map[string]bottle.ServiceSpec, value string) string {
	if services != nil {
		if _, ok := services[value]; ok {
			return bottleName + "-" + value
		}
	}
	return value
}

func extractPolicyIds(policies []bsm.PolicyInfo) []string {
	if len(policies) == 0 {
		return nil
	}
	ids := make([]string, 0, len(policies))
	for _, p := range policies {
		if p.Id != "" {
			ids = append(ids, p.Id)
		}
	}
	return ids
}

func (h *RequestHandler) buildBottleContainers(bottleName string, containers map[string]string) (map[string]BottleContainerState, error) {
	if len(containers) == 0 {
		return map[string]BottleContainerState{}, nil
	}
	out := make(map[string]BottleContainerState, len(containers))
	for serviceName, containerId := range containers {
		state, err := h.containerHandler.GetContainerById(containerId)
		if err != nil {
			return nil, err
		}
		out[serviceName] = BottleContainerState{
			ContainerId: state.ContainerId,
			Name:        buildBottleContainerName(bottleName, serviceName),
			State:       state.State,
			Pid:         state.Pid,
			Repository:  state.Repository,
			Reference:   state.Reference,
			Command:     state.Command,
			Address:     state.Address,
			Forwards:    toBottleForwards(state.Forwards),
			CreatingAt:  state.CreatingAt.UTC().Format(time.RFC3339Nano),
			CreatedAt:   state.CreatedAt.UTC().Format(time.RFC3339Nano),
			StartedAt:   state.StartedAt.UTC().Format(time.RFC3339Nano),
			StoppedAt:   state.StoppedAt.UTC().Format(time.RFC3339Nano),
		}
	}
	return out, nil
}

func toBottleForwards(forwards []container.ForwardInfo) []BottleForwardInfo {
	if len(forwards) == 0 {
		return nil
	}
	out := make([]BottleForwardInfo, 0, len(forwards))
	for _, f := range forwards {
		out = append(out, BottleForwardInfo{
			HostPort:      f.HostPort,
			ContainerPort: f.ContainerPort,
			Protocol:      f.Protocol,
		})
	}
	return out
}

func buildBottleContainerName(bottleName string, serviceName string) string {
	return bottleName + "-" + serviceName
}

func (h *RequestHandler) buildBottleStatus(info bsm.BottleInfo) string {
	if len(info.Containers) == 0 {
		return "unknown"
	}
	status := ""
	for _, containerId := range info.Containers {
		state, err := h.containerHandler.GetContainerById(containerId)
		if err != nil {
			return "abnormal"
		}
		if status == "" {
			status = state.State
			continue
		}
		if status != state.State {
			return "abnormal"
		}
	}
	if status == "" {
		return "unknown"
	}
	return status
}

func normalizePolicyEndpoint(bottleName string, value string) string {
	prefix := bottleName + "-"
	if strings.HasPrefix(value, prefix) {
		return strings.TrimPrefix(value, prefix)
	}
	return value
}
