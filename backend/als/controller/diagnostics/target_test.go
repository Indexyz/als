package diagnostics

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/samlm0/als/v2/config"
)

func TestTargets(t *testing.T) {
	data, err := os.ReadFile("../../../../ui/src/helper/diagnostic-targets.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases map[string][]string
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for kind, targets := range cases {
		for _, target := range targets {
			t.Run(kind+"/"+target, func(t *testing.T) {
				if got := ValidTarget(target); got != (kind == "valid") {
					t.Fatalf("ValidTarget(%q)=%v", target, got)
				}
			})
		}
	}
	max := strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61)
	if !ValidTarget(max) || !ValidTarget(max+".") || ValidTarget(max+"d") || ValidTarget(strings.Repeat("a", 64)+".com") {
		t.Fatal("invalid DNS length boundary")
	}
}

func TestRequestRestrictions(t *testing.T) {
	cfg := config.GetDefaultConfig()
	for _, test := range []struct{ body, code string }{
		{`{"type":"start","tool":"ping","target":"127.0.0.1"}`, ""},
		{`{"type":"start","tool":"ping","target":"127.0.0.1;id"}`, "invalid_target"},
		{`{"type":"start","tool":"sh","target":"example.com"}`, "invalid_tool"},
		{`{"type":"stop"}`, "invalid_request"},
		{`{"type":"start","tool":"ping","target":"example.com","family":"-f"}`, "invalid_family"},
	} {
		var req request
		if err := decode([]byte(test.body), &req); err != nil {
			t.Fatal(err)
		}
		err := validateRequest(&req, cfg)
		if test.code == "" {
			if err != nil {
				t.Fatal(err)
			}
		} else if err == nil || err.code != test.code {
			t.Fatalf("%s: got %v", test.body, err)
		}
	}
	for _, body := range []string{
		`{"type":"start","tool":"ping","target":"example.com","args":["-f"]}`,
		`{"type":"start"} {"type":"stop"}`, `[]`, `"start"`,
	} {
		if decode([]byte(body), new(request)) == nil {
			t.Errorf("accepted invalid JSON request: %s", body)
		}
	}
	cfg.FeatureNextTrace = false
	r := request{Type: "start", Tool: "nexttrace", Target: "example.com"}
	if err := validateRequest(&r, cfg); err == nil || err.code != "disabled" {
		t.Fatal("disabled NextTrace was accepted")
	}
}

func TestResolveLiteralFamily(t *testing.T) {
	if _, err := resolveTarget(context.Background(), "::1", "4"); err == nil {
		t.Fatal("mismatched family accepted")
	}
	addr, err := resolveTarget(context.Background(), "::ffff:192.0.2.1", "4")
	if err != nil || addr.String() != "192.0.2.1" {
		t.Fatalf("mapped IPv4: %s %v", addr, err)
	}
}
