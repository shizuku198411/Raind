package logs

import (
	"condenser/internal/utils"
	"fmt"
)

const (
	maxTailLines = 10000
	maxTailBytes = 4 * 1024 * 1024
)

func NewLogService() *LogService {
	return &LogService{}
}

type LogService struct{}

func (s *LogService) GetNetflowLogWithTailLines(n int) ([]byte, error) {
	if n > maxTailLines {
		return nil, fmt.Errorf("invalid tail lines: max=%d", maxTailLines)
	}

	data, err := utils.TailLines(utils.EnrichedLogPath, n, maxTailBytes)
	if err != nil {
		return nil, fmt.Errorf("tail failed: %v", err)
	}
	return data, nil
}
