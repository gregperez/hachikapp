# Plan 010 — Remediar observaciones de verificación y cierre

## Objetivo

Consolidar y resolver las observaciones detectadas durante la implementación y
revisión de los planes 001–009. Este plan es el cierre técnico de la secuencia:
ningún hallazgo registrado puede quedar implícitamente aceptado, sin propietario o
sin una prueba de cierre.

- Prioridad: P0/P1 según el hallazgo
- Esfuerzo: M inicial, reajustar si el registro crece
- Riesgo: medio; mezcla correcciones puntuales de seguridad, pruebas y limpieza
- Depende de: planes 001–009 implementados
- Rama sugerida: `codex/010-cierre-observaciones`
- Commit base del registro inicial: `de0954f`

## Regla de ejecución

Este plan se ejecuta al finalizar los planes 001–009, pero los hallazgos P0/P1
registrados aquí bloquean cualquier despliegue a producción antes de su corrección.
Posponer una corrección no equivale a aceptar el riesgo.

## Decisiones del responsable (plantilla greenfield)

Este repositorio se trata como **plantilla greenfield** sin instalaciones
durables gestionadas desde este workspace:

| ID | Decisión | Efecto |
|---|---|---|
| O-002 | No hay inventario de entornos con `000002` aplicada fuera de desarrollo. | Se acepta el historial editado; instalaciones futuras parten del baseline actual. |
| O-018 | Idem: sin instalaciones durables a migrar. | Baseline `000001`–`000005` + ADR 002 se mantienen; no se reescribe historial Git. |
| O-027 | Migración a `log/slog` completada (plan 011). | Logrus eliminado. |
| O-030 | `replace` local de authBase se conserva hasta publicar el módulo con `SetActive`/Gin adapter. | CI/clon aislado sigue bloqueado; documentado en README/CHANGELOG. |

## Registro incremental

### Observaciones de la verificación del plan 001

| ID | Prioridad | Evidencia | Impacto | Estado | Cierre esperado |
|---|---:|---|---|---|---|
| O-001 | P1 | `.gitignore`; `git check-ignore -q .env.staging` devolvió no ignorado | Una configuración local de staging podría añadirse al repositorio. | Resuelto | `.env*` + `!.env.example`; `scripts/secrets-smoke.sh` comprueba la matriz. |
| O-002 | P1 externo | `internal/resources/sql/migrations/000002_insert_users_table.{up,down}.sql` fue modificado | Si `000002` fue aplicada fuera de desarrollo, cambiar su checksum/historial puede bloquear migraciones. | Resuelto (decisión greenfield) | Sin entornos durables; ver sección Decisiones. |
| O-003 | P2 | `.gitleaks.toml` permite por ruta `plans/` y `docs/security-operations.md` | Un secreto nuevo en esas rutas quedaría fuera del escaneo. | Resuelto | Allowlist mínima de ejemplos/placeholders; smoke detecta secreto sintético. |

### Observaciones de la verificación del plan 002

| ID | Prioridad | Evidencia | Impacto | Estado | Cierre esperado |
|---|---:|---|---|---|---|
| O-004 | P1 | `internal/infra/primary/controller/auth_controller.go:156` descarta el error de `RevokeRefreshFamily` | Durante una caída de PostgreSQL el cliente recibe `204`, aunque su familia refresh puede seguir válida. | Resuelto | Logout propaga 5xx; tests en `auth_http_test.go`. |
| O-005 | P1 | `internal/infra/secondary/repositories/postgres/refreshstore/store_test.go:21-24` omite integración sin `DATABASE_TEST_URL` | No había evidencia ejecutada de que la rotación concurrente produjera exactamente un ganador en PostgreSQL. | Resuelto | `make test-db` pasó contra PostgreSQL efímera: migraciones up/down/up y `go test -race` de refreshstore/testdb. |
| O-006 | P1 | Store, adaptador Gin, migración `000007` y pruebas aparecían como archivos no versionados durante la revisión | Un clon limpio no incluiría implementación ni pruebas. | Resuelto | En `7ee6e9b`, los artefactos del baseline quedaron versionados; `git status` sólo muestra este plan 010 aún no agregado. |
| O-007 | P2 | `internal/infra/config/prop/jwt_validate_test.go:26-30` no cubre audience vacío ni leeway fuera de rango; no hay prueba de middleware app para headers Bearer malformados | La validación depende de la librería sin caracterización completa en la integración local. | Resuelto | Casos audience/leeway en `jwt_validate_test.go`; matriz Bearer en `auth_http_test.go`. |

### Observaciones de la verificación del plan 003

| ID | Prioridad | Evidencia | Impacto | Estado | Cierre esperado |
|---|---:|---|---|---|---|
| O-008 | P2 | `.github/workflows/ci.yml:21` usa `postgres:16-alpine` sin digest y `:53-54` descarga Gitleaks mediante una tubería sin checksum | La puerta funciona, pero la imagen y binario de CI no tienen integridad/reproducibilidad criptográficamente verificable. | Resuelto | Postgres fijado por digest; Gitleaks verificado con SHA-256. |

### Observaciones de la verificación del plan 004

| ID | Prioridad | Evidencia | Impacto | Estado | Cierre esperado |
|---|---:|---|---|---|---|
| O-009 | P1 | `internal/core/services/admin_bootstrap.go:28-49` hace `CountAdmins` antes de `Save`; `internal/core/services/user_service.go:160-185,213-239` cuenta admins antes de actualizar, en transacciones separadas | Dos bootstraps concurrentes pueden crear más de un primer admin; con dos admins, dos demociones/desactivaciones simultáneas pueden dejar cero admins activos. | Resuelto | Advisory lock + `FOR UPDATE` en demote/deactivate; tests de concurrencia en `admin_concurrency_test.go`. |
| O-010 | P1 | `internal/resources/sql/migrations/000008_tighten_default_roles.down.sql:4-10` restituye CRUD de `USERS` y `ROLES` para `admin`, `user` y `guest` | Un rollback de migración en un entorno real reabre la escalación de privilegios corregida por el plan. | Resuelto | Down es no-op seguro (`SELECT 1`); no reintroduce grants. |
| O-011 | P2 | No hay test de controlador HTTP: la búsqueda de `httptest`, `StatusForbidden`/`StatusUnauthorized` sólo encontró matriz de dominio y adaptador Gin | Las garantías 401/403 y el rechazo de payloads privilegiados se infieren, pero no se prueban en rutas reales. | Resuelto | Matriz 401/403 + bind `*bool` en `auth_http_test.go`. |

### Observaciones de la verificación del plan 005

| ID | Prioridad | Evidencia | Impacto | Estado | Cierre esperado |
|---|---:|---|---|---|---|
| O-012 | P1 | `internal/infra/config/appconfig/load.go:48-51` acepta `DATABASE_URL`, mientras `internal/infra/config/appconfig/validate.go:90-103` valida TLS y placeholder sólo contra campos separados que quedan en defaults | En producción un `DATABASE_URL` con `sslmode=disable` (o credencial placeholder) pasa la validación y se usa sin cambios por el pool y migrador, eludiendo la política TLS/secrets declarada. | Resuelto | `applyDatabaseURLFields` + validación producción; tests en `config_test.go`. |
| O-013 | P1 | `internal/infra/config/appconfig/load.go:96-121,135-140` descarta errores de `Atoi`, `ParseInt` y `ParseDuration`, y convierte cualquier booleano no reconocido a `false` | Un valor inválido de configuración se acepta silenciosamente y conserva un default o desactiva una opción; contradice el criterio de que todo error de parseo aborta antes de escuchar y puede degradar límites operativos. | Resuelto | Parseo estricto con error por campo; tests de int/bool inválidos. |
| O-014 | P2 | `internal/infra/bootstrap/health_test.go:112-156` cancela un contexto, pero el gate ya está listo antes de iniciar y no se obtiene la dirección del listener ni se emite una señal real | No hay evidencia de que SIGTERM deje el proceso no-ready, rechace nuevas conexiones, termine una petición en vuelo dentro del deadline y libere el puerto, como exige el plan. | Abierto (P2) | Cubierto parcialmente por `TestRunServerCancel`; SIGTERM/in-flight completo queda como mejora post-cierre. |

### Observaciones de la verificación del plan 006

| ID | Prioridad | Evidencia | Impacto | Estado | Cierre esperado |
|---|---:|---|---|---|---|
| O-015 | P1 | `internal/resources/sql/migrations/000005_create_users_roles_table.up.sql:1-17` sólo tiene PK sintética; `internal/infra/secondary/repositories/postgres/dao/user_role_dao.go:32-43` inserta sin conflicto ni deduplicación | La misma relación usuario-rol puede guardarse varias veces, incluso dentro de una sola actualización con IDs repetidos o en peticiones concurrentes; no se cumple el criterio de no crear relaciones duplicadas. | Resuelto | `000009` añade `UNIQUE (user_id, role_id)`; DAO usa `ON CONFLICT DO NOTHING`; `schema_constraints_test.go`. |
| O-016 | P1 | `internal/infra/secondary/repositories/postgres/dao/permission_dao.go:27-35` consulta `public.permissions` y `public.users_permissions`, pero ninguna migración `000001`–`000008` crea dichas tablas | Sobre un esquema creado exclusivamente por las migraciones oficiales, cualquier uso de `PermissionDao.FindAllByUserId` falla por relación inexistente. | Resuelto | Permisos agregados desde JSONB de `roles` vía `users_roles`. |
| O-017 | P2 | `internal/infra/secondary/repositories/postgres/dao/persistence_test.go:33-107` sólo cubre estado, not-found, update de rol y contexto cancelado | No hay prueba de rollback tras fallar una relación, de conservación de errores de count/iteración, de cierre de rows o de la concurrencia exigida; el `make verify` verde no demuestra esos criterios. | Resuelto (parcial) | Unique→`ErrConflict`; concurrencia admin; residual de instrumentación DBTX diferido. |

### Observaciones de la verificación del plan 007

| ID | Prioridad | Evidencia | Impacto | Estado | Cierre esperado |
|---|---:|---|---|---|---|
| O-018 | P1 externo | El commit `554b123` modifica `000001`–`000005`; `docs/adr/002-migration-baseline.md:21-39` reconoce instalaciones durables previas pero propone reconstrucción/re-baseline sin inventario aportado | Las instalaciones existentes conservan `OWNER TO postgres` y downs históricos; un intento operativo de normalizarlas o de bajar versiones puede divergir del baseline nuevo. Se ejecutó la modificación pese a la condición de parada sobre historial aplicado. | Resuelto (decisión greenfield) | Sin instalaciones durables; ver Decisiones. |
| O-019 | P1 | `internal/resources/sql/migrations/000009_harden_identity_schema.up.sql:28-55` escoge un rol duplicado por fecha/id, reasigna algunas relaciones y elimina los demás sin comparar rutas o permisos | Dos roles con igual nombre pueden tener autorizaciones distintas; la migración puede borrar permisos/rutas legítimos y alterar accesos sin una decisión humana, contrario a la condición de detenerse ante datos ambiguos. | Resuelto | Abort si rutas/permisos divergen; `TestHardenUpgradeAbortsDivergentRoleDuplicates`. |
| O-020 | P2 | `internal/test/schema_constraints_test.go:48-151` prueba el esquema ya endurecido; no hay fixture que migre desde versión 8 con datos válidos, nulos o duplicados | `up/down/up` en una DB limpia pasa, pero no demuestra la ruta crítica de upgrade ni la política ante datos inválidos/saneamiento exigida por el plan. | Resuelto | Upgrade 8→9 con fixture divergente en `upgrade_009_test.go`. |

### Observaciones de la verificación del plan 008

| ID | Prioridad | Evidencia | Impacto | Estado | Cierre esperado |
|---|---:|---|---|---|---|
| O-021 | P2 | `internal/infra/secondary/repositories/dto/permissiondto.go` sigue con tags `bson` | No afecta la frontera HTTP (corregido en `controller/dto`), pero confunde el modelo interno. | Resuelto | DTOs secundarios unificados a tags `json`. |
| O-022 | P2 | `O-011` sigue abierta: hay contrato OpenAPI y tests de bind/map, pero no matriz HTTP completa 401/403 por rol | Las garantías de autorización en rutas reales siguen sin evidencia de handler. | Resuelto | Cubierto junto a O-011 en `auth_http_test.go`. |
| O-023 | P2 | Errores HTTP cambian a `application/problem+json` (breaking para clientes del envelope `ResponseDto`) | Consumidores que parseaban `message`/`status` en 4xx/5xx deben adaptarse. | Resuelto | `docs/CHANGELOG.md` documenta el breaking change. |
| O-024 | P1 | `internal/infra/primary/controller/dto/inputs/users/requests.go:28-31,39-42` usa `bool` no puntero y sin validación de presencia para operaciones privilegiadas/de estado | `{}` o un campo omitido se decodifica como `false`; una solicitud incompleta puede desactivar un usuario, un rol o el flag admin en vez de responder 400/422. | Resuelto | `*bool` + `binding:"required"`; tests de `{}`/`false`; OpenAPI regenerado. |
| O-025 | P1 | `internal/infra/secondary/repositories/user_repository.go:93-100` y `role_repository.go:51-61` devuelven conflictos como `fmt.Errorf`; `internal/infra/primary/httpapi/map.go:18-59` no los reconoce | Crear un usuario/rol existente, o una carrera contra la constraint única, acaba como `500 INTERNAL_ERROR` en lugar de `409 CONFLICT`, incumpliendo la taxonomía pública. | Resuelto | `domain.ErrConflict` + map 409 + unique PG `23505`. |
| O-026 | P1 | `git status --short` durante la revisión mostró modificados `cmd`, controladores, docs, Makefile y archivos nuevos de `httpapi`/scripts/pruebas | La implementación no estaba inicialmente versionada, pero quedó incorporada durante la revisión. | Resuelto | `03f1c47` incorpora los artefactos; el árbol de fuente quedó limpio y `make verify` pasó incluyendo OpenAPI. |

### Observaciones de la verificación del plan 009

| ID | Prioridad | Evidencia | Impacto | Estado | Cierre esperado |
|---|---:|---|---|---|---|
| O-027 | P2 | Logrus queda en `v1.9.4`; `go list -m -u` ofrece `v1.10.2` | Se omite un minor deliberadamente (política patch-only + decisión slog). | Resuelto | Migrado a `log/slog` (plan 011); Logrus eliminado de `go.mod`. |
| O-028 | P2 | Gin `v1.12.0` cambió `Context.Get/Set` a claves `any`; adaptador local reexpone `string` | Sin el wrapper, el código no compila contra authBase. El riesgo residual es divergencia futura de la API de Gin. | Resuelto | `ginadapter` implementa `Get/Set(string)` y assertion de interfaz; cubierta por test unitario. |
| O-029 | P1 | `GOTOOLCHAIN=local ... make verify` con Go `1.26.3` falla en `govulncheck`: diez vulnerabilidades alcanzables de la biblioteca estándar, con fixes desde `1.26.4` a `1.26.6` | La versión mínima configurada y su job CI no cumplen la condición de no aceptar vulnerabilidades alcanzables; afecta rutas HTTP/TLS y carga de configuración. | Resuelto | `go.mod` + CI en `1.26.6`. |
| O-030 | P1 | `go.mod:24` conserva `replace github.com/WilsonSayago/authBase/v3 => ../authBase`, mientras CI sólo hace checkout del repositorio actual | El build depende de un directorio hermano local que no existe en un checkout aislado de GitHub Actions o de consumidores; la actualización no es reproducible fuera de este workspace. | Bloqueado externo | Publicar authBase versionado; ver Decisiones. Hasta entonces `replace` es requerido. |

### Plantilla para observaciones de los planes 005–009

Agregar una fila por hallazgo con este formato. No borrar entradas resueltas; cambiar
el estado y enlazar la evidencia de cierre.

| ID | Prioridad | Evidencia | Impacto | Estado | Cierre esperado |
|---|---:|---|---|---|---|
| O-XXX | P0/P1/P2 | `archivo:línea` o comando reproducible | Consecuencia concreta | Abierto / Bloqueado externo / Resuelto | Cambio mínimo y prueba obligatoria |

## Control de deriva

Antes de ejecutar el cierre:

```bash
git rev-parse --short HEAD
git status --short
sed -n '1,280p' plans/010-remediar-observaciones-y-cierre.md
git log --oneline --all -- internal/infra/primary/controller/auth_controller.go internal/infra/secondary/repositories/postgres/refreshstore internal/resources/sql/migrations
```

Para cada fila abierta, abrir el archivo citado y repetir el comando/evidencia
original. Si el hallazgo ya fue corregido por otro plan, cambiarlo a `Resuelto` y
registrar el commit y la prueba que lo confirma.

## Alcance

- Resolver todas las filas abiertas del registro incremental.
- Ejecutar pruebas de integración PostgreSQL en una instancia efímera aislada.
- Versionar los artefactos necesarios y validar un clon/worktree limpio.
- Ajustar `.gitignore` y Gitleaks para que las excepciones sean estrechas.
- Añadir las pruebas de seguridad/errores faltantes en auth y cierre de sesión.
- Actualizar el estado de cada observación y de los planes de origen con evidencia.

## Fuera de alcance

- Añadir funcionalidades de producto no relacionadas con un hallazgo registrado.
- Rotar secretos o modificar bases de datos reales sin autorización explícita.
- Reescribir historial Git; sigue siendo una operación coordinada y destructiva.
- Reabrir refactors cerrados de los planes 001–009 salvo que una observación lo exija.

## Secuencia de implementación

1. Revisar el registro completo y clasificar cada fila: corregible en repo, bloqueada por estado externo o ya resuelta.
2. Resolver primero P0/P1. Para O-004, no convertir una caída de DB en logout exitoso; la idempotencia se aplica a estado de familia, no a errores de infraestructura.
3. Si O-005 se reabre, levantar o recibir una PostgreSQL efímera mediante `DATABASE_TEST_URL`. No usar desarrollo ni producción. Ejecutar concurrencia repetida con `-count=20` y `-race`.
4. Si O-006 se reabre, añadir todos los archivos pertinentes en un commit atómico y verificar desde un worktree/clon sin cachés ni paths locales. No usar `replace`.
5. Para O-001 y O-003, endurecer las reglas y añadir pruebas/smoke checks que demuestren tanto el caso permitido (`.env.example`) como los rechazados (`.env.staging`, secreto sintético fuera de allowlist).
6. Para O-002, pedir al responsable el inventario de entornos antes de tocar migraciones ya publicadas. Si hay una instalación afectada, diseñar una migración forward o baseline; no alterar checksums existentes.
7. Para O-007, incorporar tests de configuración y handlers. Afirmar status/cuerpo seguros sin comparar secretos ni tokens completos.
8. Repetir la matriz de verificación, actualizar cada fila a `Resuelto` con commit/prueba, y sólo entonces marcar este plan como hecho.

## Pruebas y comandos obligatorios

| Comando | Resultado esperado |
|---|---|
| `git check-ignore -v .env .env.staging .env.production` | Las tres configuraciones reales quedan ignoradas. |
| `git check-ignore -q .env.example; test $? -ne 0` | El ejemplo permanece versionable. |
| `make check-secrets` | Sale 0 sin allowlists amplias que oculten archivos de documentación/planes. |
| `DATABASE_TEST_URL='<dsn-efimero>' GOWORK=off go test -race -count=20 ./internal/infra/secondary/repositories/postgres/refreshstore` | Sale 0; prueba exactamente un ganador y revocación por replay. |
| `GOWORK=off go test -race -count=1 ./...` | Sale 0. |
| `GOWORK=off go vet ./...` | Sale 0. |
| `test -z "$(gofmt -l cmd internal)"` | Sale 0. |
| `git diff --check` | Sale 0. |
| `git status --short` | Vacío tras el commit de cierre. |

## Criterios de aceptación

- [x] Ninguna fila P0/P1 permanece abierta o bloqueada sin decisión explícita del responsable.
- [x] Logout diferencia un fallo de infraestructura de una revocación idempotente.
- [x] La rotación PostgreSQL se probó realmente bajo concurrencia y detector de carreras.
- [ ] Todas las implementaciones y pruebas requeridas están versionadas y funcionan desde un checkout limpio. *(bloqueado por O-030 / `replace` authBase)*
- [x] Reglas de secretos cubren variantes locales y no excluyen directorios de documentación completos.
- [x] Los tests de auth cubren las configuraciones y entradas HTTP registradas.
- [x] Cada observación tiene estado final, evidencia de cierre y no se eliminaron registros históricos.

## Condiciones de parada

- Detenerse si no se puede garantizar que `DATABASE_TEST_URL` es efímera; no ejecutar pruebas destructivas contra una DB compartida.
- Detenerse ante una migración histórica aplicada sin inventario de entornos; requerir decisión humana.
- Detenerse si resolver un hallazgo exige ampliar producto, cambiar contratos públicos o incorporar una versión major; crear un plan nuevo y referenciarlo.
- No marcar este plan como completado porque las pruebas unitarias pasen si queda una fila P0/P1 sin cierre comprobable.

## Notas de mantenimiento

- Cada revisión posterior a un plan debe añadir sus hallazgos aquí antes de cerrar la tarea.
- Mantener la prioridad basada en impacto verificable, no en antigüedad de la observación.
- Programar una revisión de este registro antes de cada release hasta que quede vacío de P0/P1.
- Residual abierto: O-014 (P2 lifecycle SIGTERM), O-030 (publicar authBase).
