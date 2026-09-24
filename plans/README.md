# Planes de mejora de base-golang

Estado analizado: commit `99a9826` (`main`), 2026-09-23.

Estos documentos son instrucciones autocontenidas para que otro agente pueda ejecutar la modernización sin repetir el análisis inicial. El orden recomendado prioriza cortar riesgos de seguridad y recuperar una compilación reproducible antes de refactorizar.

## Reglas para todos los planes

- Crear una rama `codex/<numero>-<slug>` por plan y mantener commits pequeños y revisables.
- Antes de editar, ejecutar el control de deriva indicado en el plan. Si cambió una interfaz o el comportamiento ya fue corregido, detenerse y actualizar el plan en vez de aplicar mecánicamente los pasos.
- No imprimir, copiar a logs ni incluir en diffs valores secretos existentes.
- No editar migraciones que ya puedan haberse aplicado en entornos persistentes salvo que el plan lo indique expresamente; preferir migraciones nuevas y reversibles.
- Conservar cambios ajenos presentes en el worktree.
- Ejecutar todas las verificaciones del plan y adjuntar los comandos/resultados al traspaso.
- No avanzar al siguiente plan si queda una prueba obligatoria en rojo, salvo fallo externo documentado y aceptado.

## Orden y dependencias

| ID | Plan | Prioridad | Esfuerzo | Depende de | Estado |
|---|---|---:|---:|---|---|
| 001 | [Contener secretos y datos de arranque](./001-contener-secretos-y-datos-bootstrap.md) | P0 | M | — | Hecho (`codex/001-contener-secretos`) |
| 002 | [Recuperar compilación y migrar authBase/initModules](./002-recuperar-compilacion-y-migrar-auth.md) | P0 | L | 001 | Hecho (`codex/002-migrar-auth`) |
| 003 | [Establecer baseline de calidad y CI](./003-establecer-baseline-calidad-ci.md) | P1 | M | 002 | Hecho (`codex/003-baseline-ci`) |
| 004 | [Corregir límites de autorización](./004-corregir-limites-autorizacion.md) | P0 | M | 002, 003 | Pendiente |
| 005 | [Reconstruir configuración, bootstrap y lifecycle](./005-reconstruir-bootstrap-lifecycle.md) | P1 | L | 002, 003 | Pendiente |
| 006 | [Corregir límites de persistencia PostgreSQL](./006-corregir-persistencia-postgresql.md) | P1 | L | 003, 005 | Pendiente |
| 007 | [Endurecer esquema y migraciones](./007-endurecer-esquema-y-migraciones.md) | P1 | M | 003, 004, 006 | Pendiente |
| 008 | [Estabilizar contrato HTTP y OpenAPI](./008-estabilizar-contrato-http-openapi.md) | P1 | L | 004, 006, 007 | Pendiente |
| 009 | [Actualizar dependencias de terceros](./009-actualizar-dependencias.md) | P2 | M | 003–008 | Pendiente |
| 010 | [Remediar observaciones de verificación y cierre](./010-remediar-observaciones-y-cierre.md) | P0/P1 | M | 001–009 | Pendiente; registro incremental |

```text
001 -> 002 -> 003 -> 005 -> 006 -----> 007 -> 008 -> 009 -> 010
               \-> 004 -----------------------------------/
```

## Resultado esperado

Al completar la secuencia, el repositorio debe ser una base Go reproducible y reutilizable, sin secretos ni usuarios privilegiados embebidos, con autenticación de refresh rotativa, autorización de mínimo privilegio, arranque y apagado deterministas, acceso PostgreSQL con contexto, migraciones verificadas, contrato HTTP estable, documentación OpenAPI coherente y CI que bloquee regresiones.

## Registro de observaciones diferidas

Las observaciones encontradas al verificar implementaciones no se deben perder ni
marcar como resueltas por el mero hecho de diferirse. Se consolidan en el
[plan 010](./010-remediar-observaciones-y-cierre.md), que se ejecuta una vez
implementados los planes 001–009 y antes de declarar la modernización cerrada.

Al verificar cada plan, agregar al registro del plan 010 una entrada con: plan de
origen, prioridad, evidencia `archivo:línea` o comando, impacto, estado
(`abierto`, `bloqueado externo` o `resuelto`) y prueba de cierre esperada. Los
hallazgos P0/P1 siguen bloqueando producción aunque su corrección se programe al
final.

## Prompt de entrega a otro agente

Usar este texto, sustituyendo el ID:

```text
Implementa únicamente el plan plans/NNN-<nombre>.md. Lee primero plans/README.md
y respeta sus dependencias. Ejecuta el control de deriva antes de editar; si falla,
detente y reporta la diferencia. No amplíes el alcance. Completa las pruebas y todos
los criterios de aceptación, conserva cambios ajenos y al finalizar informa archivos
modificados, comandos ejecutados, resultados, riesgos residuales y cualquier paso
operativo que requiera autorización humana. No marques el plan como completado si
queda una verificación obligatoria en rojo.
```

## Hallazgos considerados y no convertidos en planes separados

- No se observó interpolación SQL directa: las consultas revisadas usan parámetros. Se preservará esta propiedad en los planes 006 y 007.
- La política CORS no es wildcard, pero está fija en código; se incorpora al plan 005 como configuración validada.
- El hashing bcrypt pertenece a `authBase`; la actualización y sus pruebas quedan en el plan 002.
- No apareció un N+1 evidente en los listados principales. Los índices y límites de paginación se tratan en los planes 007 y 008.
- La sustitución de Logrus por `log/slog` no es requisito de seguridad. El plan 009 obliga a tomar y documentar la decisión, sin mezclar una migración de logging con actualizaciones rutinarias.

## Trabajo explícitamente fuera de alcance global

- Desplegar a producción, rotar secretos en proveedores externos o modificar bases de datos reales.
- Cambiar el producto o añadir endpoints de negocio no necesarios para corregir la base.
- Garantizar compatibilidad con tokens emitidos por la implementación antigua; por seguridad se espera su invalidación controlada.
- Convertir este repositorio en framework genérico. El objetivo es una plantilla mantenible, no una capa de abstracción propia.
