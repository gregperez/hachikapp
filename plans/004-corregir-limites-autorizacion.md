# Plan 004 — Corregir límites de autorización

## Objetivo

Eliminar escalación de privilegios por rutas, DTOs y seeds; separar administración de identidad de la edición ordinaria de usuarios y aplicar mínimo privilegio con respuestas 401/403 consistentes.

- Prioridad: P0
- Esfuerzo: M
- Depende de: planes 002 y 003
- Rama sugerida: `codex/004-autorizacion`
- Commit analizado: `99a9826`

## Estado actual verificado

Las rutas de roles están protegidas con la entidad equivocada en `internal/infra/primary/controller/role_controller.go`:

```go
authorizationHandler.HasPermission("USERS", "CREATE", ...)
authorizationHandler.HasPermission("USERS", "UPDATE", ...)
authorizationHandler.HasPermission("USERS", "DELETE", ...)
```

Los payloads de creación/actualización de usuario permiten asignar campos privilegiados desde el mismo endpoint general en `user_controller.go`:

```go
IsAdmin: userData.IsAdmin,
Roles:   userData.Roles,
```

El seed `internal/resources/sql/migrations/000004_insert_roles_table.up.sql` concede CRUD de `USERS` y `ROLES` no sólo al rol administrador, sino también a roles de usuario e invitado.

## Control de deriva

```bash
git rev-parse --short HEAD
git status --short
rg -n 'HasPermission|IsAdmin|Roles:' internal/infra/primary/controller internal/infra/primary/dto
sed -n '1,180p' internal/resources/sql/migrations/000004_insert_roles_table.up.sql
```

No continuar si los nombres de entidades/acciones cambiaron sin actualizar la matriz de acceso.

## Matriz objetivo mínima

| Operación | Invitado | Usuario | Administrador |
|---|---:|---:|---:|
| Leer/editar su perfil no privilegiado | No / según producto | Sí, propio | Sí |
| Listar/crear/editar/desactivar usuarios | No | No | Sí |
| Cambiar `is_admin` o roles | No | No | Sí, endpoint explícito |
| CRUD de roles/permisos | No | No | Sí |

Si el producto necesita delegación adicional, crear permisos específicos (`USERS_ASSIGN_ROLES`, por ejemplo) y documentarlos; no reutilizar un `UPDATE` genérico.

## Alcance

- Corregir rutas de roles para autorizar sobre `ROLES`.
- Separar DTOs/endpoints de perfil, administración de usuario y asignación de privilegios.
- Impedir mass assignment de `is_admin`, roles, estado y password en endpoints generales.
- Rediseñar seeds de permisos con mínimo privilegio mediante una migración nueva.
- Añadir bootstrap administrativo seguro definido en plan 001, si aún falta.
- Aplicar 401 a identidad ausente/inválida y 403 a identidad válida sin permiso.
- Tests de matriz y regresión de escalación.

## Fuera de alcance

- Rediseñar el motor genérico de permisos de `authBase`.
- Crear roles específicos de una aplicación consumidora; esta base sólo incluye defaults conservadores.
- Corregir el mapeo global de todos los errores HTTP; plan 008.

## Implementación

1. Definir constantes tipadas para entidades/acciones o un catálogo único. Eliminar strings duplicados en controladores.
2. Cambiar todos los guards de endpoints de roles a entidad `ROLES` y acción correspondiente.
3. Crear DTOs separados:
   - `UpdateOwnProfileRequest`: sólo campos personales permitidos.
   - `AdminUpdateUserRequest`: estado/campos administrativos, excluyendo password.
   - `AssignUserRolesRequest`: lista de IDs de rol.
   - cambio de password separado, con verificación apropiada.
4. No mapear `IsAdmin` ni `Roles` desde requests ordinarios. La capa de caso de uso también debe imponer la regla; no confiar sólo en routing.
5. Preferir derivar capacidad administrativa de roles/permisos. Si `is_admin` se conserva por compatibilidad, sólo un flujo administrativo explícito puede modificarlo y cada cambio debe ser auditable.
6. Añadir `internal/resources/sql/migrations/000007_tighten_default_roles.{up,down}.sql` para retirar permisos amplios de `user`/`guest` y asegurar permisos administrativos sólo al rol de administración. Si el número ya está ocupado, usar el siguiente par libre y actualizar el plan 007. La migración debe ser idempotente por claves naturales y tener down documentado.
7. Implementar la utilidad de bootstrap de un solo uso: falla si existe admin, exige password robusto, no registra entrada y ejecuta todo en una transacción.
8. Registrar eventos de asignación/revocación con actor, sujeto y IDs, sin PII innecesaria ni tokens.

## Pruebas obligatorias

- Invitado y usuario ordinario reciben 403 en todo CRUD de roles.
- Usuario con `USERS UPDATE` no puede enviar `isAdmin`, roles, estado ni campos desconocidos para elevarse.
- Sólo el permiso específico/admin puede asignar o retirar roles.
- Un administrador no puede eliminar/desactivar accidentalmente al último administrador; si esta protección se implementa, probar concurrencia transaccional.
- Identidad inexistente/inactiva no autoriza aunque el token tenga claims antiguos.
- 401 vs 403 correcto.
- Seeds nuevos no otorgan CRUD administrativo a `user`/`guest`.

## Comandos de verificación

```bash
make fmt-check
make lint
make test
DATABASE_TEST_URL='<dsn-efimero>' make test-db
make test-race
rg -n '"USERS"' internal/infra/primary/controller/role_controller.go
```

La última orden no debe devolver coincidencias. `make verify` debe salir 0.

## Criterios de aceptación

- [ ] Cada ruta usa la entidad/acción correcta y la matriz está cubierta por tests table-driven.
- [ ] Ningún endpoint general permite mass assignment de privilegios.
- [ ] Los defaults de `user` e `guest` no administran usuarios, roles o permisos.
- [ ] Cambios privilegiados pasan por un caso de uso explícito y auditable.
- [ ] Bootstrap administrativo no crea credenciales conocidas y no puede repetirse.
- [ ] 401 y 403 tienen semántica consistente.

## Condiciones de parada

- Detenerse si no existe una decisión de producto sobre qué puede editar un usuario de sí mismo; bloquear sólo esos campos ambiguos y continuar con el cierre de privilegios obvio.
- Detenerse antes de revocar permisos en una base real: entregar la migración probada, pero el rollout requiere inventario de consumidores.
- No conservar compatibilidad con payloads que permitían escalar privilegios.

## Notas de mantenimiento

- Cada endpoint nuevo debe declarar entidad/acción y añadir una fila de test a la matriz.
- Revisar permisos seed como código sensible en cada release.
- Evitar flags administrativos paralelos que puedan divergir del RBAC.
