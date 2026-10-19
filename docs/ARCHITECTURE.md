# Arquitectura de base-golang

`base-golang` es una API HTTP de autenticación, usuarios y roles construida con
arquitectura hexagonal. El núcleo depende de contratos; Gin, PostgreSQL,
`authBase/v4` e `initModules/v2` se conectan en los adaptadores y el bootstrap.

## Capas y dependencias

- `cmd/`: entrypoints del servidor y del bootstrap administrativo.
- `internal/core/domain/`: modelo de usuario, errores y reglas puras.
- `internal/core/ports/`: contratos propios de persistencia y auditoría.
- `internal/core/services/`: casos de uso de usuario y bootstrap de
  administrador.
- `internal/infra/primary/`: controllers Gin, DTOs, middleware y traducción del
  contrato HTTP.
- `internal/infra/secondary/repositories/`: implementación de ports,
  PostgreSQL DAOs y `RefreshTokenStore`.
- `internal/infra/bootstrap/`: composition root aislado (`Application`),
  engine HTTP, health probes y adapters `Lifecycle`.
- `internal/resources/sql/migrations/`: evolución incremental del esquema.

La dirección principal es:

```text
HTTP/Gin → controllers → use cases/ports → repositories/DAO → PostgreSQL
```

El dominio no conoce Gin, pgx ni detalles de JWT. Los controllers dependen de
interfaces de casos de uso y traducen errores a `application/problem+json`.

## Arranque y apagado

`cmd/main.go` carga configuración, crea un logger y construye un
`bootstrap.Application` aislado. El `initModules.App` de esa instancia registra
lifecycles en este orden:

1. Base de datos: ping al iniciar, close al detener.
2. Traducciones: carga de locales.
3. Limpieza de sesiones expiradas del plan 013.
4. Limpieza de transacciones/grants OIDC (si el login federado está habilitado).
5. HTTP: listen al iniciar; readiness false y `Shutdown` al detener.

`RunWithSignals` bloquea hasta una señal y detiene en orden inverso
(HTTP → workers → DB) con un contexto de stop independiente del contexto
cancelado. La decisión vigente está en
[ADR 001](adr/001-initmodules-lifecycle.md).

## Autenticación y autorización

`authBase/v4` aporta contratos y servicios; la aplicación aporta los
adaptadores:

- `UserRepository` implementa lectura de identidad y credenciales.
- `refreshstore.Store` persiste hashes, rota atómicamente, revoca por usuario y
  purga expiradas en lotes limitados.
- `ginadapter.Context` adapta Gin al contexto esperado por autorización.
- `AuthenticationService` implementa login, emisión privilegiada, refresh y
  revocación de familia/usuario.
- El login federado OIDC vive en `base-golang`: un adaptador provider-neutral
  verifica Authorization Code + PKCE, resuelve `(issuer, subject)` y llama
  `EstablishSession`. No hay auto-registro ni auto-enlace por email. El
  callback redirige con un grant opaco; los JWT locales salen solo de
  `POST /v1/auth/oidc/google/exchange`.
- `Authorization` aplica `AuthorizeJWT` y `PoliciesGuard`.
- `RoleService` usa el `RolePort` PostgreSQL de la aplicación.
- El signing HMAC sigue siendo el default. Ed25519 se activa solo con
  `JWT_ALG=ed25519` y claves de archivo; `/.well-known/jwks.json` publica
  únicamente claves públicas de access. Los eventos tipados de authBase se
  escriben a slog sin JWT, secretos ni PII cruda.

Rutas públicas:

- `POST /v1/auth/login`
- `POST /v1/auth/refresh-token`
- `POST /v1/auth/logout`
- `GET /v1/auth/oidc/google/login`
- `GET /v1/auth/oidc/google/callback`
- `POST /v1/auth/oidc/google/exchange`
- `GET /health/live`
- `GET /health/ready`
- `GET /.well-known/jwks.json` — solo si hay claves públicas de access
- `GET /v1/public/tags/{code}` y `GET /v1/public/tags/{code}/photo`
- `POST /v1/public/tags/{code}/scans`
- `POST /v1/public/scans/{scanId}/messages` y `.../contact` (cabecera
  `X-Scan-Token`)

Las rutas de usuarios y roles requieren access token; las operaciones
administrativas además requieren permisos por entidad y acción. `base-agent`
puede llamar `GET /v1/roles` con el access token del usuario; Go no confía en
el modelo y vuelve a aplicar `PoliciesGuard`.

## Fichas de mascotas

`/v1/pets` expone las fichas de la cuenta autenticada. No usa `PoliciesGuard`
por entidad: como `/v1/users/me/sessions`, el recurso pertenece al usuario y
`PetService` filtra siempre por `owner_id`; una ficha ajena responde 404.

- `domain/pet.go` normaliza y valida (errores por campo con código estable) y
  define la visibilidad pública por campo con los valores por defecto del
  manual de marca. Dirección exacta, chip, teléfonos y fecha de nacimiento no
  son campos públicos.
- `PetPort` (`petstore.Store`) persiste la ficha con versión optimista.
  El número de chip es único por cuenta (no global) para evitar que alguien
  registre el chip ajeno y para no revelar si un chip existe; los duplicados
  entre cuentas quedarán para un flujo de transferencia.
- Las fotos se re-codifican en el servidor (`domain.SanitizePetPhoto`): se
  aplica la orientación EXIF y se descartan todos los metadatos (GPS incluido).
- `PetPhotoPort` (`petstore.PhotoStore`) guarda los bytes en `pet_photos`; la
  ficha solo conoce `photo_updated_at`, así que la foto puede migrar a S3/GCS
  cambiando el adaptador.

## Placas y ficha pública

Una placa (`pet_tags`, física o digital) enlaza a `PUBLIC_BASE_URL/t/<code>`.
`TagService` genera lotes (admin) y activa, crea y desactiva placas (dueño);
`PublicTagService` atiende la página pública sin login y el historial de
escaneos del dueño. `tagstore.Store` implementa `TagPort` y `TagScanPort`.

- **Rutas públicas** (`PublicTagController`): sin JWT. Responden con
  `Cache-Control: no-store` y `X-Robots-Tag: noindex`, incluso en errores y
  429. El perfil sale solo de `domain.PublicProfileOf`: nunca lleva teléfono,
  dirección, número de chip ni apellidos. `/contact` es la única ruta que
  devuelve un teléfono (solo el principal y solo si el botón de contacto es
  visible). Placa, escaneo o token desconocidos responden el mismo 404.
- **Límites por IP**: un `ratelimit.Memory` propio (`ratelimit.PublicTagScopes`),
  separado del de login para que el tráfico anónimo no desaloje esas
  entradas. La clave es `KeyedHash(pepper, ip)`. Es memoria del proceso: **con
  varias instancias hay que reemplazarlo por Redis**.
- **IP del visitante**: las rutas públicas y el login llegan a través del
  servidor Next, que reenvía la IP del visitante en `X-Forwarded-For` (en el
  frontend, `CLIENT_IP_HEADER`). El backend solo lee esa cabecera si la
  conexión viene de `AUTH_TRUSTED_PROXY_CIDRS`, así que **esa lista debe
  incluir la dirección del servidor Next**; si está vacía, todos los
  visitantes comparten la misma cuota y en producción se registra un aviso al
  arrancar.
- **Dueño** (`TagController`): `/v1/pets/{id}/tags`, `/v1/tags/{code}/activate`
  y `/v1/pets/{id}/scans`, solo con JWT; la propiedad la valida el caso de uso.
  El historial nunca expone los hashes de visitante y token ni el prefijo IP.
- **Admin** (`TagAdminController`): `/v1/admin/tag-batches` con
  `PoliciesGuard` sobre la entidad `TAGS` (crear para POST, leer para GET).
  La migración `000020_tags_permission` da ese permiso al rol `admin`.

## Paginación keyset

Los listados `GET /v1/users` y `GET /v1/roles` aceptan `limit` y un `cursor`
opaco. El adaptador HTTP:

1. valida `limit` entre 1 y 100;
2. decodifica el cursor a `authBase/domain.CursorKey`;
3. llama al caso de uso con `PageRequest`;
4. consulta por `(created_at, id) DESC` usando `LIMIT + 1`;
5. devuelve `hasNext` y codifica `nextCursor` desde el último elemento.

No se usan offset, número de página ni conteos exactos.

## Persistencia y migraciones

PostgreSQL es la fuente de verdad para usuarios, roles, permisos, asociaciones
y sesiones refresh. Las migraciones se aplican en orden y no deben reescribirse
después de llegar a un entorno persistente. Los cambios correctivos se agregan
como migraciones forward-only; [ADR 002](adr/002-migration-baseline.md) explica
la política de baseline.

Los timestamps `created_at` y `updated_at` se administran en base de datos, lo
que mantiene consistente el keyset entre procesos.

Cambiar contraseña o desactivar una cuenta actualiza `users` y revoca sus
sesiones en una única transacción. El componente
`infra/lifecycle.ExpiredSessionPurger` ejecuta limpieza inicial y periódica con
retención configurable; su registro en `initModules.App` se difiere al plan 015
para preservar el orden de apagado antes de cerrar PostgreSQL.

## Observabilidad y límites operativos

- Logs estructurados JSON mediante `log/slog`.
- `X-Request-ID` se conserva o genera y se incluye en access logs/problemas.
- `/health/live` informa vida del proceso.
- `/health/ready` exige gate activo y `Ping` PostgreSQL con timeout.
- CORS, límites de body, recovery y timeouts HTTP se configuran antes de rutas.
- Swagger puede deshabilitarse por configuración.

## Diagramas

- [Mapa de arquitectura](diagrams/base-golang-architecture.html)
- [Arranque y apagado](diagrams/base-golang-startup.html)
- [Listado protegido con cursor](diagrams/base-golang-paginated-request.html)
- [Índice y regeneración](diagrams/README.md)

Los JSON Archify versionados junto a los HTML son la fuente editable de cada
diagrama.
