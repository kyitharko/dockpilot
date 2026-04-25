package cmd

import (
	"fmt"
	"strings"

	"myplatform/internal/services"
)

// instanceTarget is the resolved result of a user-supplied argument to
// status or remove. It holds everything needed to act on a single container.
type instanceTarget struct {
	cfg           services.ServiceConfig // the parent service definition
	containerName string                 // exact Docker container name
	volumes       []string               // per-instance volume mappings
}

// parseInstanceArg resolves the argument passed to status / remove.
//
// Two forms are accepted:
//
//  1. Service name  (e.g. "mongodb")
//     Optionally combined with --name to target a specific instance:
//     --name staging  →  myplatform-mongodb-staging
//     --name 2        →  myplatform-mongodb-2
//     (no --name)     →  myplatform-mongodb   (the default instance)
//
//  2. Full container name  (e.g. "myplatform-mongodb-2")
//     Detected automatically; --name is ignored in this form.
//     The parent service is inferred from the name so volumes can be derived.
func parseInstanceArg(arg, nameFlag string) (instanceTarget, error) {
	arg = strings.ToLower(strings.TrimSpace(arg))

	// — Form 1: known service name —
	if cfg, ok := services.Get(arg); ok {
		containerName := cfg.ContainerName
		if nameFlag != "" {
			containerName = cfg.ContainerName + "-" + nameFlag
		}
		return instanceTarget{
			cfg:           cfg,
			containerName: containerName,
			volumes:       remapVolumeNames(cfg.Volumes, cfg.ContainerName, containerName),
		}, nil
	}

	// — Form 2: full container name (myplatform-<anything>[-suffix]) —
	if strings.HasPrefix(arg, "myplatform-") {
		inner := strings.TrimPrefix(arg, "myplatform-") // e.g. "mongodb-2", "myapp-staging"
		// Try registered services first so volume remapping works for built-in types.
		for _, svcName := range services.Names() {
			if inner == svcName || strings.HasPrefix(inner, svcName+"-") {
				cfg, _ := services.Get(svcName)
				return instanceTarget{
					cfg:           cfg,
					containerName: arg,
					volumes:       remapVolumeNames(cfg.Volumes, cfg.ContainerName, arg),
				}, nil
			}
		}
		// No registered service matched — treat as a custom container with no volumes.
		return instanceTarget{
			cfg:           services.ServiceConfig{Name: inner, ContainerName: arg},
			containerName: arg,
		}, nil
	}

	return instanceTarget{}, fmt.Errorf(
		"%q is not a known service name or a myplatform container name\navailable services: %s",
		arg, strings.Join(services.Names(), ", "),
	)
}

// remapVolumeNames replaces the base container name prefix inside each volume
// mapping so that every instance gets its own isolated volume.
//
// Example:  "myplatform-mongodb-data:/data/db"  →  "myplatform-mongodb-2-data:/data/db"
func remapVolumeNames(volumes []string, oldBase, newBase string) []string {
	if oldBase == newBase {
		return volumes
	}
	out := make([]string, len(volumes))
	for i, vol := range volumes {
		out[i] = strings.Replace(vol, oldBase, newBase, 1)
	}
	return out
}

// serviceFromArg is a thin wrapper used only by the deploy command, which
// works exclusively with service names (not full container names).
func serviceFromArg(arg string) (services.ServiceConfig, error) {
	name := strings.ToLower(arg)
	cfg, ok := services.Get(name)
	if !ok {
		return services.ServiceConfig{}, fmt.Errorf(
			"unknown service %q — available: %s", name, strings.Join(services.Names(), ", "),
		)
	}
	return cfg, nil
}
