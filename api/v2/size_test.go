package v2

import "testing"

func TestSizeCanonical(t *testing.T) {
	tests := []struct {
		name string
		size Size
		want Size
	}{
		{name: "canonical 2xlarge", size: Size2XLarge, want: Size2XLarge},
		{name: "legacy xxlarge", size: SizeXXLarge, want: Size2XLarge},
		{name: "other size", size: SizeLarge, want: SizeLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.size.Canonical(); got != tt.want {
				t.Fatalf("Canonical() = %q, want %q", got, tt.want)
			}
		})
	}
}
