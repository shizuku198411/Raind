package utils

import (
	"crypto/rand"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

var entropy = ulid.Monotonic(rand.Reader, 0)

func NewUlid() string {
	return strings.ToLower(ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String())
}
