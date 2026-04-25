package stack

import (
	"fmt"
	"sort"
	"strings"
)

// GraphError is returned when the dependency graph contains a cycle.
type GraphError struct {
	CycleMembers []string // service keys that are stuck in the cycle
}

func (e *GraphError) Error() string {
	return fmt.Sprintf("circular dependency detected involving: %s",
		strings.Join(e.CycleMembers, ", "))
}

// ResolveOrder returns services sorted in topological (dependency-first) order
// using Kahn's algorithm. A service always appears after every service it lists
// in depends_on. Among services with equal priority, original YAML order is
// preserved so the output is deterministic.
//
// Precondition: all keys in depends_on exist in the services slice and no
// service depends on itself. The validator enforces both; this function skips
// unknown keys silently rather than panicking so it can be used defensively.
//
// Returns *GraphError when a cycle prevents a complete ordering.
func ResolveOrder(services []NamedService) ([]NamedService, error) {
	n := len(services)
	if n == 0 {
		return nil, nil
	}

	// Map service key → slice index for O(1) lookup.
	idx := make(map[string]int, n)
	for i, ns := range services {
		idx[ns.Key] = i
	}

	// inDegree[i] = number of unresolved dependencies service i still has.
	inDegree := make([]int, n)
	// dependents[i] = indices of services that list service i in their depends_on.
	dependents := make([][]int, n)

	for i, ns := range services {
		for _, dep := range ns.Def.DependsOn {
			depIdx, ok := idx[dep]
			if !ok {
				continue // unknown dep — validator should have caught this
			}
			inDegree[i]++
			dependents[depIdx] = append(dependents[depIdx], i)
		}
	}

	// Seed the queue with all ready services (no deps).
	// Iterating 0..n-1 keeps original YAML order among equals.
	queue := make([]int, 0, n)
	for i, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, i)
		}
	}

	result := make([]NamedService, 0, n)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		result = append(result, services[cur])

		// Sort newly unblocked dependents by original index so that among
		// services that become ready at the same time, YAML order is preserved.
		deps := make([]int, len(dependents[cur]))
		copy(deps, dependents[cur])
		sort.Ints(deps)

		for _, d := range deps {
			inDegree[d]--
			if inDegree[d] == 0 {
				queue = append(queue, d)
			}
		}
	}

	if len(result) != n {
		var stuck []string
		for i, deg := range inDegree {
			if deg > 0 {
				stuck = append(stuck, services[i].Key)
			}
		}
		sort.Strings(stuck)
		return nil, &GraphError{CycleMembers: stuck}
	}
	return result, nil
}
