package instance

import (
	portsAuthBase "github.com/WilsonSayago/authBase/core/port"
	"github.com/WilsonSayago/authBase/infra/secundary"
	"jwt/base/internal/core/ports"
	database2 "jwt/base/internal/infra/config/database"
	"jwt/base/internal/infra/secondary/repositories"
	pg "jwt/base/internal/infra/secondary/repositories/postgres/dao"
)

func GetUserRepositoryInstance() ports.UserPort {
	// ToDo: Postgresql return repositories.NewUserInstance(GetUserDaoInstance())
	//return repositories.NewUserInstance(GetUserMongoDaoInstance())
	return repositories.NewUserInstance(GetUserDaoInstance(), GetRoleDaoInstance(), GetValidationService())
}

func GetRoleRepositoryInstance() portsAuthBase.RolePort {
	return repositories.NewRoleInstance(GetRoleDaoInstance())
}

func GetUserDaoInstance() pg.IUserDao {
	var dbPgx database2.Database = database2.GetDatabasePostgresPgx()
	return pg.NewUserDaoInstance(&dbPgx)
}

func GetPermissionDaoInstance() pg.IPermissionDao {
	var dbPgx database2.Database = database2.GetDatabasePostgresPgx()
	return pg.NewPermissionDaoInstance(&dbPgx)
}

func GetRoleDaoInstance() pg.IRoleDao {
	var dbPgx database2.Database = database2.GetDatabasePostgresPgx()
	return pg.NewRoleDaoInstance(&dbPgx)
}

func GetValidationService() portsAuthBase.ValidationPort {
	return secundary.NewValidationService()
}
