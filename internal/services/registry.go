package services

import "sort"

// ServiceDef is the built-in definition for a known service.
// The engine uses it when no custom image/config is provided in a deploy request.
type ServiceDef struct {
	Name    string
	Image   string
	Ports   []string
	Volumes []string // named volume mounts in "volumeName:mountPath" form
	Env     []string
	Command []string
}

var registry = map[string]ServiceDef{}

// Register adds a ServiceDef to the built-in registry.
// Called from init() in each service-specific file.
func Register(def ServiceDef) {
	registry[def.Name] = def
}

// Get returns the ServiceDef for name and whether it was found.
func Get(name string) (ServiceDef, bool) {
	def, ok := registry[name]
	return def, ok
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

// All returns all registered ServiceDefs.
func All() []ServiceDef {
	defs := make([]ServiceDef, 0, len(registry))
	for _, def := range registry {
		defs = append(defs, def)
	}
	return defs
}
