# Plan 001 — Contener secretos y datos de arranque

## Objetivo

Eliminar credenciales, claves JWT y cuentas privilegiadas reproducibles del contenido versionado. Dejar un flujo de configuración local documentado y una guía operativa de rotación sin ejecutar cambios en sistemas externos.

- Prioridad: P0
- Esfuerzo: M
- Riesgo: alto; una eliminación incompleta mantiene exposición y una edición incorrecta de migraciones puede romper instalaciones existentes.
- Rama sugerida: `codex/001-contener-secretos`
- Commit analizado: `99a9826`

## Estado actual verificado

`internal/resources/properties.yml` contiene valores concretos para la conexión y las dos claves JWT:

```yaml
database:
  # ...
  password: <valor-versionado>
jwt:
  accessTokenSecret: <valor-versionado>
  refreshTokenSecret: <valor-versionado>
```

`internal/resources/sql/migrations/000002_insert_users_table.up.sql` inserta una identidad administrativa fija con un hash de contraseña versionado:

```sql
INSERT INTO users (..., is_admin, ...)
VALUES (..., true, ...);
```

La configuración se carga desde recursos internos y no existe un archivo de ejemplo seguro ni una guía de rotación. La revisión también confirmó que `.gitignore` no establece una política completa para configuraciones locales.

## Control de deriva

Ejecutar antes de editar:

```bash
git rev-parse --short HEAD
git status --short
git log -1 --oneline -- internal/resources/properties.yml internal/resources/sql/migrations/000002_insert_users_table.up.sql .gitignore README.md
```

Continuar si los archivos siguen cumpliendo la descripción anterior. No mostrar sus valores con `cat`, `sed` ni un grep que imprima coincidencias. Si el commit cambió o los secretos ya fueron retirados, revisar el diff y adaptar el plan.

## Alcance

- `internal/resources/properties.yml`: dejar de versionarlo o convertirlo en una plantilla que sólo contenga referencias a variables de entorno, nunca valores utilizables.
- `internal/resources/properties.example.yml` o `.env.example`: documentar todas las claves requeridas con placeholders no sensibles.
- `.gitignore`: ignorar configuración local real, `.env*` salvo ejemplos y artefactos de credenciales.
- `internal/resources/sql/migrations/000002_insert_users_table.up.sql`: no editar si hay riesgo de checksums aplicados; en ese caso conservar y añadir estrategia forward/baseline.
- Decisión aplicada (sin inventario de entornos): conservar `000002` intacta y añadir `000006_revoke_seeded_admin` que elimina el admin sembrado por clave natural.
- Añadir `docs/security-operations.md` con rotación, bootstrap administrativo y respuesta ante exposición.
- Actualizar `README.md` para apuntar al ejemplo y al procedimiento de bootstrap.
- Añadir una prueba o script de CI que detecte configuraciones reales y patrones de secretos en archivos versionados.

## Fuera de alcance

- Rotar credenciales en una base de datos, gestor de secretos o despliegue real.
- Reescribir el historial Git. Eso requiere coordinación con todos los clones y autorización explícita.
- Implementar el cargador y validador completo de configuración; corresponde al plan 005.
- Diseñar el endpoint de administración; el bootstrap seguro puede ser una utilidad explícita y de un solo uso.

## Implementación

1. Crear una plantilla de configuración sin secretos. Debe listar host, puerto, nombre de base, usuario, SSL mode, issuer, audience, expiraciones y nombres de variables de entorno. Usar ejemplos inequívocamente falsos, por ejemplo `change-me-with-32-random-bytes`; nunca reutilizar valores del archivo actual.
2. Hacer que el archivo con valores reales quede fuera de Git. Si el runtime todavía requiere `properties.yml` hasta el plan 005, conservar una copia sólo local e ignorada. El commit debe contener únicamente la plantilla.
3. Agregar reglas estrechas a `.gitignore`, por ejemplo `internal/resources/properties.yml`, `.env`, `.env.local`, manteniendo con `!` los archivos `*.example`.
4. Retirar del seed la identidad administrativa fija. Para instalaciones nuevas, el esquema puede crear tablas y catálogos, pero no una credencial conocida.
5. Documentar un bootstrap administrativo que acepte el password sólo por entrada no eco o variable efímera, genere bcrypt en runtime, valide unicidad y falle si ya existe un administrador. Si la utilidad aún no existe, documentar su contrato y dejar su implementación dentro del plan 004; no sugerir SQL con password literal.
6. Documentar el incidente: asumir que cualquier valor previamente versionado está comprometido; rotar DB/access/refresh por fuera del repositorio; revocar sesiones; desplegar; verificar; y recién entonces invalidar los anteriores. No incluir los valores antiguos en el documento.
7. Añadir un control automatizado. Preferir `gitleaks` con configuración versionada y allowlist mínima. El control debe revisar el historial disponible en CI y el working tree.
8. Evaluar la reescritura histórica en la guía, pero marcarla como operación coordinada y destructiva. No ejecutarla en este plan.

## Pruebas y verificación

| Comando | Resultado esperado |
|---|---|
| `git status --short` | Sólo aparecen los archivos del alcance. |
| `git ls-files internal/resources/properties.yml` | No imprime nada si el archivo real se ignora. |
| `git check-ignore -v internal/resources/properties.yml` | Muestra la regla intencional de `.gitignore`. |
| `git grep -nE '(accessTokenSecret|refreshTokenSecret|password):[[:space:]]+[^<$]' -- ':!plans/**' ':!*example*'` | Sin coincidencias de valores concretos; no ampliar el patrón de forma que imprima secretos existentes durante la preparación. |
| `gitleaks detect --source . --no-banner --redact` | Sale 0 y no revela secretos. |
| `git diff --check` | Sale 0. |

Además, crear pruebas para cualquier utilidad de bootstrap: primer alta exitosa, segundo intento rechazado, password débil rechazado, entrada nunca registrada y transacción revertida ante error.

## Criterios de aceptación

- [x] Ninguna credencial o clave JWT utilizable queda en archivos versionados.
- [x] El archivo local real está ignorado y existe una plantilla completa, segura y validable.
- [x] Una instalación nueva no conserva una cuenta administrativa con identidad o password predecible (revoke forward `000006`; `000002` histórica intacta por checksums).
- [x] La guía explica rotación, revocación de sesiones y bootstrap sin copiar secretos.
- [x] El escaneo de secretos es ejecutable localmente y queda listo para integrarse en CI.
- [x] El diff no modifica código de negocio ni sistemas externos.

## Condiciones de parada

- Detenerse si se descubre que los valores versionados siguen activos en un entorno real. Reportar sólo el tipo de secreto y ubicación; solicitar al propietario que coordine la rotación, sin copiar el valor.
- Detenerse antes de reescribir historial, eliminar una cuenta real o cambiar secretos externos: esas acciones requieren autorización explícita.
- Detenerse si `000002` ya está aplicada en entornos persistentes y quitar su insert afectaría una estrategia de checksums. En ese caso, conservar la migración histórica, crear una nueva estrategia de baseline para instalaciones frescas y documentar la decisión.

## Notas de mantenimiento

- Revisar ejemplos y guía al agregar una nueva variable sensible.
- Ejecutar el escáner en cada PR y en una tarea periódica.
- No usar los mismos secretos entre desarrollo, pruebas y producción.
- Registrar fecha y responsable de rotaciones fuera del repositorio, nunca los valores.
