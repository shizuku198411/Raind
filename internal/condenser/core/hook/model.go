package hook

type ServiceStateAnnotaionModel struct {
	AnnotationVersion string `json:"io.raind.runtime.annotation.version"`
	NetConfig         string `json:"io.raind.net.config"`
	ImageConfig       string `json:"io.raind.image.config"`
}

type ServiceStateModel struct {
	OciVersion  string                     `json:"ociVersion"`
	Id          string                     `json:"id"`
	Status      string                     `json:"status"`
	Pid         int                        `json:"pid"`
	ExitCode    int                        `json:"exit_code"`
	Reason      string                     `json:"reason"`
	Message     string                     `json:"message"`
	Rootfs      string                     `json:"rootfs"`
	Bundler     string                     `json:"bundle"`
	Annotations ServiceStateAnnotaionModel `json:"annotations,omitempty"`
}
