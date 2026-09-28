# Plan 015: Aislar el composition root y eliminar estado global productivo

> **Instrucciones para el agente ejecutor**: Migre base-golang a instancias
> explícitas usando las APIs aisladas de initModules/v2. No modifique initModules;
> sus APIs ya existen. Mantenga el proceso compilable al final de cada fase y
> actualice la fila 015 del índice.
>
> **Control de deriva**:
>
> ```bash
> git diff --stat 59aeb1f..HEAD -- cmd internal/infra/config/instance internal/infra/config/database internal/infra/bootstrap internal/infra/logging docs/adr/001-initmodules-lifecycle.md
> git -C /Users/wilsonsayago/workspace/golang/initModules diff --stat d698e2a..HEAD -- container.go app.go lifecycle.go
> ```

## Estado

- **Estado**: Verificado — O-033 cerrado en 018 (cleanup del pool poseído)
- **Prioridad**: P1
- **Esfuerzo**: L
- **Riesgo**: HIGH
- **Depende de**: planes 012 y 013
- **Categoría**: tech-debt / correctness / tests
- **Planeado en**: `base-golang@59aeb1f`, `initModules@d698e2a`, 2026-09-25

## Por qué importa

La plantilla aún mezcla constructores con `initModules.Once`, singleton de DB,
logger global y señales gestionadas aparte. Eso impide crear dos aplicaciones
aisladas en tests o embebidas y mantiene abierta O-031. initModules/v2 ya ofrece
`NewContainer`, `OnceIn`, `NewApp` y `Lifecycle`; base-golang debe ser su ejemplo
de referencia sin depender de registros globales.

## Estado actual

- `cmd/main.go:42-43` usa `OnceValue`; `:79` llama `slog.SetDefault`; `:91`
  registra la DB global; `:114` crea señales directamente.
- `internal/infra/config/instance/usecase.go` construye servicios mediante
  `initModules.Once` y usa `audit.NewLogger(nil)`.
- `internal/infra/config/instance/primary.go` construye controladores globales.
- `internal/infra/config/database/persistencepgx.go` expone singleton.
- `initModules/container.go` ofrece `NewContainer`, `OnceIn` y `OnceValueIn`.
- `initModules/app.go` ofrece `NewApp`, registro ordenado de `Lifecycle`, stop en
  orden inverso y `RunWithSignals`.
- `initModules/README.md:176-188` recomienda instancias locales y evita el App
  global para aislamiento.

## Comandos necesarios

| Propósito | Comando | Resultado esperado |
|---|---|---|
| Unit/race | `GOWORK=off go test -race -count=1 ./...` | Sale 0 |
| Gate completo | `DATABASE_TEST_URL='<dsn-efimero>' GOWORK=off make verify` | Sale 0 |
| Globales | `rg -n 'slog\.SetDefault|slog\.Default\(|database\.SetSingleton|GetDatabasePostgresPgx|initModules\.Once(Value)?\(' cmd internal --glob '*.go'` | Sin matches productivos; excepciones explícitas sólo legacy/test |

## Alcance

**Dentro de alcance**:

- `cmd/main.go`, nuevo composition root bajo `internal/infra/bootstrap` o
  `internal/bootstrap`, wiring de config, DB, logger, auth, repositorios,
  controladores y lifecycles.
- Constructores explícitos y tests de aislamiento.
- Registro del cleaner de sesiones del plan 013.
- Actualización de ADR/arquitectura.

**Fuera de alcance**:

- Cambiar endpoints, DTOs, SQL o reglas de autorización.
- Modificar authBase/initModules.
- Introducir un framework DI por reflexión.
- Mantener dos composition roots productivos indefinidamente.
- Usar reset global en tests como sustituto del aislamiento.

## Flujo Git

- Rama: `codex/015-isolated-composition-root`.
- Commits sugeridos: constructores explícitos, lifecycles, switch de main,
  eliminación de globales, docs/tests.

## Pasos

### Paso 1: caracterizar el wiring actual

Añada un smoke test de composición que construya engine, servicios y repositorios
con dobles sin escuchar puertos. Capture qué dependencias deben existir y el
orden de cierre. Este test debe fallar si un constructor lee un singleton global.

**Verificar**: test enfocado pasa antes del switch y no requiere señales reales.

### Paso 2: crear `Application`

Defina una estructura de composición dueña de:

- configuración validada;
- `*initModules.Container` y `*initModules.App`;
- logger `*slog.Logger`;
- pool PostgreSQL;
- token manager, servicios, repositorios y controladores;
- engine y servidor HTTP.

`NewApplication` debe retornar `(*Application, error)`, no panic ni `os.Exit`.
Use constructores directos o `OnceIn` sólo para dependencias realmente únicas
dentro de esa instancia. No use service locator desde dominio/controladores.

### Paso 3: convertir recursos en Lifecycle

Implemente adaptadores `Lifecycle` explícitos para:

1. DB: ping al iniciar, close al detener;
2. traducciones u otros procesos iniciables;
3. limpieza refresh del plan 013;
4. HTTP: listen al iniciar; readiness false y shutdown con deadline al detener.

Registre en orden de dependencia para que el stop inverso cierre HTTP antes de
DB. No cree goroutines desde constructores.

**Verificar**: test de orden Start/Stop y rollback de componentes ya iniciados
si uno falla.

### Paso 4: eliminar fallbacks globales de logger

Inyecte logger en audit, recovery, access log, DB/config y lifecycles. Elimine
`slog.SetDefault` y `logging.OrDefault` de caminos productivos; constructores
deben exigir logger no nil o aplicar un logger local explícito durante bootstrap.
`main` puede usar un logger mínimo sólo antes de construir `Application`.

### Paso 5: sustituir instancias/controladores globales

Añada constructores `New...` explícitos para DAOs, repositorios, servicios y
controladores. Cambie el registro de rutas para recibir una estructura de
dependencias. Retire `internal/infra/config/instance` cuando no tenga callers;
si debe permanecer por compatibilidad interna, márquelo deprecated y pruebe que
producción no lo importa.

### Paso 6: simplificar `main`

`main` debe cargar/validar configuración, crear logger, construir Application y
llamar una sola vez `app.RunWithSignals`. Propague errores a `main`; no use
`log.Fatal` dentro de config/adapters. Use `initModules.NewConfigLoader` si es
necesario para eliminar el registro global de propiedades.

### Paso 7: probar aislamiento real

Construya dos aplicaciones en paralelo con loggers, contenedores, DB dobles y
engines distintos. Una no debe recibir logs, rutas ni dependencias de la otra.
Ejecute bajo `-race`.

## Plan de pruebas

- `application_test.go`: dos instancias paralelas, dependencia ausente, orden de
  lifecycle, fallo de Start y stop inverso.
- Tests HTTP existentes deben seguir pasando sin reset global.
- Test de shutdown con listener real en `127.0.0.1:0`, request en vuelo,
  readiness false y puerto liberado; esto cierra O-014.
- Test de logger aislado con dos buffers; cierra O-031.

## Criterios de terminado

- [x] `main` usa un `Application` y un `initModules.App` aislados.
- [x] No hay singletons/global logger en caminos productivos.
- [x] DB, HTTP y cleaner implementan lifecycle y cierran en orden seguro.
- [x] Dos aplicaciones paralelas pasan bajo race detector sin contaminación.
- [x] SIGTERM/cancelación drena una request y libera el listener.
- [x] O-014 y O-031 se actualizan como resueltas con evidencia.
- [x] `make verify` pasa contra DB efímera.

## Condiciones de parada

- El plan 013 no entrega una limpieza construible sin globales.
- Un contrato público HTTP o authBase tendría que cambiar.
- La migración requiere editar initModules en lugar de usar sus APIs publicadas.
- No puede establecerse el orden seguro HTTP → workers → DB.
- Aparecen cambios ajenos en los mismos archivos.

## Notas de mantenimiento

- Nuevos adapters con recursos deben implementar `Lifecycle`; no inicien trabajo
  en constructores.
- Evite reintroducir `Once` global por comodidad. `OnceIn` sólo es aceptable con
  el container de una Application.
- Revise especialmente ownership de contextos y que shutdown no use el contexto
  ya cancelado para cerrar recursos.
