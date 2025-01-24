package prop

type DatabaseProperty struct {
	DataSource `yaml:"datasource"`
}

type DataSource struct {
	Host       string `yaml:"host"`
	Port       string `yaml:"port"`
	Name       string `yaml:"database"`
	User       string `yaml:"user"`
	Password   string `yaml:"password"`
	DbMaxConns int32  `yaml:"dbMaxConns"`
}

func NewDatabaseProperty() DatabaseProperty {
	return DatabaseProperty{}
}
