# Plan 003 — Establecer baseline de calidad y CI

## Objetivo

Convertir compilación, pruebas, análisis estático, migraciones y detección de vulnerabilidades/secretos en una puerta automática reproducible para cada cambio.

- Prioridad: P1
- Esfuerzo: M
- Depende de: plan 002
- Rama sugerida: `codex/003-baseline-ci`
- Commit analizado: `99a9826`

## Estado actual verificado

- Sólo existen dos archivos `_test.go`; uno contiene gran parte del cuerpo comentado y el test de propiedades no compila contra las firmas actuales.
- No existen `.github/workflows`, `Makefile`, configuración de lint, compose de prueba ni health checks utilizables por CI.
- `go test ./...` y `go vet ./...` fallan actualmente por el grafo de módulos; el plan 002 debe dejarlos verdes antes de comenzar.
- `staticcheck` está disponible en la máquina de análisis, pero `govulncheck`, `gitleaks` y `trivy` no estaban instalados.

La inspección base produjo únicamente estos tests y ningún archivo de CI/lint/build:

```text
internal/infra/config/database/persistencepgx_test.go
internal/infra/config/prop/database_test.go
```

## Control de deriva

```bash
git rev-parse --short HEAD
git status --short
find . -maxdepth 3 -type f \( -path './.github/*' -o -name 'Makefile' -o -name '.golangci.yml' \) -print
find . -name '*_test.go' -print
GOWORK=off go test -count=1 ./...
```

No avanzar si la última orden no sale 0; corregir primero el plan 002.

## Alcance

- `Makefile` con targets documentados y composables.
- `.github/workflows/ci.yml` con PostgreSQL efímero.
- Configuración de `golangci-lint` o conjunto explícito equivalente; no habilitar decenas de reglas sin baseline.
- `docker-compose.test.yml` o script equivalente para reproducir la DB de CI.
- Integración de `gofmt`, `go vet`, `staticcheck`, `govulncheck`, `gitleaks`, tests unitarios, integración, race y migraciones up/down/up.
- Reparar o reemplazar los dos tests existentes para que prueben contratos vigentes.
- Documentar ejecución local en `README.md`/`CONTRIBUTING.md`.

## Fuera de alcance

- Alcanzar un porcentaje arbitrario de cobertura.
- Corregir todos los defectos funcionales hallados; cada plan posterior incorpora sus pruebas.
- Añadir despliegue continuo o publicar imágenes.
- Introducir servicios externos distintos de PostgreSQL.

## Diseño de la puerta

El `Makefile` debe ofrecer al menos:

```text
fmt-check     comprueba gofmt sin modificar
lint          go vet + staticcheck/golangci-lint
test          unitarios rápidos
test-race     suite con detector de carreras
test-db       migración e integración PostgreSQL
vuln          govulncheck
secrets       gitleaks --redact
verify        agrega todas las comprobaciones obligatorias
```

El workflow debe:

1. Ejecutarse en pull requests y pushes a `main`.
2. Usar la versión fijada por `toolchain`/`go.mod`, caché por `go.sum` y permisos GitHub mínimos (`contents: read`).
3. Fijar acciones de terceros por SHA completo, con el tag humano en comentario.
4. Levantar PostgreSQL con healthcheck y credenciales efímeras exclusivas de CI.
5. Aplicar migraciones desde cero, ejecutar down hasta cero y aplicar nuevamente.
6. Ejecutar `make verify` con `DATABASE_TEST_URL` inyectada por el job.
7. No imprimir DSN ni secretos; usar `gitleaks --redact`.

## Implementación

1. Borrar tests comentados sólo después de reemplazarlos por casos ejecutables.
2. Separar tests unitarios (`go test ./...`) de integración usando build tag `integration` o una variable validada. Los tests de DB deben saltar con mensaje claro si falta `DATABASE_TEST_URL`, pero CI siempre la provee.
3. Añadir helpers de test para aplicar migraciones y truncar datos. Cada test debe aislarse por transacción o limpieza determinista.
4. Configurar lint inicialmente con reglas de alto valor: errores ignorados, shadowing, context, SQL rows, imports y complejidad razonable. Registrar exclusiones por línea sólo con explicación.
5. Incorporar `govulncheck ./...`; una vulnerabilidad alcanzable bloquea. Una no alcanzable se documenta y se actualiza en el plan 009.
6. Incorporar el escáner del plan 001.
7. Añadir coverage como artefacto/información, no como número usado para maquillar calidad. Establecer una base inicial y prohibir descenso en paquetes críticos sólo después de medirla.
8. Documentar prerequisitos, comandos y diferencia entre test unitario/integración.

## Comandos de verificación

| Comando | Resultado esperado |
|---|---|
| `make fmt-check` | Sale 0; no modifica archivos. |
| `make lint` | Sale 0. |
| `make test` | Sale 0 sin requerir DB externa. |
| `DATABASE_TEST_URL='<dsn-efimero>' make test-db` | Migra y sale 0. |
| `make test-race` | Sale 0 sin carreras. |
| `make vuln` | Sin vulnerabilidades alcanzables. |
| `make secrets` | Sale 0, salida redactada. |
| `DATABASE_TEST_URL='<dsn-efimero>' make verify` | Sale 0 y reproduce CI. |
| `git diff --check` | Sale 0. |

## Criterios de aceptación

- [x] Existe una sola orden local (`make verify`) equivalente a la puerta de CI.
- [x] CI usa PostgreSQL efímero y verifica migraciones up/down/up.
- [x] Unitarios pueden ejecutarse sin DB; integración nunca apunta accidentalmente a desarrollo/producción.
- [x] Los tests existentes fueron reparados o reemplazados, sin cuerpos comentados.
- [x] Lint (`go vet` + `staticcheck`), race, vulnerabilidades y secretos bloquean el workflow.
- [x] Herramientas y acciones están fijadas a versiones/SHA reproducibles (`tools/versions.env`).

## Condiciones de parada

- Detenerse si el plan 002 no está verde.
- Detenerse si `DATABASE_TEST_URL` parece una base no efímera: exigir nombre/host de prueba y una protección adicional en el helper.
- No silenciar un hallazgo de seguridad para poner CI verde; abrir excepción documentada con propietario y caducidad.

## Notas de mantenimiento

- Renovar SHAs de acciones con PRs automáticos revisados.
- Actualizar versiones de herramientas en un archivo central.
- Mantener `verify` por debajo de un tiempo razonable; paralelizar jobs, no eliminar controles.
