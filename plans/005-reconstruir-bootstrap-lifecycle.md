# Plan 005 — Reconstruir configuración, bootstrap y lifecycle

## Objetivo

Hacer que configuración, migración, dependencias, servidor HTTP y pool PostgreSQL tengan un orden de inicio/apagado explícito, validado y observable, sin carreras de registro de rutas ni errores ignorados.

- Prioridad: P1
- Esfuerzo: L
- Depende de: planes 002 y 003
- Rama sugerida: `codex/005-bootstrap-lifecycle`
- Commit analizado: `99a9826`

## Estado actual verificado

`cmd/main.go` carga configuración, migra, registra componentes y termina con llamadas cuyos errores/lifecycle no se gestionan de forma explícita:

```go
initModules.Init(true, true)
r.Run()
```

`internal/infra/config/sql/migration.go` construye su propia conexión hardcodeada, separada de la configuración runtime. `persistencepgx.go` agrega `sslmode=disable`, ignora el error de `pgxpool.ParseConfig` y usa el resultado inmediatamente. No hay `http.Server` con timeouts, manejo de señales, cierre del pool, liveness/readiness ni validación integral de config.

## Control de deriva

```bash
git rev-parse --short HEAD
git status --short
sed -n '1,240p' cmd/main.go
sed -n '1,180p' internal/infra/config/sql/migration.go
sed -n '1,180p' internal/infra/config/database/persistencepgx.go
rg -n 'Run\(|ListenAndServe|Shutdown|signal.Notify|sslmode|AllowOrigins|swagger' cmd internal
```

## Arquitectura objetivo

```text
load env/file -> validate config -> build logger -> open pool + ping
             -> run migrations (same DSN) -> construct services/routes
             -> mark ready -> serve HTTP
signal/error -> mark unready -> HTTP shutdown with deadline
             -> stop background work -> close pool -> exit
```

La composición puede usar `initModules/v2.NewApp`/`Lifecycle` si ofrece exactamente este orden y propagación de errores. Si añade complejidad o concurrencia implícita, preferir un bootstrap explícito en este repositorio y retirar la dependencia. Documentar la decisión en un ADR corto.

## Alcance

- Modelo único de configuración con carga y `Validate()` estrictos.
- Un único DSN para migraciones y runtime; soporte TLS configurable y seguro por defecto.
- Composición explícita sin registro de rutas en goroutines.
- `http.Server` con timeouts, límites y apagado por `SIGINT`/`SIGTERM`.
- Ping de DB antes de readiness y cierre ordenado del pool.
- `/health/live` y `/health/ready` sin información sensible.
- CORS, Swagger, logging y modo Gin configurables por ambiente.
- Tests unitarios de configuración y de integración del lifecycle.

## Fuera de alcance

- Rediseñar cada DAO; plan 006.
- Agregar Kubernetes/Terraform.
- Implementar métricas/tracing completos. Se deben dejar puntos de extensión, no un stack nuevo.

## Configuración mínima

Definir un único `Config` con grupos tipados:

```text
Server: address, readHeaderTimeout, readTimeout, writeTimeout, idleTimeout,
        shutdownTimeout, maxHeaderBytes
Database: URL o host/port/name/user/password, sslMode, pool min/max,
          maxConnLifetime, connectTimeout, migrationsPath
JWT: access/refresh secrets, issuer, audience, TTLs, leeway
CORS: allowedOrigins, methods, headers, credentials
Docs: swaggerEnabled
Runtime: environment, logLevel
```

Precedencia: variables de entorno > archivo local opcional > defaults no sensibles. Producción debe rechazar secrets placeholder, `sslmode=disable`, CORS vacío/incorrecto y Swagger público salvo opt-in.

## Implementación

1. Consolidar estructuras de config duplicadas y añadir `Validate()` con errores que nombren el campo, no el valor.
2. Construir el DSN una sola vez o usar `DATABASE_URL`; pasarlo tanto al migrador como a `pgxpool.ParseConfig`. Comprobar el error antes de usar config.
3. Aplicar configuración del pool y hacer `Ping` con timeout de startup.
4. Ejecutar migraciones sin credenciales propias. Un error de migración aborta antes de exponer el puerto.
5. Construir repositorios, casos de uso, controladores y rutas secuencialmente. No iniciar goroutines hasta terminar el router.
6. Crear `http.Server` y tratar `http.ErrServerClosed` como cierre normal; cualquier otro error termina con código no cero.
7. Al recibir señal: readiness false, cancelar contexto raíz, llamar `Shutdown` con deadline, esperar workers registrados y cerrar pool. Si expira, registrar causa y salir no cero.
8. `live` sólo indica proceso vivo. `ready` comprueba estado de startup y DB con timeout corto; devolver 503 si no está listo.
9. Habilitar recovery seguro en Gin; logs estructurados con request ID, sin headers Authorization ni bodies.
10. Configurar CORS por lista explícita. Si credentials es true, prohibir `*`.
11. Swagger apagado por defecto fuera de desarrollo y ruta separada de la API.

## Pruebas obligatorias

- Config válida carga por env; cada campo obligatorio inválido aborta con error redactado.
- `ParseConfig`, ping o migración fallidos impiden escuchar.
- Rutas existen inmediatamente al arrancar; ejecutar detector de carreras.
- Readiness: 503 antes de completar startup, 200 al estar listo, 503 al comenzar shutdown o perder DB.
- SIGTERM deja de aceptar requests, permite terminar uno en vuelo dentro del deadline y cierra el pool.
- Error inesperado de `ListenAndServe` se propaga.
- CORS rechaza origen no permitido; Swagger respeta flag/ambiente.

## Comandos de verificación

```bash
make fmt-check
make lint
make test
DATABASE_TEST_URL='<dsn-efimero>' make test-db
make test-race
DATABASE_URL='<dsn-efimero>' go run ./cmd
```

Con el proceso iniciado, verificar en otra terminal:

```bash
curl -fsS http://127.0.0.1:8080/health/live
curl -fsS http://127.0.0.1:8080/health/ready
```

Enviar `SIGTERM` al PID exacto y comprobar salida limpia y puerto liberado. No usar `killall`.

## Criterios de aceptación

- [x] Migraciones y runtime usan exactamente la misma configuración de DB.
- [x] Todo error de parseo, ping, migración o servidor se comprueba y propaga.
- [x] Las rutas se registran antes de escuchar y no hay carrera en `go test -race`.
- [x] Servidor/pool tienen startup y shutdown deterministas con deadlines.
- [x] Liveness/readiness, CORS y Swagger tienen semántica probada y segura.
- [x] La decisión sobre `initModules/v2` está documentada y no queda API v1.

## Condiciones de parada

- Detenerse si el entorno real exige migraciones con una identidad DB separada: modelarla explícitamente y documentar privilegios, no volver a hardcodear un segundo DSN.
- Detenerse si hay background workers no inventariados; agregarlos al lifecycle antes de afirmar shutdown limpio.
- No degradar TLS para hacer pasar tests; la instancia efímera puede permitir disable sólo mediante configuración de test explícita.

## Notas de mantenimiento

- Toda dependencia con `Close` debe registrarse en el lifecycle en orden inverso de creación.
- Nuevas opciones deben tener validación, ejemplo seguro y test.
- Mantener endpoints health baratos y sin versión, host o errores internos detallados.
