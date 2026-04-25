package stack

import (
	"errors"
	"testing"
)

// makeServices builds a []NamedService from an ordered key list and a dep map.
// Each service gets image "<key>:latest" so it passes the image check.
func makeServices(keys []string, deps map[string][]string) []NamedService {
	result := make([]NamedService, len(keys))
	for i, k := range keys {
		result[i] = NamedService{
			Key: k,
			Def: ServiceDef{
				Image:     k + ":latest",
				DependsOn: deps[k],
			},
		}
	}
	return result
}

// serviceKeys extracts the Key field from each element in order.
func serviceKeys(services []NamedService) []string {
	out := make([]string, len(services))
	for i, ns := range services {
		out[i] = ns.Key
	}
	return out
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// indexOf returns the position of key in the result slice, or -1.
func indexOf(services []NamedService, key string) int {
	for i, ns := range services {
		if ns.Key == key {
			return i
		}
	}
	return -1
}

// ---- no dependencies -------------------------------------------------------

func TestResolveOrder_NoDeps_PreservesYAMLOrder(t *testing.T) {
	svc := makeServices([]string{"a", "b", "c"}, nil)
	got, err := ResolveOrder(svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !equalSlice(serviceKeys(got), []string{"a", "b", "c"}) {
		t.Errorf("got %v, want [a b c]", serviceKeys(got))
	}
}

func TestResolveOrder_Empty(t *testing.T) {
	got, err := ResolveOrder(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

// ---- linear chain ----------------------------------------------------------

func TestResolveOrder_LinearChain(t *testing.T) {
	// YAML order: c, b, a — but c→b→a means a must deploy first.
	svc := makeServices([]string{"c", "b", "a"}, map[string][]string{
		"c": {"b"},
		"b": {"a"},
	})
	got, err := ResolveOrder(svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !equalSlice(serviceKeys(got), []string{"a", "b", "c"}) {
		t.Errorf("got %v, want [a b c]", serviceKeys(got))
	}
}

func TestResolveOrder_SingleWithNoDeps(t *testing.T) {
	svc := makeServices([]string{"solo"}, nil)
	got, err := ResolveOrder(svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !equalSlice(serviceKeys(got), []string{"solo"}) {
		t.Errorf("got %v, want [solo]", serviceKeys(got))
	}
}

// ---- diamond (two independent, one that needs both) -----------------------

func TestResolveOrder_Diamond_ApiLast(t *testing.T) {
	// api depends on mongodb and redis; mongodb and redis are independent.
	svc := makeServices([]string{"mongodb", "redis", "api"}, map[string][]string{
		"api": {"mongodb", "redis"},
	})
	got, err := ResolveOrder(svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotKeys := serviceKeys(got)
	apiIdx := indexOf(got, "api")
	if apiIdx != len(got)-1 {
		t.Errorf("api should be last, got order %v", gotKeys)
	}
	if indexOf(got, "mongodb") >= apiIdx || indexOf(got, "redis") >= apiIdx {
		t.Errorf("mongodb and redis must appear before api, got %v", gotKeys)
	}
}

func TestResolveOrder_Diamond_IndependentPreservesYAMLOrder(t *testing.T) {
	// mongodb and redis have no mutual dependency; YAML order should be kept.
	svc := makeServices([]string{"mongodb", "redis", "api"}, map[string][]string{
		"api": {"mongodb", "redis"},
	})
	got, err := ResolveOrder(svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotKeys := serviceKeys(got)
	// The two independent services must appear in their original order.
	if gotKeys[0] != "mongodb" || gotKeys[1] != "redis" {
		t.Errorf("independent services should preserve YAML order, got %v", gotKeys)
	}
}

// ---- mixed: some services with deps, some without -------------------------

func TestResolveOrder_Mixed(t *testing.T) {
	// nginx has no deps; api depends on redis; redis has no deps.
	// YAML order: nginx, redis, api
	svc := makeServices([]string{"nginx", "redis", "api"}, map[string][]string{
		"api": {"redis"},
	})
	got, err := ResolveOrder(svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if indexOf(got, "redis") >= indexOf(got, "api") {
		t.Errorf("redis must come before api, got %v", serviceKeys(got))
	}
	// nginx should come first (no deps, first in YAML)
	if serviceKeys(got)[0] != "nginx" {
		t.Errorf("nginx should be first, got %v", serviceKeys(got))
	}
}

// ---- error: cycle ----------------------------------------------------------

func TestResolveOrder_TwoNodeCycle(t *testing.T) {
	svc := makeServices([]string{"a", "b"}, map[string][]string{
		"a": {"b"},
		"b": {"a"},
	})
	_, err := ResolveOrder(svc)
	if err == nil {
		t.Fatal("expected *GraphError for two-node cycle, got nil")
	}
	var ge *GraphError
	if !errors.As(err, &ge) {
		t.Fatalf("expected *GraphError, got %T: %v", err, err)
	}
	if len(ge.CycleMembers) != 2 {
		t.Errorf("expected 2 cycle members, got %v", ge.CycleMembers)
	}
}

func TestResolveOrder_ThreeNodeCycle(t *testing.T) {
	// a→b→c→a
	svc := makeServices([]string{"a", "b", "c"}, map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"a"},
	})
	_, err := ResolveOrder(svc)
	var ge *GraphError
	if !errors.As(err, &ge) {
		t.Fatalf("expected *GraphError, got %T: %v", err, err)
	}
	if len(ge.CycleMembers) != 3 {
		t.Errorf("expected 3 cycle members, got %v", ge.CycleMembers)
	}
}

func TestResolveOrder_PartialCycle_IndependentServiceSucceeds(t *testing.T) {
	// d is independent; a↔b form a cycle. ResolveOrder should fail because
	// the overall graph cannot be fully resolved.
	svc := makeServices([]string{"d", "a", "b"}, map[string][]string{
		"a": {"b"},
		"b": {"a"},
	})
	_, err := ResolveOrder(svc)
	var ge *GraphError
	if !errors.As(err, &ge) {
		t.Fatalf("expected *GraphError, got %T: %v", err, err)
	}
	// Only a and b should be reported, not d.
	for _, m := range ge.CycleMembers {
		if m == "d" {
			t.Errorf("independent service d should not appear in cycle members: %v", ge.CycleMembers)
		}
	}
}

// ---- error: self-dependency (caught by graph as a one-node cycle) ----------

func TestResolveOrder_SelfDependency(t *testing.T) {
	svc := makeServices([]string{"a"}, map[string][]string{
		"a": {"a"},
	})
	_, err := ResolveOrder(svc)
	var ge *GraphError
	if !errors.As(err, &ge) {
		t.Fatalf("expected *GraphError, got %T: %v", err, err)
	}
}
