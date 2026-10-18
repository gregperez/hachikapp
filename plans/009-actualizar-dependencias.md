# Plan 009 — Actualizar dependencias de terceros

## Objetivo

Actualizar Go y las librerías directas a versiones vigentes verificadas, en cambios pequeños y reversibles, con pruebas de compatibilidad y vulnerabilidades después de que la base esté estable.

- Prioridad: P2
- Esfuerzo: M
- Depende de: planes 003 a 008
- Rama sugerida: `codex/009-dependencias`
- Commit analizado: `99a9826`

## Estado actual verificado

`go.mod` usa `go 1.24.0`. La máquina de análisis tiene Go 1.26.3 y, al 2026-09-23, la versión estable publicada es 1.27.1. Varias dependencias importadas directamente están marcadas como indirectas.

Versiones observadas y candidatas verificadas durante el análisis:

| Módulo | Actual | Candidata |
|---|---:|---:|
| `github.com/gin-gonic/gin` | 1.10.0 | 1.12.0 |
| `github.com/jackc/pgx/v5` | 5.7.2 | 5.11.0 |
| `github.com/gin-contrib/cors` | 1.7.4 | 1.7.9 |
| `github.com/go-playground/validator/v10` | 10.25.0 | 10.30.5 |
| `github.com/golang-migrate/migrate/v4` | 4.18.1 | 4.20.1 |
| `github.com/nicksnyder/go-i18n/v2` | 2.5.1 | 2.6.1 |
| `github.com/sirupsen/logrus` | 1.9.3 | 1.9.4 |
| `github.com/swaggo/files/v2` | 1.0.1 | verificar latest compatible |
| `github.com/swaggo/gin-swagger` | 1.6.0 | 1.6.1 |
| `github.com/swaggo/swag` | 1.16.4 | 1.16.6 |

Las versiones deben volver a comprobarse al ejecutar el plan; son un snapshot, no autorización para saltar a una major nueva.

## Control de deriva

```bash
git rev-parse --short HEAD
git status --short
go version
GOWORK=off go list -m -u -mod=readonly all
GOWORK=off go mod why -m github.com/gin-gonic/gin
```

Ejecutar `make verify` antes de tocar versiones. Debe estar verde para atribuir correctamente cualquier regresión.

## Alcance

- Actualizar `go`/`toolchain` si no quedó resuelto en plan 002.
- Corregir clasificación direct/indirect mediante `go mod tidy`.
- Actualizar familias de dependencias una por una.
- Revisar changelogs oficiales y adaptar breaking/deprecations.
- Ejecutar `govulncheck`, tests, race, OpenAPI e integración en cada grupo.
- Configurar actualización automatizada posterior (Dependabot/Renovate) con agrupación conservadora.

## Fuera de alcance

- Adoptar versiones major nuevas sin plan específico.
- Sustituir framework web/DB/migrador.
- Migrar Logrus a `slog` dentro de un bump rutinario. Tomar decisión y abrir plan separado si se desea.
- Actualizar sólo para reducir números de avisos sin comprobar alcanzabilidad/compatibilidad.

## Estrategia

1. Consultar fuentes primarias: release notes/tags oficiales de Go y cada módulo. Registrar links y breaking changes en el PR.
2. Mantener en `go.mod` todas las dependencias importadas por código productivo como directas. Dejar indirectas sólo las transitivas.
3. Actualizar en este orden, con commit y `make verify` por grupo:
   1. patch/minor de tooling Swagger/i18n/validator/cors;
   2. pgx y migrate, ejecutando toda integración/migraciones;
   3. Gin, ejecutando handlers, middleware, CORS y shutdown;
   4. logging sólo patch;
   5. Go toolchain, ejecutando además vet/staticcheck.
4. Para cada módulo usar `go get module@version`, después `go mod tidy`; revisar el diff de `go.mod`/`go.sum`, no aceptar cambios transitivos inexplicados.
5. Revisar release notes por cambios de defaults, seguridad, tipos o soporte Go mínimo. Añadir tests de regresión antes de adaptar el código cuando sea posible.
6. Ejecutar `govulncheck ./...`. Una vulnerabilidad alcanzable bloquea; si no hay fix, documentar componente, path alcanzable, mitigación temporal, propietario y fecha de revisión.
7. Confirmar builds con la versión Go mínima (`go 1.26.x`) y toolchain preferido (`1.27.1` o estable verificada al ejecutar). Si se sube el mínimo, actualizar README/CI/containers juntos.
8. Configurar Dependabot/Renovate semanal: patches agrupados, minors revisados, majors nunca automerge; Go toolchain en PR separado.

## Matriz de regresión por familia

| Familia | Verificación adicional |
|---|---|
| Gin/CORS | routing, binding, recovery, headers, preflight, shutdown |
| pgx/migrate | pool, transactions, cancellation, rows, up/down/up |
| validator/i18n | códigos/campos y fallback sin panic |
| swaggo | generación sin diff y lint OpenAPI |
| Go | build mínimo/preferido, vet, staticcheck, race |

## Comandos de verificación

Después de cada grupo:

```bash
GOWORK=off go mod tidy
GOWORK=off go mod verify
git diff -- go.mod go.sum
make verify
```

Al final:

```bash
GOWORK=off go list -m -u -mod=readonly all
govulncheck ./...
git diff --check
git status --short
```

La primera orden puede mostrar majors deliberadamente no adoptadas; deben quedar documentadas. Todas las demás salen 0 y el status contiene sólo archivos esperados.

## Criterios de aceptación

- [x] Cada versión elegida fue revalidada contra una fuente oficial al ejecutar el plan.
- [x] `go.mod` distingue correctamente dependencias directas/transitivas.
- [x] Cada familia se actualizó y verificó en un commit aislable.
- [x] La suite completa, race, migraciones, OpenAPI, lint y `govulncheck` están verdes.
- [x] CI prueba la versión Go mínima y la preferida.
- [x] Majors omitidas y decisión Logrus/slog están documentadas, no ocultas.
- [x] Existe automatización conservadora para futuras actualizaciones.

## Condiciones de parada

- Detenerse ante una major o cambio de mínimo Go no previsto; crear plan de migración.
- Detenerse si una actualización altera autenticación, parsing o SQL de forma no cubierta; añadir primero el test de regresión.
- No usar `-mod=mod`, excludes o replaces permanentes para esconder incompatibilidades sin explicación.
- No aceptar una vulnerabilidad alcanzable sin mitigación y fecha de resolución.

## Notas de mantenimiento

- Ejecutar actualizaciones pequeñas y frecuentes; evitar saltos anuales masivos.
- Mantener Go N-1 como mínimo sólo mientras reciba soporte compatible con la política del equipo.
- Revisar dependencias transitivas grandes y licencias en el ciclo de release.
