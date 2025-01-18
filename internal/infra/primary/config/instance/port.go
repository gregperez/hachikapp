package instance

import (
	portsAuthBase "github.com/WilsonSayago/middleware/core/ports"
	"github.com/WilsonSayago/middleware/infra/secundary"
	"jwt/base/internal/core/ports"
	"jwt/base/internal/infra/primary/config/database"
	"jwt/base/internal/infra/secondary/repositories"
	pg "jwt/base/internal/infra/secondary/repositories/postgres/dao"
)

func GetUserRepositoryInstance() ports.UserPort {
	// ToDo: Postgresql return repositories.NewUserInstance(GetUserDaoInstance())
	//return repositories.NewUserInstance(GetUserMongoDaoInstance())
	return repositories.NewUserInstance(GetUserDaoInstance(), GetRoleDaoInstance())
}

func GetUserDaoInstance() pg.IUserDao {
	var dbPgx database.Database = database.GetDatabasePostgresPgx()
	return pg.NewUserDaoInstance(&dbPgx)
}

func GetPermissionDaoInstance() pg.IPermissionDao {
	var dbPgx database.Database = database.GetDatabasePostgresPgx()
	return pg.NewPermissionDaoInstance(&dbPgx)
}

func GetRoleDaoInstance() pg.IRoleDao {
	var dbPgx database.Database = database.GetDatabasePostgresPgx()
	return pg.NewRoleDaoInstance(&dbPgx)
}

func GetValidationService() portsAuthBase.ValidationPort {
	return secundary.NewValidationService()
}
