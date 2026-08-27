package workspace

import "testing"

func strPtr(s string) *string { return &s }

func TestExternalLaunchURLPassthroughWhenProxyDisabled(t *testing.T) {
	getenv := func(string) string { return "" }
	raw := strPtr("http://127.0.0.1:51234")
	got := externalLaunchURL("ws-1", "app-1", raw, getenv)
	if got == nil || *got != "http://127.0.0.1:51234" {
		t.Fatalf("expected passthrough, got %v", got)
	}
}

func TestExternalLaunchURLProxyPathWhenEnabled(t *testing.T) {
	getenv := func(key string) string {
		if key == AppProxyEnabledEnv {
			return "1"
		}
		return ""
	}
	raw := strPtr("http://127.0.0.1:51234")
	got := externalLaunchURL("ws-1", "app-1", raw, getenv)
	want := "/v1/workspaces/ws-1/apps/app-1/proxy/"
	if got == nil || *got != want {
		t.Fatalf("expected %q, got %v", want, got)
	}
}

func TestExternalLaunchURLEscapesIDs(t *testing.T) {
	getenv := func(key string) string {
		if key == AppProxyEnabledEnv {
			return "true"
		}
		return ""
	}
	raw := strPtr("http://127.0.0.1:51234")
	got := externalLaunchURL("ws/1", "app 1", raw, getenv)
	want := "/v1/workspaces/ws%2F1/apps/app%201/proxy/"
	if got == nil || *got != want {
		t.Fatalf("expected %q, got %v", want, got)
	}
}

func TestExternalLaunchURLNilPassthrough(t *testing.T) {
	getenv := func(key string) string {
		if key == AppProxyEnabledEnv {
			return "1"
		}
		return ""
	}
	if got := externalLaunchURL("ws-1", "app-1", nil, getenv); got != nil {
		t.Fatalf("expected nil passthrough, got %v", got)
	}
	empty := strPtr("   ")
	if got := externalLaunchURL("ws-1", "app-1", empty, getenv); got != empty {
		t.Fatalf("expected empty passthrough unchanged")
	}
}
