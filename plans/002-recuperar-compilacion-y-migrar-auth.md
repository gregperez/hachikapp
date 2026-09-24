# Plan 002 — Recuperar compilación y migrar authBase/initModules

## Objetivo

Restablecer un grafo de módulos reproducible y migrar la integración a `authBase/v3.0.0` e `initModules/v2.0.0`, reemplazando el refresh token reutilizable por rotación atómica y adaptando Gin al contrato de contexto de la librería.

- Prioridad: P0
- Esfuerzo: L
- Riesgo: alto; cambia autenticación, sesiones y el path público del módulo.
- Depende de: plan 001
- Rama sugerida: `codex/002-migrar-auth`
- Commit analizado: `99a9826`

## Estado actual verificado

`go.mod` declara un nombre local y no requiere dos módulos que sí se importan:

```go
module jwt/base

go 1.24.0
```

Los controladores y servicios importan `github.com/WilsonSayago/authBase/...`, y `cmd/main.go` importa `github.com/WilsonSayago/initModules/...`, pero ninguno aparece en `go.mod`. Por eso `go test ./...` y `go vet ./...` fallan con “no required module provides package”.

La aplicación usa contratos de una versión antigua de `authBase`. La API verificada de `authBase/v3.0.0` requiere:

```go
type AuthenticationUseCase[T UserData] interface {
    Login(ctx context.Context, username, password string) (*Token, error)
    RefreshToken(ctx context.Context, refreshToken string) (*Token, error)
    ValidateToken(ctx context.Context, token string) (*UserToken[T], error)
    RevokeRefreshFamily(ctx context.Context, refreshToken string) error
}

type Context interface {
    RequestContext() context.Context
}
```

También separa `UserReader`, `CredentialReader` y `RefreshTokenStore`, y valida issuer/audience, usuario activo y replay de refresh. `*gin.Context` no implementa `RequestContext()`, por lo que se necesita un adaptador local explícito.

## Control de deriva

```bash
git rev-parse --short HEAD
git status --short
go env GOVERSION
rg -n 'jwt/base|WilsonSayago/(authBase|initModules)' --glob '*.go' go.mod
rg -n 'NewAuthentication|RefreshToken|GetUserToken|initModules' cmd internal
```

Confirmar además que existen los tags remotos `authBase/v3.0.0` e `initModules/v2.0.0`. Si apareció una versión mayor posterior, no adoptarla automáticamente: primero revisar changelog y abrir un plan de migración separado.

## Alcance

- `go.mod`, `go.sum` y todos los imports del módulo local.
- Configuración JWT necesaria para v3: secretos distintos, issuer, audience, access TTL, refresh TTL y leeway.
- Adaptadores de dominio/repository requeridos por `UserReader`, `CredentialReader` y `RefreshTokenStore`.
- Adaptador Gin para `authBase/v3`.
- Migración nueva para sesiones de refresh y su DAO PostgreSQL con rotación atómica.
- Wiring de autenticación, autorización y middleware.
- Sustitución del arranque legado de `initModules` por APIs v2 sólo hasta dejar wiring determinista; el lifecycle HTTP completo se termina en el plan 005.
- Pruebas de login, validación, refresh, replay, revocación y usuario inactivo.

## Fuera de alcance

- Reestructurar todas las consultas PostgreSQL; plan 006.
- Corregir permisos funcionales de roles/usuarios; plan 004.
- Actualizar el resto de dependencias; plan 009.
- Mantener válidos access/refresh tokens emitidos por la implementación anterior.

## Diseño objetivo

1. Cambiar el módulo a `github.com/WilsonSayago/base-golang`.
2. Declarar `go 1.26.0` y `toolchain go1.27.1`. El mínimo conserva una versión estable anterior y el toolchain fija el compilador actual recomendado.
3. Requerir directamente:

```text
github.com/WilsonSayago/authBase/v3 v3.0.0
github.com/WilsonSayago/initModules/v2 v2.0.0
```

4. Mantener adaptadores bajo infraestructura; el dominio no debe importar Gin, pgx ni detalles JWT.
5. Crear un wrapper, por ejemplo en `internal/infra/primary/ginadapter`:

```go
type Context struct { *gin.Context }

func (c *Context) RequestContext() context.Context {
    return c.Request.Context()
}
```

Agregar funciones adaptadoras de handlers/middleware genéricos v3 a `gin.HandlerFunc`. No modificar `authBase` para acoplarlo a Gin.
6. Separar lectura pública de usuario y credenciales. `UserReader.FindByID` devuelve identidad/estado/roles sin password; `CredentialReader.FindCredentialsByUsername` es el único adaptador que obtiene el hash.
7. Implementar `RefreshTokenStore` con una tabla nueva, por ejemplo `refresh_sessions`:

```text
token_hash bytea UNIQUE NOT NULL
token_id text UNIQUE NOT NULL
family_id text NOT NULL
user_id uuid NOT NULL REFERENCES users(id)
issued_at timestamptz NOT NULL
expires_at timestamptz NOT NULL
consumed_at timestamptz NULL
revoked_at timestamptz NULL
```

Nunca almacenar el token plano. Indexar `family_id`, `user_id` y `expires_at`.
8. `Rotate` debe abrir transacción, bloquear la fila actual, rechazar tokens ausentes/expirados/revocados/consumidos, marcar el actual consumido e insertar el siguiente. Sólo una rotación concurrente puede confirmar. Si se detecta replay, revocar toda la familia.
9. `RevokeFamily` debe ser idempotente. El logout/revocación nunca debe responder diferente porque la familia ya esté revocada.
10. Construir `services.NewTokenManager`, `services.NewAuthenticationService` y `services.NewAuthorization` mediante inyección explícita. No añadir singletons globales.
11. Validar al arrancar que access/refresh secrets sean distintos y tengan al menos 32 bytes; issuer/audience no vacíos; refresh TTL mayor que access TTL; leeway entre 0 y 120 segundos.
12. Documentar que el despliegue invalida tokens antiguos y requiere volver a iniciar sesión.

## Secuencia de implementación

1. Actualizar module path, versión Go y dependencias; ejecutar `go mod tidy` una vez adaptados los imports.
2. Actualizar tipos de dominio para satisfacer `authBase/v3/core.UserData` sin exponer password.
3. Implementar readers de usuario y credenciales con `context.Context`.
4. Añadir `internal/resources/sql/migrations/000006_create_refresh_sessions.{up,down}.sql` y el store pgx. Si ya existe el número por deriva, usar el siguiente par libre y actualizar los planes 004/007 antes de continuar.
5. Crear el adaptador Gin y tests unitarios del wrapper.
6. Reemplazar constructores y middleware antiguos en controladores/rutas.
7. Sustituir accesos crudos a `c.Get("user")` por el helper v3 después de adaptar el contexto.
8. Adaptar el punto de entrada a `initModules/v2` sin lanzar registro de rutas en goroutines. Si el API v2 no encaja con el diseño explícito, usar su `NewApp`/`Lifecycle` documentado o retirar la dependencia; no conservar una mezcla v1/v2.
9. Añadir tests con fakes para la lógica auth y tests PostgreSQL para concurrencia del refresh store.
10. Ejecutar la matriz completa de verificación.

## Pruebas obligatorias

- Login correcto e incorrecto; credenciales nunca aparecen en respuesta/logs.
- Usuario inactivo no puede iniciar sesión y un access token previo deja de autorizarlo.
- Access token valida issuer, audience, firma, tipo y expiración.
- Un refresh válido rota y retorna un par nuevo.
- Dos refresh concurrentes con el mismo token: exactamente uno confirma.
- Reutilizar el refresh consumido revoca la familia y el descendiente deja de funcionar.
- Logout/revoke es idempotente.
- Secretos iguales/cortos, issuer vacío y TTL inválido impiden arrancar.
- Middleware sin header, esquema incorrecto, token malformado y claims incompletos responden sin panic.

## Comandos de verificación

| Comando | Resultado esperado |
|---|---|
| `GOWORK=off go mod tidy` | Sale 0 y no elimina los módulos v3/v2 requeridos. |
| `GOWORK=off go mod verify` | `all modules verified`. |
| `GOWORK=off go test -count=1 ./...` | Sale 0. |
| `GOWORK=off go test -race -count=1 ./...` | Sale 0, incluida rotación concurrente. |
| `GOWORK=off go vet ./...` | Sale 0. |
| `test -z "$(gofmt -l cmd internal)"` | Sale 0. |
| `rg -n 'github.com/WilsonSayago/authBase(?!/v3)|github.com/WilsonSayago/initModules(?!/v2)|module jwt/base' go.mod cmd internal` | Sin coincidencias; si `rg` no soporta lookaround, hacer tres búsquedas literales equivalentes. |

Para tests de DB, usar `DATABASE_TEST_URL` de una instancia efímera. No usar la base local de desarrollo ni credenciales versionadas.

## Criterios de aceptación

- [ ] Un clon limpio resuelve módulos, compila y ejecuta pruebas sin `replace` a rutas locales.
- [ ] Todos los imports usan el module path público y las versiones mayores correctas.
- [ ] Gin se integra mediante un adaptador local probado, sin cambios en la librería externa.
- [ ] No se almacena refresh token plano y la rotación/replay son atómicos y probados.
- [ ] Usuario inactivo, issuer/audience incorrectos y secrets inválidos son rechazados.
- [ ] Tokens antiguos quedan explícitamente invalidados y documentados.
- [ ] No quedan constructores/singletons de la API antigua.

## Condiciones de parada

- Detenerse si el tag v3/v2 no es accesible o su API real difiere de los contratos descritos; registrar la diferencia y actualizar este plan.
- Detenerse si no hay una base efímera para probar atomicidad. No aprobar el store sólo con mocks.
- Detenerse si el despliegue exige mantener tokens antiguos; esa compatibilidad contradice el modelo de seguridad y requiere una estrategia temporal explícita.
- No usar `replace ../authBase` o `replace ../initModules` como solución final.

## Notas de mantenimiento

- Tratar upgrades de versiones mayores de auth como migraciones de seguridad, con pruebas de replay y revocación.
- Añadir limpieza periódica de sesiones expiradas en un trabajo posterior; no debe bloquear requests.
- Mantener adaptadores web pequeños para poder sustituir Gin sin contaminar el dominio.
