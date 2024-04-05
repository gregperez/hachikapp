package instance

import (
	"jwt/base/internal/core/ports"
	"jwt/base/internal/infra/primary/config/database"
	"jwt/base/internal/infra/secondary/repositories"
	mongo "jwt/base/internal/infra/secondary/repositories/mongo/dao"
	pg "jwt/base/internal/infra/secondary/repositories/postgres/dao"
)

func GetUserInstance() ports.RepositoryPort {
	// ToDo: Postgresql return repositories.NewUserInstance(GetUserDaoInstance())
	//return repositories.NewUserInstance(GetUserMongoDaoInstance())
	return repositories.NewUserInstance(GetUserDaoInstance())
}

func GetUserDaoInstance() *repositories.IGenericDAO {
	var dbPgx database.Database = database.GetDatabasePostgresPgx()
	return pg.NewUserDaoInstance(&dbPgx)
}

func GetUserMongoDaoInstance() *repositories.IGenericDAO {
	var dbMongo database.IMongo = database.GetDatabaseMongo()
	return mongo.NewUserMongoDaoInstance(&dbMongo)
}
