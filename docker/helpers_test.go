package docker

import (
	"net/netip"
	"testing"

	"gdocker/models"

	containertypes "github.com/moby/moby/api/types/container"
)

func TestEnvCacheFromContainersUsesFullIDThenID(t *testing.T) {
	containers := []models.Container{
		{ID: "short-a", FullID: "full-a", EnvLoaded: true, Env: []string{"A=1"}},
		{ID: "short-b", FullID: "", EnvLoaded: false, Env: []string{"B=2"}},
	}

	cache := envCacheFromContainers(containers)

	if _, ok := cache["full-a"]; !ok {
		t.Fatalf("expected cache key full-a")
	}
	if _, ok := cache["short-b"]; !ok {
		t.Fatalf("expected cache key short-b")
	}
	if !cache["full-a"].loaded || len(cache["full-a"].env) != 1 {
		t.Fatalf("unexpected cache value for full-a: %#v", cache["full-a"])
	}
	if cache["short-b"].loaded {
		t.Fatalf("expected short-b loaded=false")
	}
}

func TestContainersFromListMapsFieldsAndReusesEnvCache(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	summaries := []containertypes.Summary{
		{
			ID:      id,
			Names:   []string{"/api"},
			Image:   "nginx:latest",
			Created: 1700000000,
			Ports: []containertypes.PortSummary{
				{
					IP:          netip.MustParseAddr("127.0.0.1"),
					PrivatePort: 8080,
					PublicPort:  18080,
					Type:        "tcp",
				},
			},
			Labels: map[string]string{"com.docker.compose.project": "proj"},
			State:  containertypes.ContainerState("running"),
			Status: "Up 1 minute",
		},
	}
	cache := map[string]cachedContainerEnv{
		id: {loaded: true, env: []string{"A=B"}},
	}

	got := containersFromList(summaries, cache)
	if len(got) != 1 {
		t.Fatalf("expected one container, got %d", len(got))
	}
	c := got[0]

	if c.ID != id[:12] {
		t.Fatalf("unexpected short id: %q", c.ID)
	}
	if c.FullID != id {
		t.Fatalf("unexpected full id: %q", c.FullID)
	}
	if c.Name != "api" {
		t.Fatalf("unexpected name: %q", c.Name)
	}
	if c.Project != "proj" {
		t.Fatalf("unexpected project: %q", c.Project)
	}
	if !c.EnvLoaded || len(c.Env) != 1 || c.Env[0] != "A=B" {
		t.Fatalf("unexpected env cache reuse: %#v", c.Env)
	}
	if len(c.Ports) != 1 {
		t.Fatalf("expected one port mapping")
	}
	if c.Ports[0].IP != "127.0.0.1" {
		t.Fatalf("unexpected mapped IP: %q", c.Ports[0].IP)
	}
}

func TestContainerRuntimeIDPrefersFullID(t *testing.T) {
	c := &models.Container{ID: "short", FullID: "full"}
	if got := containerRuntimeID(c); got != "full" {
		t.Fatalf("expected full ID, got %q", got)
	}

	c = &models.Container{ID: "short"}
	if got := containerRuntimeID(c); got != "short" {
		t.Fatalf("expected short ID fallback, got %q", got)
	}
}

func TestShortIDTruncates(t *testing.T) {
	if got := shortID("1234567890123456"); got != "123456789012" {
		t.Fatalf("unexpected short id: %q", got)
	}
	if got := shortID("abc"); got != "abc" {
		t.Fatalf("unexpected unchanged short id: %q", got)
	}
}
