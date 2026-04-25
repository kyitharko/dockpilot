package services

func init() {
	Register(ServiceConfig{
		Name:          "postgres",
		Image:         "postgres:latest",
		ContainerName: "myplatform-postgres",
		Ports:         []string{"5432:5432"},
		Volumes:       []string{"myplatform-postgres-data:/var/lib/postgresql/data"},
		Env: []string{
			"POSTGRES_USER=admin",
			"POSTGRES_PASSWORD=admin123",
			"POSTGRES_DB=appdb",
		},
	})
}
