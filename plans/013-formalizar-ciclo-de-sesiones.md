# Plan 013: Formalizar emisión, revocación y limpieza de sesiones

> **Instrucciones para el agente ejecutor**: Este plan modifica primero authBase
> y luego base-golang. Mantenga compatibilidad fuente con consumidores v4: no
> agregue métodos obligatorios a interfaces existentes. Ejecute cada gate y
> actualice la fila 013 en `plans/README.md`.
>
> **Control de deriva**:
>
> ```bash
> git -C /Users/wilsonsayago/workspace/golang/authBase diff --stat b5c0a2c..HEAD -- core/authenticationusecase.go core/port/refresh_token_port.go core/services/authenticationservice.go core/services/refresh_token_test.go docs
> git -C /Users/wilsonsayago/workspace/golang/base-golang diff --stat 59aeb1f..HEAD -- internal/core internal/infra/secondary/repositories/postgres/refreshstore internal/resources/sql/migrations
> ```

## Estado

- **Estado**: Verificado (`authBase/v4.1.0`; revocación atómica y purger)
- **Prioridad**: P0
- **Esfuerzo**: L
- **Riesgo**: MED
- **Depende de**: plan 012
- **Categoría**: security / correctness
- **Planeado en**: `base-golang@59aeb1f`, `authBase@b5c0a2c`, 2026-09-25

## Por qué importa

Cambiar una contraseña no revoca refresh tokens existentes. Una cuenta
desactivada rechaza refresh mientras está inactiva, pero una sesión anterior
puede volver a ser útil si se reactiva. Además, la tabla de sesiones sólo crece.
El ciclo debe permitir revocar por usuario, limpiar expiradas y reutilizar una
única frontera segura de emisión para futuros métodos OIDC/WebAuthn.

## Estado actual

- `authBase/core/port/refresh_token_port.go` sólo exige `Create`, `Rotate` y
  `RevokeFamily`.
- `authBase/core/services/authenticationservice.go:71-93` valida password,
  genera el par y persiste la sesión dentro de `Login`.
- `base-golang/internal/core/services/user_service.go:193-215` termina con
  `UpdatePassword`; no revoca sesiones.
- `base-golang/internal/infra/secondary/repositories/postgres/refreshstore/store.go`
  sólo revoca por familia.
- `000007_create_refresh_sessions.up.sql` ya posee índices por `family_id`,
  `user_id` y `expires_at`; no hay que reescribir esa migración histórica.
- Convención: errores públicos son sentinels de `authBase/core/errors.go`; los
  adaptadores envuelven errores internos con contexto sin incluir tokens.

## Comandos necesarios

| Propósito | Comando | Resultado esperado |
|---|---|---|
| Auth | `cd /Users/wilsonsayago/workspace/golang/authBase && GOWORK=off make verify` | Sale 0 |
| Base unit/race | `cd /Users/wilsonsayago/workspace/golang/base-golang && GOWORK=off go test -race -count=1 ./...` | Sale 0 |
| Base DB | `cd /Users/wilsonsayago/workspace/golang/base-golang && DATABASE_TEST_URL='<dsn-efimero>' GOWORK=off make test-db` | Sale 0 |
| Formato | `test -z "$(gofmt -l core infra)"` en authBase y `test -z "$(gofmt -l cmd internal)"` en base | Sin salida |

## Alcance

**Dentro de alcance**:

- authBase: nuevos contratos aditivos de sesión, refactor interno de emisión,
  pruebas y documentación.
- base-golang: refresh store, puertos/repositorios de usuario, servicio de
  cambio de contraseña/desactivación, migración nueva sólo si es necesaria,
  tests unitarios y PostgreSQL.

**Fuera de alcance**:

- OIDC, Google, MFA o passkeys.
- Revocación inmediata de access JWT mediante denylist.
- Reescribir migraciones `000001`–`000012`.
- Añadir métodos a `RefreshTokenStore` o `AuthenticationUseCase` que rompan mocks
  y adaptadores existentes.
- Ejecutar limpieza destructiva contra una DB compartida.

## Flujo Git

- authBase: `codex/013-session-lifecycle`.
- base-golang: `codex/013-session-lifecycle`.
- Publicar primero una versión patch/minor compatible de authBase; actualizar
  base-golang después. No publicar sin autorización.

## Pasos

### Paso 1: añadir contratos aditivos en authBase

En `core/port/refresh_token_port.go`, conserve intacta `RefreshTokenStore` y
añada interfaces de capacidad separadas:

```go
type UserSessionRevoker interface {
    RevokeUser(ctx context.Context, userID string) error
}

type ExpiredSessionPurger interface {
    PurgeExpired(ctx context.Context, before time.Time) (int64, error)
}
```

En `core`, defina interfaces aditivas `SessionIssuer` y `SessionManager`; no
modifique `AuthenticationUseCase`. La implementación concreta debe exponer:

- `EstablishSession(ctx, userID)` para una identidad ya autenticada.
- `RevokeUserSessions(ctx, userID)`.

`EstablishSession` debe cargar el usuario, exigir que esté activo, emitir el par
y persistir el refresh antes de devolver tokens. Documente que es una frontera
privilegiada: el caller debe haber autenticado al sujeto.

**Verificar**: tests de compilación que demuestren que un store v4 antiguo que
sólo implementa `RefreshTokenStore` todavía compila.

### Paso 2: hacer que Login reutilice la frontera de emisión

Después de validar credenciales, `Login` debe llamar un helper único compartido
con `EstablishSession`; no duplique generación/persistencia. Conserve la firma y
errores actuales de `Login`.

**Verificar**: tests existentes de login/refresh pasan y un fallo de `Create`
nunca devuelve tokens.

### Paso 3: implementar capacidades PostgreSQL

En `refreshstore.Store` implemente:

- `RevokeUser`: `UPDATE refresh_sessions SET revoked_at=... WHERE user_id=$1 AND revoked_at IS NULL`.
- `PurgeExpired`: borrar únicamente sesiones con `expires_at < before`; conserve
  un período de retención configurable si se usan registros para forensia.

Valide contexto e IDs vacíos. No registre hashes ni JWT.

**Verificar**: tests PostgreSQL cubren idempotencia, dos familias de un mismo
usuario, aislamiento entre usuarios y límite exacto de limpieza.

### Paso 4: revocar atómicamente en eventos sensibles

Amplíe el puerto/repositorio local de usuarios con operaciones transaccionales:

- cambiar hash de password y revocar todas las sesiones del usuario;
- desactivar usuario y revocar todas sus sesiones.

Ambas escrituras deben usar una sola transacción PostgreSQL. La promoción,
democión y activación sin cambio a `false` no deben revocar por accidente.

**Verificar**: fuerce un error entre ambas escrituras y confirme rollback total;
en éxito, todas las familias quedan revocadas.

### Paso 5: exponer revocación global controlada

Añada al servicio/aplicación una operación interna o endpoint administrativo
sólo si ya existe una política apropiada. No cree un endpoint público sin
autorización. Como mínimo, cambio de contraseña y desactivación deben usarla.
Documente respuesta operativa ante compromiso.

### Paso 6: preparar limpieza como Lifecycle

Implemente un componente sin globales que ejecute limpieza al iniciar y luego a
intervalo configurable, responda a cancelación y limite cada batch. Si el plan
015 aún no está implementado, déjelo construible y probado pero registre su
integración final como dependencia explícita del plan 015.

## Plan de pruebas

- authBase: `EstablishSession` activo/inactivo/no encontrado/Create fallido;
  compatibilidad de interfaces; revocador no soportado devuelve error explícito.
- base unit: cambio de password y desactivación solicitan revocación; activación
  no lo hace; errores se propagan.
- base DB: atomicidad, idempotencia, aislamiento por usuario, purge por cutoff y
  carrera entre refresh y revocación.
- Modele tests DB sobre `refreshstore/store_test.go` y transacciones sobre
  `dao/admin_concurrency_test.go`.

## Criterios de terminado

- [x] Consumidores que implementan el viejo `RefreshTokenStore` siguen compilando.
- [x] Existe una única ruta interna de emisión inicial y persiste antes de retornar.
- [x] Cambio de contraseña revoca sesiones en la misma transacción.
- [x] Desactivar un usuario revoca sesiones en la misma transacción.
- [x] `RevokeUser` y `PurgeExpired` tienen tests PostgreSQL con race detector.
- [x] Ninguna prueba/log contiene tokens o hashes completos.
- [x] Gates de ambos repositorios pasan.

## Condiciones de parada

- La única solución parece exigir romper interfaces públicas v4.
- La DB de pruebas no es efímera.
- Se descubre una instalación durable donde modificar migraciones históricas
  sería necesario; cree una migración forward o reporte.
- No puede garantizar atomicidad entre password/estado y revocación.
- La retención legal/forense de sesiones no está definida y se pretende borrar
  datos de producción automáticamente.

## Notas de mantenimiento

- `EstablishSession` será la base del plan 016; revise que no permita omitir la
  validación de usuario activo ni la persistencia refresh.
- Si más adelante se añade denylist de access tokens, manténgala separada del
  contrato de refresh.
- El plan 015 debe registrar el purger mediante `initModules.App`, no goroutines
  huérfanas desde constructores.
