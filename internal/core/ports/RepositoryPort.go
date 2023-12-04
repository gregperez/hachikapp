package ports

type RepositoryPort interface {
	Save(v interface{}) error
}
