package analytics

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestIsolation(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{name: "positive control", args: []string{"list", "-deps", "./internal/analytics"}, want: true},
		{name: "safety and command path", args: []string{"list", "-deps", "./internal/safety", "./internal/storage/publisher"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, "go", testCase.args...)
			command.Dir = "../.."
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("go list failed: %v\n%s", err, output)
			}
			found := false
			for _, dependency := range strings.Fields(string(output)) {
				if strings.HasSuffix(dependency, "/internal/analytics") {
					found = true
				}
			}
			if found != testCase.want {
				t.Fatalf("analytics dependency = %t, want %t", found, testCase.want)
			}
		})
	}
}
