package diagnostics

import (
	"context"
	"net/netip"
	"strings"
	"testing"

	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/util"
)

func TestNativeNextTraceRejectsCLIPanicMode(t *testing.T) {
	previous := util.EnvDevMode
	util.EnvDevMode = true
	t.Cleanup(func() { util.EnvDevMode = previous })
	called := false
	var output string
	address := netip.MustParseAddr("127.0.0.1")
	result := runNextTrace(context.Background(), request{Tool: "nexttrace", Target: address.String()}, address, func(e event) bool {
		output += e.Line
		return true
	}, func(context.Context, trace.Method, trace.Config) (*trace.Result, error) {
		called = true
		return nil, nil
	})
	if called || result.err == nil || result.err.code != "execution_failed" || !strings.Contains(output, "NEXTTRACE_DEVMODE") {
		t.Fatalf("CLI panic mode reached native probing: %+v %q", result, output)
	}
}
