package cdk8s

import (
	"github.com/Masterminds/semver"
)

// IsVersionSupport checks if the version is supported by the operator
func IsVersionSupport(version string) (bool, error) {
	c, _ := semver.NewConstraint(SupportedVersions)
	v, err := semver.NewVersion(version)
	if err != nil {
		return false, err
	}
	a, msgs := c.Validate(v)
	for _, m := range msgs {
		return a, m
	}
	return a, nil
}

func ApplyVersion() {}
