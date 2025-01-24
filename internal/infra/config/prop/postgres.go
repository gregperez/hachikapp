package prop

import (
	"log"
	"sync"
)

var (
	postgresOnce sync.Once
	postgresProp *DatabaseProp
)

type DatabaseProp struct {
	Database Database `yaml:"database"`
}

type Database struct {
	Postgres Postgres `yaml:"postgres"`
}

type Postgres struct {
	Timeout  int    `yaml:"timeout"`
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Db       string `yaml:"db"`
	Port     string `yaml:"port"`
}

func GetPostgresProp() *DatabaseProp {
	postgresOnce.Do(func() {
		postgresProp = &DatabaseProp{}
	})
	return postgresProp
}

func (d *DatabaseProp) Validate() {
	if d.Database.Postgres.Host == "" {
		log.Fatal("postgres.Host is empty")
	}
	if d.Database.Postgres.Db == "" {
		log.Fatal("postgres.Db is empty")
	}
	if d.Database.Postgres.Port == "" {
		log.Fatal("postgres.Port is empty")
	}
	if d.Database.Postgres.User == "" {
		log.Fatal("postgres.User is empty")
	}
	if d.Database.Postgres.Password == "" {
		log.Fatal("postgres.Password is empty")
	}
	if d.Database.Postgres.Timeout < 1 {
		log.Fatal("postgres.Timeout is not more than 1 second")
	}
}
