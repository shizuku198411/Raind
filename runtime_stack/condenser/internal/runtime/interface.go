package runtime

type RuntimeHandler interface {
	Spec(specParameter SpecModel) error
	Create(createParameter CreateModel, podPid int) error
	Start(startParameter StartModel) error
	Delete(deleteParameter DeleteModel) error
	Stop(stopParameter StopModel) error
	Exec(execParameter ExecModel) error
}
