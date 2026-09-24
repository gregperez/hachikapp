package instance

import (
	"github.com/WilsonSayago/base-golang/internal/core/ports"
	database2 "github.com/WilsonSayago/base-golang/internal/infra/config/database"
	"github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories"
	pg "github.com/WilsonSayago/base-golang/internal/infra/secondary/repositories/postgres/dao"

	portsAuthBase "github.com/WilsonSayago/authBase/v3/core/port"
	"github.com/WilsonSayago/authBase/v3/infra/secundary"
)

func GetUserRepositoryInstance() ports.UserPort {
	return repositories.NewUserInstance(GetUserDaoInstance(), GetRoleDaoInstance(), GetUserRoleDaoInstance(), GetValidationService())
}

func GetRoleRepositoryInstance() portsAuthBase.RolePort {
	return repositories.NewRoleInstance(GetRoleDaoInstance())
}

func GetUserDaoInstance() pg.IUserDao {
	return pg.NewUserDaoInstance(database2.GetDatabasePostgresPgx())
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

func GetUserRoleDaoInstance() pg.IUserRoleDao {
	return pg.NewUserRoleDaoInstance(database2.GetDatabasePostgresPgx())
}
