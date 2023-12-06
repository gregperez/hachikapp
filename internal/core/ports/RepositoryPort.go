package ports

type RepositoryPort interface {
	Save(v interface{}) error
	FindById(id string) (interface{}, error)
}
