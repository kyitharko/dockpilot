package services

func init() {
	Register(ServiceDef{
		Name:  "redis",
		Image: "redis:latest",
		Ports: []string{"6379:6379"},
	})
}
