package services

func init() {
	Register(ServiceConfig{
		Name:          "nginx",
		Image:         "nginx:latest",
		ContainerName: "myplatform-nginx",
		Ports:         []string{"8080:80"},
	})
}
