package common

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// infraNameHashLen is enough to keep sibling CRs sharing a long prefix from colliding.
const infraNameHashLen = 5

// FitDefaultInfraName returns "<crName><suffix>", falling back to a
// deterministic "<prefix>-<hash><suffix>" when that would not be a valid
// DNS-1123 label within budget — any CR name yields a usable default.
func FitDefaultInfraName(crName, suffix string, budget int) string {
	plain := crName + suffix
	if len(plain) <= budget && len(validation.IsDNS1123Label(plain)) == 0 {
		return plain
	}

	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(crName)))[:infraNameHashLen]
	prefix := sanitizeLabelPrefix(crName, budget-len(suffix)-infraNameHashLen-1)
	if prefix == "" {
		return digest + suffix
	}
	return fmt.Sprintf("%s-%s%s", prefix, digest, suffix)
}

// sanitizeLabelPrefix truncates s to maxLen and strips what can't lead a
// DNS-1123 label (CR names may contain dots; truncation may leave hyphens).
func sanitizeLabelPrefix(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	s = strings.ReplaceAll(s, ".", "-")
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return strings.Trim(s, "-")
}
