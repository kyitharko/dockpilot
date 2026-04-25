package services

func init() {
	Register(ServiceConfig{
		Name:          "redis",
		Image:         "redis:latest",
		ContainerName: "myplatform-redis",
		Ports:         []string{"6379:6379"},
	})
}
