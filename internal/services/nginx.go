package services

func init() {
	Register(ServiceDef{
		Name:  "nginx",
		Image: "nginx:latest",
		Ports: []string{"8080:80"},
	})
}
