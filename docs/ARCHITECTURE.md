# Arquitectura de base-golang

`base-golang` es una API HTTP de autenticación, usuarios y roles construida con
arquitectura hexagonal. El núcleo depende de contratos; Gin, PostgreSQL,
`authBase/v4` e `initModules/v2` se conectan en los adaptadores y el bootstrap.

## Capas y dependencias

- `cmd/`: entrypoints del servidor y del bootstrap administrativo.
- `internal/core/domain/`: modelo de usuario, errores y reglas puras.
- `internal/core/ports/`: contratos propios de persistencia y auditoría.
- `internal/core/services/`: casos de uso de usuario y bootstrap de
  administrador.
- `internal/infra/primary/`: controllers Gin, DTOs, middleware y traducción del
  contrato HTTP.
- `internal/infra/secondary/repositories/`: implementación de ports,
  PostgreSQL DAOs y `RefreshTokenStore`.
- `internal/infra/config/instance/`: composition root de repositorios, casos de
  uso, `authBase` y controllers.
- `internal/infra/bootstrap/`: engine HTTP, health probes, señales y shutdown.
- `internal/resources/sql/migrations/`: evolución incremental del esquema.

La dirección principal es:

```text
HTTP/Gin → controllers → use cases/ports → repositories/DAO → PostgreSQL
```

El dominio no conoce Gin, pgx ni detalles de JWT. Los controllers dependen de
interfaces de casos de uso y traducen errores a `application/problem+json`.

## Arranque y apagado

`cmd/main.go` mantiene un bootstrap explícito:

1. Registra y carga propiedades YAML con expansión de entorno.
2. Valida JWT y configuración runtime.
3. Construye el logger JSON con `log/slog`.
4. Abre un único pool pgx y verifica PostgreSQL.
5. Ejecuta migraciones con el mismo DSN del pool.
6. Construye traducción, readiness, middleware y rutas.
7. Activa readiness y ejecuta `http.Server`.
8. Ante `SIGHUP`, `SIGINT`, `SIGTERM` o `SIGQUIT`, baja readiness, ejecuta
   `Shutdown` con deadline y cierra el pool.

La decisión de no usar `initModules.App` para el lifecycle HTTP está registrada
en [ADR 001](adr/001-initmodules-lifecycle.md). `initModules/v2` permanece para
carga de propiedades y singletons tipados.

## Autenticación y autorización

`authBase/v4` aporta contratos y servicios; la aplicación aporta los
adaptadores:

- `UserRepository` implementa lectura de identidad y credenciales.
- `refreshstore.Store` persiste hashes de refresh token y rota en una
  transacción.
- `ginadapter.Context` adapta Gin al contexto esperado por autorización.
- `AuthenticationService` implementa login, refresh y revocación de familia.
- `Authorization` aplica `AuthorizeJWT` y `PoliciesGuard`.
- `RoleService` usa el `RolePort` PostgreSQL de la aplicación.

Rutas públicas:

- `POST /v1/auth/login`
- `POST /v1/auth/refresh-token`
- `POST /v1/auth/logout`
- `GET /health/live`
- `GET /health/ready`

Las rutas de usuarios y roles requieren access token; las operaciones
administrativas además requieren permisos por entidad y acción.

## Paginación keyset

Los listados `GET /v1/users` y `GET /v1/roles` aceptan `limit` y un `cursor`
opaco. El adaptador HTTP:

1. valida `limit` entre 1 y 100;
2. decodifica el cursor a `authBase/domain.CursorKey`;
3. llama al caso de uso con `PageRequest`;
4. consulta por `(created_at, id) DESC` usando `LIMIT + 1`;
5. devuelve `hasNext` y codifica `nextCursor` desde el último elemento.

No se usan offset, número de página ni conteos exactos.

## Persistencia y migraciones

PostgreSQL es la fuente de verdad para usuarios, roles, permisos, asociaciones
y sesiones refresh. Las migraciones se aplican en orden y no deben reescribirse
después de llegar a un entorno persistente. Los cambios correctivos se agregan
como migraciones forward-only; [ADR 002](adr/002-migration-baseline.md) explica
la política de baseline.

Los timestamps `created_at` y `updated_at` se administran en base de datos, lo
que mantiene consistente el keyset entre procesos.

## Observabilidad y límites operativos

- Logs estructurados JSON mediante `log/slog`.
- `X-Request-ID` se conserva o genera y se incluye en access logs/problemas.
- `/health/live` informa vida del proceso.
- `/health/ready` exige gate activo y `Ping` PostgreSQL con timeout.
- CORS, límites de body, recovery y timeouts HTTP se configuran antes de rutas.
- Swagger puede deshabilitarse por configuración.

## Diagramas

- [Mapa de arquitectura](diagrams/base-golang-architecture.html)
- [Arranque y apagado](diagrams/base-golang-startup.html)
- [Listado protegido con cursor](diagrams/base-golang-paginated-request.html)
- [Índice y regeneración](diagrams/README.md)

Los JSON Archify versionados junto a los HTML son la fuente editable de cada
diagrama.
