package capabilities

import "testing"

func TestContractClass(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		status   string
		maturity string
		want     string
	}{
		{
			name:     "stable-runtime",
			status:   "runtime_supported",
			maturity: "stable",
			want:     "production-ready",
		},
		{
			name:     "beta-runtime",
			status:   "runtime_supported",
			maturity: "beta",
			want:     "runtime-supported but not stable",
		},
		{
			name:     "experimental-runtime",
			status:   "runtime_supported",
			maturity: "experimental",
			want:     "public-experimental",
		},
		{
			name:     "beta-non-runtime",
			status:   "packaging_only",
			maturity: "beta",
			want:     "public-beta",
		},
		{
			name:     "experimental-non-runtime",
			status:   "packaging_only",
			maturity: "experimental",
			want:     "public-experimental",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := contractClass(tc.status, tc.maturity); got != tc.want {
				t.Fatalf("contractClass(%q, %q) = %q want %q", tc.status, tc.maturity, got, tc.want)
			}
		})
	}
}
