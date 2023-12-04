package repositories

type IRepository interface {
	Save(c interface{}) error
}
