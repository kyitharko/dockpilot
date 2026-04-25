package services

import "sort"

// ServiceConfig is the canonical description of a deployable service.
// It is intentionally flat and serialisation-friendly so it can later be
// loaded from YAML/TOML configuration files without changing callers.
type ServiceConfig struct {
	// Name is the CLI identifier (e.g. "mongodb").
	Name string
	// Image is the Docker image reference.
	Image string
	// ContainerName is the name Docker assigns to the running container.
	ContainerName string
	// Ports maps host ports to container ports ("hostPort:containerPort").
	Ports []string
	// Volumes maps named volumes to mount paths ("volumeName:mountPath").
	// Only named volumes are supported; bind-mounts are out of scope.
	Volumes []string
	// Env holds environment variables in "KEY=VALUE" form.
	Env []string
	// Command overrides the default container entrypoint command.
	Command []string
}

// registry is the single source of truth for known services.
// It is populated by each service file's init() function.
var registry = map[string]ServiceConfig{}

// Register adds a ServiceConfig to the registry.
// Called from the init() of each service-specific file.
func Register(cfg ServiceConfig) {
	registry[cfg.Name] = cfg
}

// Get returns the ServiceConfig for the given name, and whether it was found.
func Get(name string) (ServiceConfig, bool) {
	cfg, ok := registry[name]
	return cfg, ok
}

// Names returns all registered service names in sorted order.
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// All returns all registered ServiceConfigs.
func All() []ServiceConfig {
	cfgs := make([]ServiceConfig, 0, len(registry))
	for _, cfg := range registry {
		cfgs = append(cfgs, cfg)
	}
	return cfgs
}
