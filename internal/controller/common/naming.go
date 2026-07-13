package common

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// infraNameHashLen is the number of hex digest characters appended when a
// default infra name must be shortened; enough that sibling CRs sharing a
// long prefix in one namespace don't collide.
const infraNameHashLen = 5

// FitDefaultInfraName returns "<crName><suffix>" when that is a valid
// DNS-1123 label within budget. Otherwise it returns "<prefix>-<hash><suffix>",
// where prefix is what fits of crName and hash is a short stable digest of the
// full CR name, so any CR name (however long, or containing dots) yields a
// usable, collision-free default. The budget must leave room for the suffix
// plus the hashed form; derived names are persisted in the spec by the
// defaulting webhook, so the result must be deterministic.
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

// sanitizeLabelPrefix truncates s to maxLen and makes it usable as the leading
// part of a DNS-1123 label: CR names may contain dots (they are DNS
// subdomains), and a truncation point may leave stray hyphens.
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
