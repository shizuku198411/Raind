package hook

type HookServiceHandler interface {
	HookAction(stateParameter ServiceStateModel, eventType string) error
}
