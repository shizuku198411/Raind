package pathenv

import "strings"

const Default = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

func OrDefault(env []string) string {
	for _, item := range env {
		if strings.HasPrefix(item, "PATH=") {
			value := strings.TrimPrefix(item, "PATH=")
			if value != "" {
				return value
			}
			return Default
		}
	}
	return Default
}
