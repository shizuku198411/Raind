package vsm

type StoreHandler interface {
	SetVolumeState() error
}

type Handler interface {
	StorePVC(pvcId string, info PersistentVolumeClaimInfo) error
	GetPVCList() ([]PersistentVolumeClaimInfo, error)
	GetPVCById(pvcId string) (PersistentVolumeClaimInfo, error)
	GetPVCByName(name, namespace string) (PersistentVolumeClaimInfo, error)
	RemovePVC(pvcId string) error
	IsNameAlreadyUsed(name, namespace string) bool
}
