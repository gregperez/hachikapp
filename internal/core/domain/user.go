package domain

import middlewareDomain "github.com/WilsonSayago/middleware/core/domains"

type EntityEnum string
type OperationEnum string

type User struct {
	middlewareDomain.UserGeneric
}
