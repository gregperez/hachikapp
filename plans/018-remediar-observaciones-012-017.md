# Plan 018: Cerrar observaciones de verificación de los planes 012–017

> **Instrucciones para el agente ejecutor**: Este es un cierre multirrepositorio.
> Trabaje en el orden indicado, conserve la compatibilidad pública de authBase/v4
> y no publique tags, releases ni commits remotos sin autorización explícita.
> Ejecute cada verificación y no marque 012–017 como verificados mientras el gate
> completo de `base-golang` siga rojo.
>
> **Control de deriva (ejecutar primero)**:
>
> ```bash
> git -C /Users/wilsonsayago/workspace/golang/base-golang diff --stat 34e23f6..HEAD -- plans .gitleaks.toml scripts internal/infra/bootstrap internal/infra/config internal/infra/primary go.mod go.sum
> git -C /Users/wilsonsayago/workspace/golang/authBase diff --stat 9405427..HEAD -- core/services docs CHANGELOG.md README.md Makefile tools go.mod go.sum
> git -C /Users/wilsonsayago/workspace/golang/initModules diff --stat d698e2a..HEAD -- go.mod go.sum Makefile
> ```
>
> Si cambió un archivo afectado, confirme primero si la observación ya fue
> resuelta. Si la evidencia o el contrato vivo contradicen este documento,
> deténgase y actualice el plan; no aplique pasos mecánicamente.

## Estado

- **Estado**: Verificado — O-032–O-039 cerradas; gates verdes; sin tags/releases nuevos
- **Prioridad**: P1/P2
- **Esfuerzo**: L
- **Riesgo**: MED
- **Depende de**: planes 012–017 implementados
- **Categoría**: correctness / security / tests / docs / dependencies
- **Planeado en**: `base-golang@34e23f6`, `authBase@9405427`,
  `initModules@d698e2a`, 2026-09-28

## Por qué importa

La segunda ola implementó sus capacidades principales, pero la verificación
independiente encontró un gate rojo y varios criterios marcados como cumplidos
sin evidencia suficiente. El cierre debe recuperar un `make verify` realmente
verde, hacer efectiva la separación de claves, evitar fugas de recursos durante
bootstrap y alinear release/docs/tests con el estado publicado.

## Registro de observaciones

| ID | Prioridad | Evidencia | Impacto | Estado | Cierre esperado |
|---|---:|---|---|---|---|
| O-032 | P1 | `base-golang/internal/infra/config/jwtkeys/load_test.go:56-68`; `GOWORK=off make secrets` detecta una fixture de clave privada en el commit `80d3a32` | `make verify` falla y CI no puede certificar el plan 017 | Cerrado | Excepción exacta para la fixture histórica, prueba contra allowlists amplias y Gitleaks verde |
| O-033 | P1 | `base-golang/internal/infra/bootstrap/application.go:88-119` abre un pool propio y retorna directamente si migración o wiring falla | Procesos embebidos/tests pueden filtrar conexiones cuando el constructor falla | Cerrado | Cerrar sólo el pool poseído por `NewApplication` en todo retorno con error; probar ownership |
| O-034 | P1 | `authBase/core/services/token_manager.go:84-104,117-137` rechaza `kid` iguales, pero no material HMAC/Ed25519 igual con IDs distintos | Access y refresh pueden terminar compartiendo capacidad de firma, incumpliendo aislamiento criptográfico | Cerrado | Rechazar cualquier material compartido entre los key rings de access y refresh, con tests |
| O-035 | P1 | `authBase/docs/MIGRATION_V4.md:7-14`, `CHANGELOG.md:57-74,123-127` y `tools/verify-published-module.sh:4`; el remoto y proxy público no resuelven `v4.0.0`, pero sí `v4.0.1+` | La guía y el smoke default apuntan a una versión inexistente; el registro de release es contradictorio | Cerrado | Documentar el baseline publicado real, eliminar referencias ejecutables rotas y no mover/publicar tags sin autorización |
| O-036 | P2 | `base-golang/internal/infra/secondary/oidcclient/client.go:97-120` delega firma/audience/exp al verifier, pero no exige `sub` ni valida `iat`; los tests OIDC sólo cubren happy path, nonce e issuer | Claims obligatorios y regresiones de firma/audience/exp/rate limit no quedan caracterizados | Cerrado | Validación explícita de `sub`/`iat` y matriz local sin red para claims, replay, expiración, timeouts y throttling |
| O-037 | P2 | `git status --short` en base-golang muestra `plans/012-*.md`–`017-*.md` sin seguimiento mientras `plans/README.md` los enlaza | Los enlaces del índice se rompen en clones y los checkboxes no reflejan el estado revisado | Cerrado | Versionar los planes, reconciliar sus criterios y mantener el índice como fuente coherente |
| O-038 | P2 | `base-golang/go.mod:9,13` y `initModules/go.mod:8`; `go list -m -u` ofrece `initModules/v2.0.1`, `go-jose/v4.1.5` y `properties/v1.18.12`; `initModules` carece del checksum `/go.mod` de `kr/pretty` | Quedan patches directos y la inspección `go list -m -u all` de initModules falla antes de listar actualizaciones | Cerrado | Aplicar patches compatibles, completar checksums y ejecutar gates; publicar initModules sólo con autorización |
| O-039 | P2 | `base-golang/internal/infra/primary/httpapi/map.go:25-26` habla de “complexity requirements” después de eliminar reglas de composición | El mensaje público contradice la política de passphrases/blocklist del plan 014 | Cerrado | Mensaje neutral y prueba de contrato HTTP actualizada |

## Estado actual validado

- `authBase` pasa `make verify`, incluido race, lint, govulncheck, quickstart y
  Gitleaks; un consumidor externo resuelve `authBase/v4@v4.3.0` desde el proxy.
- `base-golang` pasa unitarias, race, vet/staticcheck, govulncheck, OpenAPI,
  migraciones up/down/up y pruebas PostgreSQL. Sólo Gitleaks deja rojo el gate.
- `initModules` pasa fmt, unitarias, race, vet, golangci-lint, govulncheck y el
  ejemplo standalone; no define un target `make verify`, su gate se llama `ci`.
- La carrera refresh/revocación pasó 100 repeticiones contra PostgreSQL efímero.
- No se encontraron vulnerabilidades alcanzables con govulncheck en los tres
  repositorios.

## Comandos necesarios

| Propósito | Comando | Resultado esperado |
|---|---|---|
| Auth completo | `cd /Users/wilsonsayago/workspace/golang/authBase && GOWORK=off make verify` | Sale 0 |
| Auth consumidor | `cd /Users/wilsonsayago/workspace/golang/authBase && GOWORK=off make consumer-published AUTHBASE_VERSION=v4.3.0` | Resuelve desde module cache y sale 0 |
| Base completo | `cd /Users/wilsonsayago/workspace/golang/base-golang && DATABASE_TEST_URL='<dsn-postgres-efimero>' GOWORK=off make verify` | Sale 0, incluido Gitleaks y DB |
| InitModules | `cd /Users/wilsonsayago/workspace/golang/initModules && GOWORK=off make ci` | Sale 0 |
| Actualizaciones | `GOWORK=off go list -m -u -mod=readonly all` en cada repo | Sale 0; sin patches directos seleccionados pendientes |

## Alcance

**Dentro de alcance**:

- base-golang: archivos citados en O-032/O-033/O-036/O-037/O-038/O-039,
  pruebas asociadas, `plans/README.md` y planes 012–017.
- authBase: constructores y tests de token crypto; documentación/herramienta de
  release; `go.mod/go.sum` sólo si `tidy` lo requiere.
- initModules: `go.mod`, `go.sum` y documentación de versión si se adopta un
  patch directo.

**Fuera de alcance**:

- Reescribir historia Git, mover/reutilizar tags o publicar releases.
- Cambiar contratos HTTP, claims JWT u OIDC exitosos.
- Añadir proveedores OIDC, auto-registro o auto-link por email.
- Introducir KMS, Redis u otro servicio operativo.
- Actualizar masivamente dependencias transitivas no alcanzables.

## Flujo Git

- base-golang: `codex/018-cierre-012-017`.
- authBase: `codex/018-cierre-012-017`.
- initModules: `codex/018-dependency-hygiene`, sólo si se modifica.
- Commits separados por observación: `ci:`, `fix:`, `security:`, `test:`,
  `docs:` y `deps:`. No push/tag/release sin autorización.

## Pasos

### Paso 1: restaurar el gate de secretos sin ocultar hallazgos reales

No reescriba el commit histórico. Añada una excepción que coincida sólo con el
sentinel inequívocamente falso de la fixture O-032; no permita todo el archivo,
el directorio `internal/`, documentación ni planes. Extienda
`scripts/secrets-smoke.sh` para demostrar que la fixture conocida es aceptada y
que otra clave de prueba sin ese sentinel sigue fallando.

Si se usa baseline en lugar de regex, agregue sólo el fingerprint exacto y
mantenga valores redacted. Nunca copie material de credenciales al plan, logs o
mensajes de commit.

**Verificar**:

```bash
cd /Users/wilsonsayago/workspace/golang/base-golang
GOWORK=off make secrets
```

Resultado esperado: smoke y escaneo de historia/worktree salen 0; una fixture
negativa independiente todavía es detectada.

### Paso 2: cerrar pools poseídos cuando falle el composition root

En `bootstrap.NewApplication`, distinga explícitamente si el pool fue abierto por
el constructor o inyectado por el caller. Mientras la construcción no haya
terminado, un defer debe cerrar únicamente el pool poseído ante cualquier error
de DSN, migración, crypto, OIDC, servicios o wiring. Al retornar la Application,
transfiera ownership al `DatabaseLifecycle` y desactive ese cleanup.

No cierre `Options.DB` en errores: es propiedad del caller. Añada pruebas para
ambos casos y para un fallo posterior a abrir DB (por ejemplo configuración de
claves inválida) usando sólo PostgreSQL efímero.

**Verificar**: `GOWORK=off go test -race -count=20 ./internal/infra/bootstrap` →
sale 0 sin crecimiento de conexiones ni contaminación entre instancias.

### Paso 3: hacer real la separación criptográfica access/refresh

En authBase, valide todos los keys activos y anteriores como dos conjuntos:

- ningún secreto HMAC de access puede ser byte-a-byte igual a uno de refresh;
- ninguna clave pública Ed25519 derivada de access puede ser igual a una de
  refresh, aunque cambien path o `kid`;
- los mensajes de error no incluyen material, paths sensibles ni hashes;
- se conservan constructor y tokens HMAC legacy compatibles cuando las claves
  son distintas.

En base-golang rechace además paths activos textualmente iguales como error
temprano, pero mantenga la validación criptográfica de authBase porque symlinks o
copias pueden contener la misma clave.

Añada tests para active/active, previous/active y previous/previous compartidos,
con `kid` distintos, tanto HMAC como Ed25519.

**Verificar**:

```bash
cd /Users/wilsonsayago/workspace/golang/authBase
GOWORK=off go test -race -count=1 ./core/services
GOWORK=off make verify
```

Resultado esperado: configuraciones compartidas fallan cerrado; compatibilidad
HMAC y rotación válida siguen verdes.

### Paso 4: reconciliar la línea v4 publicada

Compruebe de nuevo remoto y proxy. No intente crear `v4.0.0`. Actualice la guía
de migración para usar una versión publicada soportada y explique brevemente que
el primer baseline resoluble fue `v4.0.1`. Corrija links/comparaciones del
changelog que dependan de la etiqueta ausente. Haga que
`verify-published-module.sh` exija una versión explícita o use un default
publicado vigente, y pruebe que un valor inexistente falla claramente.

**Verificar**:

```bash
cd /Users/wilsonsayago/workspace/golang/authBase
GOWORK=off make consumer-published AUTHBASE_VERSION=v4.3.0
git ls-remote --tags origin 'v4*'
```

Resultado esperado: consumidor verde; docs no ordenan instalar `v4.0.0`; no se
creó ni movió ninguna etiqueta.

### Paso 5: completar validación y caracterización OIDC

Después de `IDTokenVerifier.Verify`, exija subject no vacío e `iat` presente y no
futuro fuera de una tolerancia documentada. Mantenga issuer, audience, firma,
expiración y nonce como validaciones obligatorias. No añada llamadas a Google en
tests.

Amplíe el proveedor `httptest` para cubrir firma, audience, expiry, subject e
issued-at inválidos; agregue timeout/cancelación. En servicio/DB cubra usuario
inactivo, grants expirados y doble link concurrente. En HTTP cubra 429 en login,
callback y exchange OIDC, cookie segura y ausencia de tokens en `Location`/logs.

**Verificar**:

```bash
cd /Users/wilsonsayago/workspace/golang/base-golang
GOWORK=off go test -race -count=1 ./internal/core/services ./internal/infra/secondary/oidcclient ./internal/infra/primary/controller
DATABASE_TEST_URL='<dsn-postgres-efimero>' GOWORK=off go test -race -count=1 ./internal/infra/secondary/repositories/postgres/oidcstore
```

Resultado esperado: todos los casos locales pasan y ninguna prueba necesita red.

### Paso 6: alinear mensajes, planes y patches directos

- Cambie el mensaje de password débil por uno neutral que no prometa reglas de
  composición; actualice el test/contrato correspondiente.
- Versione los planes 012–017 y deje sus checkboxes/estado consistentes con la
  evidencia final. No marque 017 verificado hasta que Gitleaks y separación de
  claves pasen.
- Actualice base-golang a `initModules/v2.0.1` y `go-jose/v4.1.5` si sus gates
  siguen verdes.
- En initModules, actualice `properties` al patch compatible vigente y complete
  checksums para que `go list -m -u -mod=readonly all` no falle. No publique una
  nueva versión como parte implícita de este paso.

No acepte automáticamente todos los upgrades transitivos mostrados por `-u`.

### Paso 7: ejecutar cierre multirrepositorio

Ejecute los gates de la tabla. En base-golang use únicamente la DB de
`docker-compose.test.yml` o una equivalente inequívocamente efímera. Confirme
árboles limpios salvo los cambios de este plan y actualice las filas 012–018 del
índice con commits y evidencia real.

## Plan de pruebas

- Secret scanning: falsa positiva histórica estrechamente aceptada y control
  negativo aún bloqueado.
- Bootstrap: pool propio cerrado en cada error; pool inyectado nunca cerrado por
  el constructor; success transfiere ownership al lifecycle.
- Crypto: misma clave con diferentes `kid` rechazada entre access/refresh;
  rotación válida y legacy siguen funcionando.
- OIDC: matriz completa descrita en el paso 5, sin Google/red externa.
- Release: consumidor temporal de v4.3.0 y fallo explícito de versión ausente.
- Dependencias: `go mod verify`, `go list -m -u -mod=readonly all`, race, lint y
  govulncheck en cada repo afectado.

## Criterios de terminado

- [x] `base-golang make verify` sale 0 contra PostgreSQL efímero, incluido Gitleaks.
- [x] Fallos de construcción de Application no filtran pools poseídos.
- [x] authBase rechaza material compartido entre access y refresh con IDs distintos.
- [x] Docs/herramientas no presentan `v4.0.0` como versión instalable.
- [x] OIDC exige `sub`/`iat` y tiene la matriz de regresión sin red.
- [x] Los planes 012–017 están versionados y sus estados coinciden con evidencia.
- [x] Patches directos seleccionados están aplicados o documentados como diferidos.
- [x] Los gates de authBase e initModules salen 0.
- [x] No se creó, movió ni publicó ningún tag/release.

## Condiciones de parada

- Resolver O-032 parece requerir excluir un directorio completo o reescribir
  historia Git.
- No puede distinguir ownership del pool sin cambiar una API pública.
- La separación de keys exige romper constructores v4 existentes en lugar de
  añadir validación compatible.
- Se propone relajar issuer, audience, firma, nonce o PKCE para arreglar tests.
- Un patch directo introduce breaking changes o exige una major.
- La DB disponible no es inequívocamente efímera.
- Se requiere publicar/mover tags o hacer push sin autorización humana.

## Notas de mantenimiento

- Mantenga el test de keys iguales: `kid` identifica una clave, no demuestra que
  dos archivos/secretos sean distintos.
- Cada nueva fixture de seguridad debe pasar Gitleaks sin allowlists amplias.
- Los tests OIDC deben usar discovery/JWKS locales y verificar claims negativos,
  no sólo happy paths.
- Si initModules se publica después, repita consumidor externo antes de actualizar
  base-golang a esa nueva versión.
