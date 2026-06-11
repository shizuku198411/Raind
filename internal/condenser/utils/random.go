package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

var adjectives = []string{
	"hot", "cold", "clean", "soft", "hard", "high",
	"low", "fast", "slow", "calm", "angular", "circular",
	"convex", "curved", "flat", "narrow", "round",
}

var nouns = []string{
	"theorem", "proof", "formula", "axis", "angle",
	"radius", "tangent", "catenary", "bezier", "spline",
	"hilbert", "clothoid", "sine", "cosine", "tangent",
}

func randIndex(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

func GenerateRandName() (string, error) {
	ai, err := randIndex(len(adjectives))
	if err != nil {
		return "", err
	}
	ni, err := randIndex(len(nouns))
	if err != nil {
		return "", err
	}
	suffix, err := randIndex(10000)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"%s-%s-%04d",
		adjectives[ai],
		nouns[ni],
		suffix,
	), nil
}
