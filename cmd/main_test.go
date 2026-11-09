package main

import (
	"flag"
	"strconv"
	"testing"
)

func TestSpecFlagsFromEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name       string
		env        string
		defaultVal bool
	}{
		{"managed-spec-enabled", "MANAGED_SPEC_ENABLED", false},
		{"user-spec-enabled", "USER_SPEC_ENABLED", true},
		{"deployer-channel-enabled", "DEPLOYER_CHANNEL_ENABLED", true},
	} {
		for _, value := range []string{"true", "false"} {
			t.Run(tc.name+"/"+value, func(t *testing.T) {
				original := flag.CommandLine
				flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
				t.Cleanup(func() { flag.CommandLine = original })
				enabled := flag.Bool(tc.name, tc.defaultVal, "")
				t.Setenv(tc.env, value)
				if errs := setFlagsFromEnvironment(); len(errs) != 0 {
					t.Fatalf("setting flags: %v", errs)
				}
				if *enabled != (value == "true") {
					t.Fatalf("%s = %t for env %q", tc.name, *enabled, value)
				}
				if err := flag.CommandLine.Parse([]string{"--" + tc.name + "=" + strconv.FormatBool(value == "false")}); err != nil {
					t.Fatal(err)
				}
				if *enabled != (value == "false") {
					t.Fatal("explicit CLI flag did not override the environment")
				}
			})
		}
	}
}
