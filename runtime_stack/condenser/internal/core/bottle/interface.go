package bottle

type BottleServiceHandler interface {
	DecodeSpec(yamlBytes []byte) (*BottleSpec, error)
	BuildStartOrder(spec *BottleSpec) ([]string, error)
	Create(bottleIdOrName string) (string, error)
	Start(bottleIdOrName string) (string, error)
	Stop(bottleIdOrName string) (string, error)
	Delete(bottleIdOrName string) (string, error)
}
