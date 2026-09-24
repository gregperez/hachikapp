# Plan 006 — Corregir límites de persistencia PostgreSQL

## Objetivo

Hacer que todas las operaciones PostgreSQL respeten cancelación, cierren recursos, conserven errores, distingan “sin filas” de éxito y ejecuten escrituras compuestas de forma transaccional e idempotente.

- Prioridad: P1
- Esfuerzo: L
- Depende de: planes 003 y 005
- Rama sugerida: `codex/006-persistencia-postgresql`
- Commit analizado: `99a9826`

## Estado actual verificado

Las interfaces de persistencia omiten `context.Context` y la implementación conserva/usa contexto de fondo. Hay iteraciones `Query` sin `defer rows.Close()` ni `rows.Err()` en:

- `internal/infra/secondary/repositories/postgres/dao/user_dao.go`
- `internal/infra/secondary/repositories/postgres/dao/role_dao.go`
- `internal/infra/secondary/repositories/postgres/dao/permission_dao.go`

Los wrappers de `Exec` descartan `pgconn.CommandTag`, por lo que un update de cero filas parece exitoso. Además:

```go
count, _ := repository.Count(...)
users, err := repository.FindAll(...)
```

sobrescribe/ignora el error del conteo en repositorios de usuarios y roles. `UpdateRole` no persiste sus rutas, y el cambio de estado usa un toggle `active = NOT active`, que no es idempotente.

## Control de deriva

```bash
git rev-parse --short HEAD
git status --short
rg -n 'context.Background|Query\(|QueryRow\(|Exec\(|rows.Next|rows.Err|rows.Close|active = NOT|Count\(' internal/infra/secondary
sed -n '1,280p' internal/infra/secondary/repositories/postgres/dao/user_dao.go
sed -n '1,280p' internal/infra/secondary/repositories/postgres/dao/role_dao.go
```

## Diseño objetivo

- Cada puerto de repositorio/DAO recibe `context.Context` como primer argumento.
- Definir una interfaz mínima `DBTX` implementada por `*pgxpool.Pool` y `pgx.Tx`, con `Exec`, `Query`, `QueryRow` contextuales.
- Los casos de uso reciben contexto del request; no crean `context.Background()`.
- Los métodos de escritura retornan error semántico `not found` cuando `RowsAffected()==0`.
- Escrituras de usuario + roles y rol + permisos/rutas ocurren en una transacción con rollback seguro.
- Las operaciones expresan estado deseado: `SetActive(ctx,id,bool)`, no toggle.

## Alcance

- Puertos, DAOs, repositorios y casos de uso afectados por contexto.
- Cierre/errores de rows y conservación de todos los errores.
- CommandTag y not-found.
- Transacciones de escrituras compuestas.
- Persistencia completa de roles, permisos y rutas.
- Estado explícito/idempotente.
- Tests unitarios e integración, incluidos fallos parciales y cancelación.

## Fuera de alcance

- Cambiar tablas/constraints salvo lo imprescindible; plan 007.
- Rediseñar respuestas HTTP; plan 008.
- Introducir ORM o generador SQL.

## Implementación

1. Cambiar las interfaces desde los casos de uso hacia abajo para aceptar `ctx`. Propagar `c.Request.Context()` desde los controladores mediante la abstracción establecida en plan 002.
2. Crear `DBTX`; hacer que cada DAO acepte la interfaz por operación o estructura. Evitar que un repositorio abra transacciones invisibles dentro de otro repositorio.
3. Para cada `Query`:

```go
rows, err := db.Query(ctx, query, args...)
if err != nil { return ..., fmt.Errorf("query users: %w", err) }
defer rows.Close()
for rows.Next() { /* Scan y comprobar error */ }
if err := rows.Err(); err != nil { return ..., fmt.Errorf("iterate users: %w", err) }
```

4. Para `QueryRow`, mapear sólo `pgx.ErrNoRows` al error de dominio not-found; envolver el resto conservando `errors.Is`.
5. Hacer que `Exec` devuelva/verifique `CommandTag`. Updates/deletes por ID esperan una fila; cero es not-found. Inserts de relaciones pueden usar `ON CONFLICT` sólo cuando la semántica sea explícitamente idempotente.
6. Corregir la secuencia count/list: si count falla, retornar inmediatamente; no ejecutar ni devolver resultados parciales.
7. Reescribir create/update de usuario y rol como unidad transaccional. Usar `defer tx.Rollback(ctx)` y `Commit` al final; el rollback posterior a commit puede ignorar `pgx.ErrTxClosed`.
8. En `UpdateRole`, sincronizar nombre, permisos y rutas. Calcular estado deseado o borrar/reinsertar relaciones dentro de la misma transacción, protegido por constraints del plan 007.
9. Reemplazar cambio de estado por `SetActive(id, active)`. Dos requests iguales producen el mismo resultado.
10. Establecer límites/orden estable en queries paginadas; el cap HTTP se completa en plan 008.
11. Envolver errores con operación/entidad, nunca SQL completo, argumentos, password/hash o DSN.

## Pruebas obligatorias

- Contexto cancelado interrumpe query y retorna error compatible con `context.Canceled`/deadline.
- Error al abrir, escanear o iterar rows se conserva.
- Rows se cierran en éxito y error.
- Update/delete de ID ausente devuelve not-found.
- Error de count no se pierde y no retorna lista parcial.
- Fallo al insertar una relación revierte la entidad principal.
- `UpdateRole` persiste altas/bajas de permisos y rutas.
- Dos `SetActive(true)` consecutivos dejan activo; no alternan.
- Operaciones concurrentes no crean relaciones duplicadas.

## Comandos de verificación

```bash
make fmt-check
make lint
make test
DATABASE_TEST_URL='<dsn-efimero>' make test-db
make test-race
rg -n 'context.Background\(' internal/infra/secondary internal/application
rg -n 'active[[:space:]]*=[[:space:]]*NOT' internal
```

Las dos búsquedas finales no deben devolver coincidencias productivas. `make verify` debe salir 0.

## Criterios de aceptación

- [x] Todas las operaciones DB reciben contexto del caller.
- [x] Todo `Query` cierra rows y comprueba `rows.Err()`.
- [x] Ningún error de DB relevante se ignora o sobrescribe.
- [x] Cero filas afectadas tiene semántica not-found.
- [x] Escrituras compuestas son atómicas y están probadas con fallo intermedio.
- [x] `UpdateRole` persiste todo su agregado.
- [x] Cambios de estado son idempotentes.

## Condiciones de parada

- Detenerse si una interfaz pública es consumida fuera del repositorio sin estrategia de migración; documentar el breaking change y versionar.
- Detenerse si una transacción necesita abarcar una llamada externa; rediseñar el límite, no mantener locks durante I/O remoto.
- No aprobar mocks sin los tests de integración PostgreSQL correspondientes.

## Notas de mantenimiento

- Toda query nueva debe tener test de error/cancelación además del happy path.
- Mantener SQL visible y revisable; si crece, evaluar `sqlc` en un plan separado.
- Medir pool/queries antes de optimizar; no agregar caché para ocultar errores de acceso.
