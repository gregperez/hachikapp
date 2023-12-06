package repositories

type IGenericDAO interface {
	Save(c interface{}) error
	FindById(id string) (interface{}, error)
	FindByUserId(userId string) ([]interface{}, error)
}
