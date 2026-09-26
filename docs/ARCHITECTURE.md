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
- `internal/infra/bootstrap/`: composition root aislado (`Application`),
  engine HTTP, health probes y adapters `Lifecycle`.
- `internal/resources/sql/migrations/`: evolución incremental del esquema.

La dirección principal es:

```text
HTTP/Gin → controllers → use cases/ports → repositories/DAO → PostgreSQL
```

El dominio no conoce Gin, pgx ni detalles de JWT. Los controllers dependen de
interfaces de casos de uso y traducen errores a `application/problem+json`.

## Arranque y apagado

`cmd/main.go` carga configuración, crea un logger y construye un
`bootstrap.Application` aislado. El `initModules.App` de esa instancia registra
lifecycles en este orden:

1. Base de datos: ping al iniciar, close al detener.
2. Traducciones: carga de locales.
3. Limpieza de sesiones expiradas del plan 013.
4. Limpieza de transacciones/grants OIDC (si el login federado está habilitado).
5. HTTP: listen al iniciar; readiness false y `Shutdown` al detener.

`RunWithSignals` bloquea hasta una señal y detiene en orden inverso
(HTTP → workers → DB) con un contexto de stop independiente del contexto
cancelado. La decisión vigente está en
[ADR 001](adr/001-initmodules-lifecycle.md).

## Autenticación y autorización

`authBase/v4` aporta contratos y servicios; la aplicación aporta los
adaptadores:

- `UserRepository` implementa lectura de identidad y credenciales.
- `refreshstore.Store` persiste hashes, rota atómicamente, revoca por usuario y
  purga expiradas en lotes limitados.
- `ginadapter.Context` adapta Gin al contexto esperado por autorización.
- `AuthenticationService` implementa login, emisión privilegiada, refresh y
  revocación de familia/usuario.
- El login federado OIDC vive en `base-golang`: un adaptador provider-neutral
  verifica Authorization Code + PKCE, resuelve `(issuer, subject)` y llama
  `EstablishSession`. No hay auto-registro ni auto-enlace por email. El
  callback redirige con un grant opaco; los JWT locales salen solo de
  `POST /v1/auth/oidc/google/exchange`.
- `Authorization` aplica `AuthorizeJWT` y `PoliciesGuard`.
- `RoleService` usa el `RolePort` PostgreSQL de la aplicación.

Rutas públicas:

- `POST /v1/auth/login`
- `POST /v1/auth/refresh-token`
- `POST /v1/auth/logout`
- `GET /v1/auth/oidc/google/login`
- `GET /v1/auth/oidc/google/callback`
- `POST /v1/auth/oidc/google/exchange`
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

Cambiar contraseña o desactivar una cuenta actualiza `users` y revoca sus
sesiones en una única transacción. El componente
`infra/lifecycle.ExpiredSessionPurger` ejecuta limpieza inicial y periódica con
retención configurable; su registro en `initModules.App` se difiere al plan 015
para preservar el orden de apagado antes de cerrar PostgreSQL.

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
