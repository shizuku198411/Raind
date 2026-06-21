package promote

import "strings"

var secretKeyMarkers = []string{
	"PASSWORD",
	"PASS",
	"SECRET",
	"TOKEN",
	"API_KEY",
	"ACCESS_KEY",
	"PRIVATE_KEY",
	"DATABASE_URL",
	"DB_PASSWORD",
	"MYSQL_PASSWORD",
	"POSTGRES_PASSWORD",
	"REDIS_PASSWORD",
}

func IsSecretLikeKey(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	if upper == "" {
		return false
	}
	for _, marker := range secretKeyMarkers {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}
