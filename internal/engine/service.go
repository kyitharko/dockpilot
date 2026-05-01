package engine

// DeployRequest is the input for deploying a single service.
// If Image is empty the engine looks up the service name in the built-in registry.
type DeployRequest struct {
	Name    string   `json:"name"`
	Image   string   `json:"image,omitempty"`
	Ports   []string `json:"ports,omitempty"`
	Volumes []string `json:"volumes,omitempty"`
	Env     []string `json:"env,omitempty"`
	Command []string `json:"command,omitempty"`
}

// DeployResult is returned after a successful deploy.
type DeployResult struct {
	Name      string   `json:"name"`
	Container string   `json:"container"`
	Image     string   `json:"image"`
	Ports     []string `json:"ports"`
}

// ServiceStatus describes the runtime state of a deployed service.
type ServiceStatus struct {
	Name      string `json:"name"`
	Container string `json:"container"`
	Image     string `json:"image"`
	State     string `json:"state"`
	Ports     string `json:"ports"`
	Running   bool   `json:"running"`
}
