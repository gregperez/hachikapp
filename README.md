# Project Name

## Overview
This project is a Go-based application that uses PostgreSQL for data storage and includes various libraries for handling authentication, database operations, and more.

## Libraries

### Go Modules
- **github.com/WilsonSayago/authBase**: A library for handling authentication and authorization.
- **github.com/sirupsen/logrus**: A structured logger for Go.
- **github.com/WilsonSayago/initModules**: A library for initializing modules in a Go application.

### Internal Packages
- **jwt/base/internal/core/ports**: Contains the core ports/interfaces for the application.
- **jwt/base/internal/infra/config/database**: Handles database configuration and connections.
- **jwt/base/internal/infra/secondary/repositories**: Contains repository implementations for data access.
- **jwt/base/internal/infra/secondary/repositories/postgres/dao**: Data access objects for PostgreSQL.

## Database
The project uses PostgreSQL as the primary database. The database schema includes a `users` table with the following fields:
- `name`
- `password`
- `email`
- `is_admin`

## Migrations
Database migrations are managed using SQL files. An example migration file is `000002_insert_users_table.up.sql`, which inserts an admin user into the `users` table.

```sh
  migrate create -ext sql -dir internal/resources/sql/migrations -seq insert_users_table
```

## Configuration
The project includes configuration for different instances such as user repository, role repository, and validation service. These configurations are defined in `internal/infra/config/instance/port.go`.

## Usage
To run the project, ensure you have Go installed and set up your PostgreSQL database. Then, use the following commands:

```sh
go mod tidy
go run main.go