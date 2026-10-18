# Plan 007 — Endurecer esquema y migraciones

## Objetivo

Alinear constraints, índices, ownership y reversibilidad del esquema con los invariantes del dominio, y retirar SQL obsoleto que puede inicializar una base incompatible.

- Prioridad: P1
- Esfuerzo: M
- Depende de: planes 003, 004 y 006
- Rama sugerida: `codex/007-esquema-migraciones`
- Commit analizado: `99a9826`

## Estado actual verificado

- Los down de las migraciones `000001` a `000005` están vacíos.
- `users_roles` permite claves foráneas nulas, no tiene unicidad `(user_id, role_id)` ni índice útil por usuario.
- `roles.name` no es único.
- El SQL asigna ownership a `postgres`, acoplando despliegues a un rol concreto.
- Los seeds de permisos son demasiado amplios; el plan 004 corrige su semántica.
- `internal/resources/init.sql` representa otro dominio, duplica nombres de tablas como usuarios/permisos y puede inicializar un esquema divergente.

Por ejemplo, la tabla junction se crea actualmente sin constraints de presencia o unicidad:

```sql
user_id uuid references users on delete cascade,
role_id uuid references roles on delete cascade
```

Y cada archivo `*.down.sql` reportó cero líneas en la inspección del commit analizado.

## Control de deriva

```bash
git rev-parse --short HEAD
git status --short
find internal/resources/sql/migrations -maxdepth 1 -type f -print | sort
wc -l internal/resources/sql/migrations/*.down.sql
rg -n 'OWNER TO|CREATE TABLE|UNIQUE|users_roles|CREATE INDEX' internal/resources/sql/migrations internal/resources/init.sql
```

Antes de modificar migraciones existentes, determinar si han sido publicadas/aplicadas y si el migrador valida checksums.

## Alcance

- Crear migraciones forward nuevas para constraints/índices en instalaciones existentes.
- Definir estrategia de baseline limpio para instalaciones nuevas sin reescribir historia a ciegas.
- Down migrations seguras para los cambios nuevos.
- Eliminar o mover fuera del runtime `internal/resources/init.sql`.
- Tests up/down/up y tests de constraints/concurrencia.
- Índices para relaciones, lookup de login y refresh sessions.

## Fuera de alcance

- Ejecutar migraciones en entornos reales.
- Inventar políticas de retención/particionado sin métricas.
- Modificar permisos RBAC; plan 004 produce su migración semántica.

## Invariantes objetivo

- `roles.name` es `NOT NULL` y único con normalización/case policy documentada.
- `users_roles.user_id` y `role_id` son `NOT NULL`, FKs con acciones explícitas y par único.
- Tablas junction equivalentes de roles/permisos/rutas tienen pares únicos e índices por ambos sentidos cuando las consultas lo exigen.
- Username/email usados para login tienen unicidad coherente con su normalización.
- Refresh sessions cumplen los índices/constraints del plan 002.
- Ningún objeto depende de que exista el rol PostgreSQL literal `postgres`.

## Implementación

1. Inventariar datos que violan los nuevos constraints con queries de sólo lectura en una copia/DB efímera: nulls, duplicados, nombres duplicados por case y FKs huérfanas.
2. Definir política de saneamiento determinista. En producción, emitir un reporte y parar si eliminar/combinar datos requiere decisión humana.
3. Añadir `internal/resources/sql/migrations/000009_harden_identity_schema.{up,down}.sql`, no editar una aplicada, que (si el número quedó ocupado, seleccionar el siguiente par libre y documentarlo):
   - limpie sólo casos inequívocos autorizados;
   - aplique `NOT NULL` y FKs explícitas;
   - añada unique constraints;
   - cree índices con nombres estables;
   - quite/reasigne ownership hardcodeado sólo mediante un mecanismo portable.
4. Para tablas grandes, planear `CREATE INDEX CONCURRENTLY` fuera de transacción y `NOT VALID` + `VALIDATE CONSTRAINT`; documentar compatibilidad con el migrador. En la base pequeña de plantilla puede usarse migración normal, pero no afirmar que es online.
5. Crear down que retire sólo objetos añadidos por esta migración. No recrear duplicados o nulls eliminados.
6. Tratar downs históricos vacíos con dos modos:
   - si nunca fueron publicados, completarlos antes de release;
   - si ya se aplicaron/checksummed, no tocarlos; documentar que son irreversibles y usar un baseline nuevo para instalaciones frescas.
7. Eliminar `init.sql` del flujo y del repositorio si no pertenece al producto. Si debe conservarse como referencia, moverlo a `docs/archive/` con advertencia no ejecutable; preferencia: eliminarlo.
8. Añadir test que enumere migraciones, aplique hasta latest, baje a cero cuando sea soportado y suba otra vez; comparar esquema esencial.

## Pruebas obligatorias

- Insertar rol duplicado falla con constraint esperado.
- Insertar relación nula/duplicada/huérfana falla.
- Borrado de usuario/rol sigue la política `RESTRICT` o `CASCADE` documentada.
- Queries de rutas críticas usan índice mediante `EXPLAIN` en fixtures representativos; no fijar costos exactos.
- Up/down/up desde DB vacía funciona.
- Upgrade desde el esquema anterior con datos válidos funciona.
- Datos inválidos provocan error claro o saneamiento explícitamente aprobado.

## Comandos de verificación

```bash
DATABASE_TEST_URL='<dsn-efimero>' make migrations-test
DATABASE_TEST_URL='<dsn-efimero>' make test-db
make test-race
make verify
rg -n 'OWNER TO postgres' internal/resources/sql/migrations
test ! -f internal/resources/init.sql
```

La búsqueda de ownership no devuelve coincidencias en el baseline ejecutable y `init.sql` ya no existe en su ubicación peligrosa.

## Criterios de aceptación

- [x] El esquema impide duplicados, nulls y FKs huérfanas en relaciones críticas.
- [x] Existen índices acordes a los paths de usuario/rol/refresh.
- [x] La estrategia para migraciones históricas aplicadas está documentada sin alterar checksums silenciosamente.
- [x] Cada migración nueva tiene down seguro y tests up/down/up.
- [x] No se requiere un owner PostgreSQL fijo.
- [x] `init.sql` obsoleto no puede ejecutarse accidentalmente.

## Condiciones de parada

- Detenerse ante duplicados/huérfanos ambiguos en datos reales; entregar reporte cuantitativo sin PII y solicitar criterio.
- Detenerse si una migración online requiere lock prolongado no medido.
- No editar migraciones aplicadas hasta confirmar política de checksums y despliegue.

## Notas de mantenimiento

- Toda regla de dominio estable debe existir también como constraint cuando PostgreSQL pueda imponerla.
- Revisar índices con planes reales y volumen; eliminar índices redundantes sólo con evidencia.
- Mantener una única vía oficial de inicialización: las migraciones.
