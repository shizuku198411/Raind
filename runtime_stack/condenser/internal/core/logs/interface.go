package logs

type LogServiceHandler interface {
	GetNetflowLogWithTailLines(n int) ([]byte, error)
}
