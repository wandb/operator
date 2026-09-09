package main

import (
	"flag"
	"testing"
)

func TestManagedSpecFlagFromEnvironment(t *testing.T) {
	for _, value := range []string{"true", "false"} {
		t.Run(value, func(t *testing.T) {
			original := flag.CommandLine
			flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
			t.Cleanup(func() { flag.CommandLine = original })
			enabled := flag.Bool("managed-spec-enabled", false, "")
			t.Setenv("MANAGED_SPEC_ENABLED", value)
			if errs := setFlagsFromEnvironment(); len(errs) != 0 {
				t.Fatalf("setting flags: %v", errs)
			}
			if *enabled != (value == "true") {
				t.Fatalf("managed spec enabled = %t for env %q", *enabled, value)
			}
		})
	}
}
