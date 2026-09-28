# Plan 012: Publicar authBase v4 y restaurar builds reproducibles

> **Instrucciones para el agente ejecutor**: Este plan toca dos repositorios. Ejecute
> los pasos en orden y no publique etiquetas, releases ni commits sin autorización
> explícita del responsable. Si ocurre una condición de parada, reporte el estado y
> no improvise. Al finalizar, actualice la fila 012 de `base-golang/plans/README.md`.
>
> **Control de deriva (ejecutar primero)**:
>
> ```bash
> git -C /Users/wilsonsayago/workspace/golang/authBase diff --stat b5c0a2c..HEAD -- go.mod README.md CHANGELOG.md docs/RELEASE_CHECKLIST.md .github
> git -C /Users/wilsonsayago/workspace/golang/base-golang diff --stat 59aeb1f..HEAD -- go.mod go.sum README.md .github/workflows/ci.yml docs/dependency-policy.md
> ```
>
> Si cambió cualquiera de esos archivos, compare el estado vivo con este plan antes
> de editar. Una etiqueta remota `v4.0.0` nueva o distinta es condición de parada.

## Estado

- **Estado**: Implementado — primer módulo proxy-resolvable `v4.0.1` (O-035 cerrado en 018)
- **Prioridad**: P0
- **Esfuerzo**: M
- **Riesgo**: MED
- **Depende de**: ninguno
- **Categoría**: migration / dx / security
- **Planeado en**: `base-golang@59aeb1f`, `authBase@b5c0a2c`, 2026-09-25

## Por qué importa

`base-golang` declara `authBase/v4 v4.0.0`, pero reemplaza el módulo por el
directorio hermano `../authBase`. El remoto de authBase no anunciaba `v4.0.0` al
redactar este plan. Un checkout aislado y la CI de base-golang no pueden resolver
esa ruta, por lo que la plantilla no es reproducible para terceros.

## Estado actual

- `base-golang/go.mod:8,73` contiene:

  ```go
  require github.com/WilsonSayago/authBase/v4 v4.0.0
  replace github.com/WilsonSayago/authBase/v4 => ../authBase
  ```

- `base-golang/.github/workflows/ci.yml:53-78` hace un único checkout y luego
  ejecuta `make verify`; no obtiene el repositorio hermano.
- `authBase/README.md:21-28,121-126` indica que la etiqueta v4 todavía no está
  publicada y recomienda temporalmente `@main`.
- `authBase/docs/RELEASE_CHECKLIST.md` exige validación humana, CI verde, build
  de un consumidor externo y verificación mediante el proxy de módulos.
- Existe una etiqueta local `v4.0.0`; nunca debe moverse ni recrearse. Primero
  confirme el commit al que apunta y si existe remotamente.
- Convención de commits observada: Conventional Commits, por ejemplo
  `feat!: add cursor pagination and publish v4` y `docs: align v4 guides`.

## Comandos necesarios

| Propósito | Comando | Resultado esperado |
|---|---|---|
| Ver etiqueta local | `git -C /Users/wilsonsayago/workspace/golang/authBase show-ref --tags -d | grep 'refs/tags/v4.0.0'` | Muestra etiqueta y, si es anotada, su commit dereferenciado |
| Ver etiquetas remotas | `git -C /Users/wilsonsayago/workspace/golang/authBase ls-remote --tags origin 'v4*'` | Antes de publicar, no aparece una etiqueta conflictiva |
| Auth unit | `cd /Users/wilsonsayago/workspace/golang/authBase && GOWORK=off go test -count=1 ./...` | Sale 0 |
| Auth race | `cd /Users/wilsonsayago/workspace/golang/authBase && GOWORK=off go test -race -count=1 ./...` | Sale 0 |
| Auth vet | `cd /Users/wilsonsayago/workspace/golang/authBase && GOWORK=off go vet ./...` | Sale 0 |
| Base completa | `cd /Users/wilsonsayago/workspace/golang/base-golang && DATABASE_TEST_URL='<dsn-postgres-efimero>' GOWORK=off make verify` | Sale 0; usar sólo DB efímera |

## Alcance

**Dentro de alcance**:

- En `authBase`: `.github/`, `Makefile`, `tools/`, `.gitleaks.toml`,
  `docs/RELEASE_CHECKLIST.md`, `README.md`, `CHANGELOG.md` y, sólo si hace falta
  para gates, archivos de configuración de herramientas.
- En `base-golang`: `go.mod`, `go.sum`, `README.md`, `docs/dependency-policy.md`,
  `.github/workflows/ci.yml` y pruebas/scripts de reproducibilidad.
- Publicar exactamente la etiqueta ya preparada sólo tras autorización humana.

**Fuera de alcance**:

- Cambiar la API pública de authBase o su comportamiento de autenticación.
- Mover o reutilizar una etiqueta existente.
- Publicar una nueva major/minor para evitar investigar la etiqueta actual.
- Introducir `go.work` como requisito de compilación.
- Modificar `initModules`.

## Flujo Git

- Rama authBase: `codex/012-publicar-v4`.
- Rama base-golang: `codex/012-consumir-authbase-v4`.
- Commits pequeños con prefijos `ci:`, `docs:` y `deps:`.
- No ejecutar `git push`, `gh release create` ni publicar tags sin permiso.

## Pasos

### Paso 1: auditar la identidad de la etiqueta

Registre el commit dereferenciado de la etiqueta local, confirme que pertenece a
la línea v4 y compare su `go.mod`, API y pruebas con lo que consume base-golang.
Consulte el remoto. Si `v4.0.0` ya existe remotamente, su commit debe ser idéntico.

**Verificar**:

```bash
git -C /Users/wilsonsayago/workspace/golang/authBase show v4.0.0^{commit} --no-patch --format='%H %s'
git -C /Users/wilsonsayago/workspace/golang/authBase ls-remote --tags origin 'v4.0.0*'
```

Resultado esperado: una única identidad local; remoto vacío o idéntico.

### Paso 2: homologar el gate de authBase

Añada un `Makefile` y versiones de herramientas siguiendo el patrón de
`base-golang/Makefile` e `initModules/Makefile`. El target `verify` debe cubrir:
`gofmt` check, `go vet`, tests, race, Staticcheck o golangci-lint,
`govulncheck`, `go mod verify`, build/test del quickstart y Gitleaks. Añada
Dependabot semanal para módulos Go y GitHub Actions. Fije acciones por SHA y
documente las versiones legibles en comentarios.

No convierta avisos de herramientas en exclusiones amplias. Ningún gate debe
imprimir secretos o tokens.

**Verificar**: `cd /Users/wilsonsayago/workspace/golang/authBase && GOWORK=off make verify` → sale 0.

### Paso 3: probar el módulo como consumidor externo

Cree un módulo temporal fuera de ambos repositorios, importe los constructores
públicos usados por `examples/quickstart` y resuelva `authBase/v4@v4.0.0` sin
`replace`. Antes de publicar, esta prueba puede usar un proxy local o quedar
preparada como script que se ejecuta después del checkpoint humano; nunca dé un
falso verde usando `../authBase`.

**Verificar después de publicar**:

```bash
GOWORK=off go list -m -json github.com/WilsonSayago/authBase/v4@v4.0.0
```

Resultado esperado: `Version` es `v4.0.0` y `Dir` apunta al caché de módulos, no
al workspace.

### Paso 4: checkpoint de publicación

Presente al responsable: commit de la etiqueta, gates ejecutados, diff de docs y
resultado del consumidor. Espere autorización explícita. Con autorización,
publique la etiqueta inmutable y verifique proxy/checksum DB. Si no hay
autorización, marque el plan `BLOCKED (esperando publicación manual)`.

**Verificar**: `git ls-remote --tags origin 'v4.0.0*'` → etiqueta y dereferencia
coinciden con el commit aprobado.

### Paso 5: eliminar el reemplazo en base-golang

Sólo tras comprobar el proxy, elimine el `replace`, ejecute `go mod tidy` y
actualice README/política. Añada a CI una comprobación que falle si `go.mod`
contiene un `replace` hacia `../` o una ruta absoluta local.

**Verificar**:

```bash
cd /Users/wilsonsayago/workspace/golang/base-golang
test -z "$(go mod edit -json | sed -n '/"Replace"/,/]/p' | grep 'Old\|New')"
GOWORK=off go mod verify
GOWORK=off go list -m -mod=readonly all
```

Resultado esperado: sale 0 y authBase se resuelve desde el módulo publicado.

### Paso 6: verificar ambos repositorios

Ejecute el gate de authBase y `make verify` de base-golang contra PostgreSQL
efímera. Confirme que la CI no depende del directorio padre.

## Plan de pruebas

- Script en base-golang que detecte `replace` locales y tenga fixture positivo y
  negativo.
- Smoke consumer en authBase que compile la API pública desde un módulo temporal.
- Gates existentes de ambos repositorios.
- No use la presencia del repositorio hermano como evidencia de éxito.

## Criterios de terminado

- [x] El primer módulo v4 proxy-resolvable es `v4.0.1`; `v4.0.0` no es instalable.
- [x] `authBase` tiene un `make verify` reproducible y verde.
- [x] `base-golang/go.mod` no contiene reemplazos locales.
- [x] `GOWORK=off go list -m -mod=readonly all` funciona desde base-golang.
- [x] Un consumidor temporal compila usando una versión publicada (`v4.3.0`).
- [x] README, changelog y checklist describen el estado real (cierre O-035).

## Condiciones de parada

- La etiqueta remota existe y apunta a otro commit.
- La etiqueta local fue movida desde una publicación previa.
- El proxy devuelve otro checksum o una versión distinta.
- Hace falta modificar API/código productivo para publicar la etiqueta preparada.
- No hay autorización explícita para publicar.
- La base PostgreSQL disponible no es inequívocamente efímera.

## Notas de mantenimiento

- Cada release de authBase debe probar un consumidor sin `replace`.
- Un `go.work` puede usarse en desarrollo, pero debe permanecer opcional.
- Revise especialmente que los nuevos gates no dupliquen descargas sin checksum
  ni introduzcan permisos de GitHub Actions innecesarios.
