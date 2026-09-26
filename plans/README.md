# Planes de mejora de base-golang

Estado histórico inicial: commit `99a9826` (`main`), 2026-09-23.

Segunda ola planificada el 2026-09-25 contra `base-golang@59aeb1f`,
`authBase@b5c0a2c` e `initModules@d698e2a`. Los planes 012–017 son
multirrepositorio cuando su cabecera lo indica y conservan la numeración
monotónica del registro original.

Estos documentos son instrucciones autocontenidas para que otro agente pueda ejecutar la modernización sin repetir el análisis inicial. El orden recomendado prioriza cortar riesgos de seguridad y recuperar una compilación reproducible antes de refactorizar.

> Archivo histórico: cada plan conserva los paths, versiones y evidencia del
> commit que analizó; no reemplaza la documentación vigente de
> [`docs/ARCHITECTURE.md`](../docs/ARCHITECTURE.md) ni el changelog.
> Después del cierre de O-030 con authBase v3.0.1, la migración a
> `authBase/v4` reintrodujo temporalmente un `replace ../authBase`. El plan 012
> lo eliminó al publicar y consumir `authBase/v4.0.1`; los checkouts aislados
> vuelven a ser reproducibles.

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
| 003 | [Establecer baseline de calidad y CI](./003-establecer-baseline-calidad-ci.md) | P1 | M | 002 | Verificado — observación P2 diferida a 010 |
| 004 | [Corregir límites de autorización](./004-corregir-limites-autorizacion.md) | P0 | M | 002, 003 | Verificado (`0e3cec1`) |
| 005 | [Reconstruir configuración, bootstrap y lifecycle](./005-reconstruir-bootstrap-lifecycle.md) | P1 | L | 002, 003 | Verificado (`0771cbd`) |
| 006 | [Corregir límites de persistencia PostgreSQL](./006-corregir-persistencia-postgresql.md) | P1 | L | 003, 005 | Implementado — observaciones P1/P2 diferidas a 010 |
| 007 | [Endurecer esquema y migraciones](./007-endurecer-esquema-y-migraciones.md) | P1 | M | 003, 004, 006 | Verificado (`554b123`) |
| 008 | [Estabilizar contrato HTTP y OpenAPI](./008-estabilizar-contrato-http-openapi.md) | P1 | L | 004, 006, 007 | Verificado (`03f1c47`) — observaciones P1/P2 diferidas a 010 |
| 009 | [Actualizar dependencias de terceros](./009-actualizar-dependencias.md) | P2 | M | 003–008 | Implementado — observaciones P1/P2 diferidas a 010 |
| 010 | [Remediar observaciones de verificación y cierre](./010-remediar-observaciones-y-cierre.md) | P0/P1 | M | 001–009 | Implementado — residual O-014 (P2); O-030 cerrado con `authBase@v3.0.1` / `initModules@v2.0.0` |
| 011 | [Migrar logging a log/slog](./011-migrar-logging-slog.md) | P2 | M | 009, 010 | Implementado — residual O-031 (inyección completa de `slog`) |
| 012 | [Publicar authBase v4 y restaurar reproducibilidad](./012-publicar-authbase-v4-y-restaurar-reproducibilidad.md) | P0 | M | — | Verificado (`authBase/v4.0.1`; proxy, consumidor y gates verdes) |
| 013 | [Formalizar ciclo de sesiones](./013-formalizar-ciclo-de-sesiones.md) | P0 | L | 012 | Verificado (`authBase/v4.1.0`; revocación atómica y purger probado) |
| 014 | [Endurecer credenciales y login](./014-endurecer-credenciales-y-login.md) | P0 | L | 012, 013 | Verificado (`authBase/v4.2.0`; política NIST, Argon2id y throttling) |
| 015 | [Aislar composition root y lifecycle](./015-aislar-composition-root-y-lifecycle.md) | P1 | L | 012, 013 | Verificado (Application aislada; O-014 y O-031 cerrados) |
| 016 | [Agregar login OIDC con Google](./016-agregar-login-oidc-google.md) | P1 | L | 012–015 | Verificado (OIDC Google, grant opaco y `EstablishSession`) |
| 017 | [Agregar agilidad de claves JWT y eventos de seguridad](./017-agilidad-claves-jwt-y-eventos-seguridad.md) | P2 | L | 012, 013, 015 | TODO |

```text
001 -> 002 -> 003 -> 005 -> 006 -----> 007 -> 008 -> 009 -> 010
               \-> 004 -----------------------------------/

012 -> 013 -> 014
        \------> 015 -> 016
                  \---> 017
```

Para la segunda ola, ejecutar 012 primero: elimina la dependencia local no
reproducible antes de ampliar contratos de autenticación. El plan 013 aporta la
emisión e invalidación explícita de sesiones que consumen 014 y 015. El plan 015
integra el limpiador de sesiones y elimina estado global del composition root.
El login OIDC de 016 reutiliza el límite de intentos de 014. El plan 017 debe
ejecutarse después de 015 y puede avanzar en paralelo con 016, coordinando ambos
si modifican configuración o bootstrap.

Los estados válidos para planes nuevos son `TODO`, `En progreso`, `Bloqueado`,
`Verificado` y `Implementado`. Solo usar `Verificado` después de ejecutar todos
los controles obligatorios del plan.

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

En planes multirrepositorio, el agente debe crear una rama con el mismo slug en
cada repositorio afectado, registrar el commit base de cada uno y ejecutar sus
gates por separado. Publicar tags, releases o cambios remotos exige autorización
humana explícita; un plan que llegue a ese checkpoint debe detenerse sin simular
la publicación.

## Hallazgos considerados y no convertidos en planes separados

- No se observó interpolación SQL directa: las consultas revisadas usan parámetros. Se preservará esta propiedad en los planes 006 y 007.
- La política CORS no es wildcard, pero está fija en código; se incorpora al plan 005 como configuración validada.
- El hashing bcrypt pertenece a `authBase`; la actualización y sus pruebas quedan en el plan 002.
- No apareció un N+1 evidente en los listados principales. Los índices y límites de paginación se tratan en los planes 007 y 008.
- La sustitución de Logrus por `log/slog` no es requisito de seguridad. El plan 009 obliga a tomar y documentar la decisión, sin mezclar una migración de logging con actualizaciones rutinarias.
- El login con Google se modela como OIDC dentro de `base-golang`, no como acceso a Gmail ni como una librería nueva. Solo se extraerá un paquete cuando exista un segundo consumidor real.
- No se vinculan cuentas automáticamente por coincidencia de email. El primer alcance de OIDC exige que un usuario autenticado vincule explícitamente el par estable `issuer` + `subject`.
- Se descartan flujos OAuth implícitos y tokens en parámetros de redirección; el plan 016 exige Authorization Code, PKCE, `state`, `nonce` y un código opaco de un solo uso para el handoff.
- OTP por SMS/email no es prioridad de esta base. Passkeys/WebAuthn queda como evolución posterior a sesiones, throttling y OIDC, para no introducir dos modelos de identidad simultáneamente.
- Dockerfile, manifiestos Kubernetes y generación de SBOM quedan diferidos hasta definir un destino de despliegue verificable; añadirlos sin ese contrato produciría infraestructura de ejemplo difícil de mantener.

## Trabajo explícitamente fuera de alcance global

- Desplegar a producción, rotar secretos en proveedores externos o modificar bases de datos reales.
- Cambiar el producto o añadir endpoints de negocio no necesarios para corregir la base.
- Garantizar compatibilidad con tokens emitidos por la implementación antigua; por seguridad se espera su invalidación controlada.
- Convertir este repositorio en framework genérico. El objetivo es una plantilla mantenible, no una capa de abstracción propia.
