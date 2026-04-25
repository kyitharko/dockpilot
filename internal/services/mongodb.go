package services

func init() {
	Register(ServiceConfig{
		Name:          "mongodb",
		Image:         "mongo:latest",
		ContainerName: "myplatform-mongodb",
		Ports:         []string{"27017:27017"},
		Volumes:       []string{"myplatform-mongodb-data:/data/db"},
	})
}
