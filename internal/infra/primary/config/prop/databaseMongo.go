package prop

import "sync"

var (
	databaseMongoPropertyInstanceOnce sync.Once
	databaseMongoPropertyInstance     *DatabaseMongoProperty
)

func GetDatabaseMongoProperty() *DatabaseMongoProperty {
	databaseMongoPropertyInstanceOnce.Do(func() {
		databaseMongoPropertyInstance = &DatabaseMongoProperty{}
	})

	return databaseMongoPropertyInstance
}

type DatabaseMongoProperty struct {
	DataSourceMongo `yaml:"datasourcemongo"`
}

type DataSourceMongo struct {
	Url string `yaml:"url"`
}
