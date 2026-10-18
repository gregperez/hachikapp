# Plan 008 — Estabilizar contrato HTTP y OpenAPI

## Objetivo

Eliminar panics y respuestas ambiguas en la frontera HTTP, definir errores/paginación consistentes y hacer que OpenAPI describa exactamente las rutas, seguridad y DTOs expuestos.

- Prioridad: P1
- Esfuerzo: L
- Depende de: planes 004, 006 y 007
- Rama sugerida: `codex/008-contrato-http`
- Commit analizado: `99a9826`

## Estado actual verificado

El binding fuerza una aserción que puede panic ante JSON malformado en `base_controller.go`:

```go
validationErrors := err.(validator.ValidationErrors)
```

La traducción usa `MustLocalize`, que también puede panic si el ID no está registrado. La mayoría de errores termina como 400 sin distinguir 401, 403, 404, 409 y 500. `PermissionDto` declara tags `bson` en vez de `json`. La paginación ignora errores de parseo y no limita tamaño.

La documentación Swagger tiene inconsistencias verificadas:

- `basePath` incluye `/v1/auth` aunque varias rutas ya documentan `/v1/...`.
- Se referencia `ApiKeyAuth` sin `securityDefinitions` correspondiente.
- Rutas de roles omiten seguridad.
- Las anotaciones de cambio de estado no coinciden con el router.
- DTOs generados están obsoletos; `RoleDto` queda vacío y una respuesta de usuario expone el campo password en el esquema.
- Swagger se registra siempre.

## Control de deriva

```bash
git rev-parse --short HEAD
git status --short
rg -n 'handleBind|MustLocalize|validator.ValidationErrors|strconv.Atoi|pageSize|bson:' internal/infra/primary
rg -n '@Router|@Security|BasePath|securityDefinitions|ApiKeyAuth' cmd internal docs
```

## Contrato objetivo

Errores usan `application/problem+json` o un envelope equivalente estable:

```json
{
  "type": "https://example.invalid/problems/validation",
  "title": "Validation failed",
  "status": 422,
  "code": "VALIDATION_ERROR",
  "detail": "Request contains invalid fields",
  "requestId": "...",
  "errors": [{"field": "username", "code": "required"}]
}
```

No incluir stack, SQL, DSN, hash, token ni error interno. Mapeo mínimo:

| Condición | Estado |
|---|---:|
| JSON sintácticamente inválido / query inválida | 400 |
| Credencial/token ausente o inválido | 401 |
| Identidad válida sin permiso | 403 |
| Recurso ausente | 404 |
| Conflicto/unique/replay relevante | 409 |
| Validación semántica | 422 |
| Error inesperado | 500 |

## Alcance

- Binding y validación sin panic.
- Taxonomía de errores y middleware central de respuesta/recovery.
- DTOs request/response separados y tags JSON correctos.
- Paginación validada, acotada y con orden estable.
- Uso del helper de identidad v3, sin assertions crudas.
- Anotaciones/regeneración OpenAPI y test de drift.
- Seguridad Bearer definida y aplicada; Swagger configurable según plan 005.
- Tests handler/contract.

## Fuera de alcance

- Cambiar reglas RBAC; plan 004.
- Cambiar consultas/constraints; planes 006/007.
- Versionar una API v2 completa. Se permiten correcciones incompatibles necesarias para seguridad, documentadas.

## Implementación

1. Reemplazar assertions de validator por `errors.As`. Distinguir `json.SyntaxError`, type mismatch, EOF, body demasiado grande, campo desconocido y `validator.ValidationErrors`.
2. Configurar decoder para rechazar campos desconocidos en requests sensibles y limitar body con middleware/reader.
3. Reemplazar `MustLocalize` por lookup seguro. Si falta traducción, usar un mensaje genérico y registrar sólo el ID técnico.
4. Definir errores de aplicación tipados/códigos estables y un único mapper HTTP. Los repositorios no deciden status.
5. Recovery captura panic inesperado, asigna request ID, registra stack internamente y devuelve 500 genérico. Tests deben confirmar que errores de usuario no llegan a recovery.
6. Separar modelos internos de DTOs públicos. Ninguna respuesta de usuario incluye password/hash, refresh token almacenado o campos internos.
7. Corregir `PermissionDto` a `json` y revisar todos los tags con tests de marshaling.
8. Paginación: defaults explícitos, enteros positivos, `pageSize` máximo (por ejemplo 100), overflow rechazado, metadata consistente y orden determinista. El valor final debe documentarse.
9. Obtener identidad con `authBase/v3` mediante el adaptador del plan 002; ausencia/type mismatch retorna error controlado.
10. Corregir anotaciones y regenerar docs desde fuente. Definir Bearer JWT:

```text
@securityDefinitions.apikey BearerAuth
@in header
@name Authorization
```

Documentar que el valor es `Bearer <token>` y aplicar `@Security BearerAuth` a cada ruta privada.
11. Elegir un único base path real. No duplicar `/v1/auth` en `BasePath` y `@Router`.
12. Añadir test que genere OpenAPI en CI y falle si `git diff --exit-code` detecta drift; validar el documento con un linter OpenAPI.

## Pruebas obligatorias

- Body vacío, JSON truncado, tipo incorrecto, campo desconocido, múltiples errores validator y body excesivo: respuesta estable, nunca panic.
- Traducción faltante no panic.
- Tabla de errores cubre todos los status anteriores y redacción de 500.
- Serialización de usuario jamás contiene `password`/hash.
- Paginación inválida y superior al máximo se rechaza; default y límite se aplican.
- Cada endpoint privado del spec declara Bearer; endpoints públicos esperados no.
- Spec contiene rutas/métodos reales, schemas no vacíos y no duplica prefijos.
- Swagger no está accesible cuando su flag está apagado.

## Comandos de verificación

```bash
make fmt-check
make lint
make test
DATABASE_TEST_URL='<dsn-efimero>' make test-db
make openapi-generate
make openapi-lint
git diff --exit-code -- docs
make verify
```

Añadir búsquedas de protección:

```bash
rg -n 'err\.\(validator\.ValidationErrors\)|MustLocalize|bson:' internal/infra/primary
rg -n '"password"' docs
```

La primera no debe devolver usos productivos; la segunda no debe mostrar password en schemas de respuesta.

## Criterios de aceptación

- [x] Ningún input inválido conocido provoca panic.
- [x] Existe un mapper único con status/códigos estables y 500 redactado.
- [x] Requests y responses usan DTOs distintos; credenciales nunca aparecen en responses/spec.
- [x] Paginación es acotada, validada y determinista.
- [x] OpenAPI coincide con el router y declara Bearer en todas las rutas privadas.
- [x] La generación/lint de OpenAPI bloquea drift en CI.
- [x] Swagger respeta configuración y ambiente.

## Condiciones de parada

- Detenerse si consumidores dependen de un payload inseguro o status incorrecto; documentar breaking change y coordinar versión, pero no reintroducir exposición.
- Detenerse si la herramienta Swagger genera un schema con password pese a DTOs separados; corregir el tipo fuente, no editar sólo el archivo generado.
- No traducir detalles internos de DB/JWT directamente al cliente.

## Notas de mantenimiento

- **Breaking change (errores):** las fallas HTTP pasan a `application/problem+json` con códigos estables (`VALIDATION_ERROR`, `UNAUTHORIZED`, …). Las respuestas 2xx conservan `dto.ResponseDto`.
- Paginación: default `page=1`, `pageSize=10`, máximo `pageSize=100` (rechazo 400 si se excede).
- OpenAPI: usar siempre `make openapi-generate` (swag pinneado en `tools/versions.env`); un `swag` global más nuevo puede generar `docs.go` incompatible con `github.com/swaggo/swag` del módulo.
- Todo error público nuevo necesita código estable, status, test y traducción opcional.
- Todo endpoint nuevo debe quedar en router, OpenAPI y test de autorización.
- Tratar el spec generado como artefacto verificable, no como fuente manual.
