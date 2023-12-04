package repositories

type IGenericDAO interface {
	Save(c interface{}) error
	FindOne(id string) (interface{}, error)
}
