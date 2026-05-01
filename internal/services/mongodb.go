package services

func init() {
	Register(ServiceDef{
		Name:    "mongodb",
		Image:   "mongo:latest",
		Ports:   []string{"27017:27017"},
		Volumes: []string{"dockpilot-mongodb-data:/data/db"},
	})
}
