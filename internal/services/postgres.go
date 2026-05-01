package services

func init() {
	Register(ServiceDef{
		Name:    "postgres",
		Image:   "postgres:latest",
		Ports:   []string{"5432:5432"},
		Volumes: []string{"dockpilot-postgres-data:/var/lib/postgresql/data"},
		Env: []string{
			"POSTGRES_USER=admin",
			"POSTGRES_PASSWORD=admin123",
			"POSTGRES_DB=appdb",
		},
	})
}
