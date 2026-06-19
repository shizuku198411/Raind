package sec

type SecHandler interface {
	StoreSecret(secretId string, spec SecretInfo) error
	GetSecretList() ([]SecretInfo, error)
	GetSecretById(secretId string) (SecretInfo, error)
	GetSecretByName(name, namespace string) (SecretInfo, error)
	RemoveSecret(secretId string) error
	IsNameAlreadyUsed(name, namespace string) bool
}
