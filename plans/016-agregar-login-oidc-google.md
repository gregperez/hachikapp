# Plan 016: Agregar login federado con Google mediante OIDC

> **Instrucciones para el agente ejecutor**: Implemente un primer proveedor OIDC
> dentro de base-golang, no una librería Google independiente. Reutilice la
> frontera `EstablishSession` del plan 013. No auto-enlace cuentas por email ni
> exponga tokens en URLs. Actualice la fila 016 del índice.
>
> **Control de deriva**:
>
> ```bash
> git diff --stat 59aeb1f..HEAD -- go.mod internal/core internal/infra/primary internal/infra/secondary internal/resources/sql/migrations internal/infra/config docs
> git -C /Users/wilsonsayago/workspace/golang/authBase diff --stat b5c0a2c..HEAD -- core/services/authenticationservice.go core/authenticationusecase.go
> ```

## Estado

- **Estado**: Verificado — O-036 cerrado en 018 (`sub`/`iat` y matriz local)
- **Prioridad**: P1
- **Esfuerzo**: L
- **Riesgo**: HIGH
- **Depende de**: planes 012, 013, 014 y 015
- **Categoría**: direction / security
- **Planeado en**: `base-golang@59aeb1f`, `authBase@b5c0a2c`, 2026-09-25

## Por qué importa

La arquitectura puede aceptar identidades externas, pero hoy la única emisión
inicial está acoplada a password. “Login con Gmail” debe implementarse como Sign
in with Google usando OpenID Connect; no requiere Gmail API. El objetivo es un
adaptador OIDC seguro y provider-neutral en base-golang, extraíble sólo cuando
exista un segundo consumidor/proveedor.

## Estado actual

- `authBase` maneja login local, emisión y refresh; después del plan 013 debe
  exponer `EstablishSession` para sujetos ya autenticados.
- `base-golang/internal/resources/sql/migrations/000001...` guarda password
  `NOT NULL`; este plan no creará cuentas externas sin password.
- `users.email` es único pero no debe utilizarse como clave federada estable.
- `AuthController` sólo registra login, refresh y logout.
- Los tokens locales se devuelven en `dto.AuthDto`; nunca deben colocarse en la
  query de un redirect.

## Estándares obligatorios

- Authorization Code flow con PKCE S256.
- `state` y `nonce` criptográficos, de un solo uso, ligados al navegador.
- Redirect URI exacta y allowlist de destinos finales.
- Verificar discovery/issuer, firma JWKS, `aud`, `exp`, `iat`, `nonce` y, cuando
  se use email, `email_verified`.
- La identidad es `(issuer, subject)`; email es atributo mutable.
- Scopes mínimos: `openid email profile`; no solicitar Gmail API.
- No persistir access/refresh tokens de Google si no se usan APIs Google.

## Comandos necesarios

| Propósito | Comando | Resultado esperado |
|---|---|---|
| Módulos | `GOWORK=off go mod tidy && GOWORK=off go mod verify` | Sale 0; deps OIDC fijadas |
| Unit/race | `GOWORK=off go test -race -count=1 ./...` | Sale 0 sin red real |
| DB | `DATABASE_TEST_URL='<dsn-efimero>' GOWORK=off make test-db` | Migración up/down/up y tests pasan |
| OpenAPI | `make openapi-lint` | Sale 0, sin drift |

## Alcance

**Dentro de alcance**:

- Config OIDC validada y redacted.
- Adaptador genérico OIDC con Google como configuración inicial.
- Persistencia `external_identities` y transacciones/grants OIDC de un solo uso.
- Flujo link explícito para usuarios autenticados y login para identidades ya
  vinculadas.
- Rate limiting, auditoría, OpenAPI, tests con proveedor OIDC local falso.

**Fuera de alcance**:

- Auto-registro de usuarios Google-only.
- Auto-enlace por coincidencia de email.
- Gmail API, contactos, calendario o almacenamiento de tokens Google.
- Implicit flow, resource owner password grant o tokens en fragment/query.
- Crear `authbase-google`/`authbase-oidc` como repositorio nuevo.
- Microsoft, Apple, GitHub, SAML o passkeys.

## Flujo Git

- Rama: `codex/016-google-oidc`.
- Commits: config/modelo, persistencia/migración, cliente OIDC, endpoints,
  OpenAPI/docs/tests.

## Pasos

### Paso 1: añadir configuración OIDC

Extienda `appconfig.Config` con proveedores: nombre estable, issuer HTTPS,
client ID, client secret, redirect URI exacta, destinos finales permitidos y
timeouts. Secretos sólo por entorno. Producción debe rechazar HTTP, redirect
vacía, wildcards y destinos no allowlisted. Documente placeholders sin valores
reales.

### Paso 2: crear migración forward

Cree el siguiente número de migración disponible, sin editar historia, con:

- `external_identities`: `id`, `provider`, `issuer`, `subject`, `user_id`,
  `email_at_link`, timestamps y último login; unique `(issuer, subject)` y FK.
- `oidc_transactions`: hash del state, nonce, hash/verifier PKCE protegido,
  intención (`login`/`link`), actor opcional, expiry, consumed_at y resultado
  verificado/grant opaco.

No guarde state, nonce, code o verifier recuperables en logs. Si el verifier
debe recuperarse para el exchange servidor-servidor, cifre el registro con una
clave separada o almacénelo en una sesión server-side con controles equivalentes.

El down sólo elimina tablas nuevas. Añada índices de expiry para limpieza.

### Paso 3: definir puertos y servicios provider-neutral

En core/application defina modelos `ExternalPrincipal` e interfaces para:

- iniciar/consumir transacción OIDC;
- resolver `(issuer, subject)` a user ID;
- vincular identidad sólo a un actor autenticado;
- emitir sesión local vía `authBase.SessionIssuer`.

El core no debe importar paquetes Google, Gin ni librerías OIDC.

### Paso 4: implementar cliente OIDC

Use `github.com/coreos/go-oidc/v3/oidc` y `golang.org/x/oauth2`, fijando versiones
compatibles revisadas. Configure discovery contra issuer allowlisted, cliente
HTTP con timeouts y PKCE S256. Valide todos los claims obligatorios y rechace
algoritmos/issuers inesperados. No confíe en `hd` o email como identificador.

### Paso 5: implementar enlace explícito

Un usuario con sesión local inicia `/v1/auth/oidc/google/link`; el callback sólo
puede asociar el principal con ese actor y debe rechazar si `(issuer,subject)` ya
pertenece a otro usuario. Requerir reautenticación reciente o password actual es
recomendado para esta operación sensible.

No ofrezca link basado únicamente en demostrar control del email Google.

### Paso 6: implementar login y handoff sin tokens en URL

El inicio público crea state/nonce/PKCE y cookie opaca `HttpOnly`, `Secure` en
producción, `SameSite=Lax`, path limitado y TTL corto. El callback consume la
transacción una vez, resuelve la identidad y crea un grant local opaco de un
solo uso. Redirige sólo a un destino fijo/allowlisted con el grant, nunca con
JWT. Un endpoint POST consume el grant y llama `EstablishSession` para obtener
los tokens locales conforme al contrato existente.

### Paso 7: aplicar controles comunes

Reutilice el rate limiter del plan 014 en inicio, callback y exchange. Registre
eventos tipados de inicio, éxito, fallo, link y conflicto sin tokens/PII cruda.
Limpie transacciones/grants expirados mediante el App Lifecycle del plan 015.

### Paso 8: OpenAPI y operación

Documente endpoints, cookies, 302/400/401/409/429/503, configuración, rotación
del client secret y procedimiento para desvincular. Deje claro que la pérdida
del último autenticador requiere un proceso separado de recuperación.

## Plan de pruebas

- Proveedor OIDC `httptest` con discovery, JWKS y token endpoint locales; nunca
  contacte Google en CI.
- Casos: happy login/link; state, nonce, issuer, audience y firma inválidos;
  code/grant replay; expiry; subject ya vinculado; email igual sin vínculo;
  usuario inactivo; rate limit; redirect no allowlisted; cancelación/timeouts.
- DB: constraints, concurrencia de doble link, up/down/up y limpieza.
- HTTP: cookies seguras en producción y ausencia de JWT en `Location`/logs.

## Criterios de terminado

- [x] Sólo Authorization Code + PKCE S256 está habilitado.
- [x] Identidades se resuelven por `(issuer, subject)`.
- [x] No existe auto-link por email ni auto-registro.
- [x] State, nonce y grants son single-use, expiran y resisten replay concurrente.
- [x] Ningún token local/Google aparece en URLs o logs.
- [x] El callback usa issuer/audience/nonce/firma verificados.
- [x] Tests no dependen de Google/red externa.
- [x] `make verify` y migraciones pasan.

## Condiciones de parada

- Plan 013 no proporciona `EstablishSession` seguro.
- Se solicita auto-link por email sin reautenticación explícita.
- La única entrega posible pone JWT en redirect URL.
- No hay decisión/allowlist de redirect y frontend final.
- El proveedor no soporta PKCE S256 o discovery/JWKS verificable.
- Se requiere acceso Gmail u otros scopes no justificados por login.

## Notas de mantenimiento

- Extraiga una librería OIDC sólo al aparecer un segundo consumidor/proveedor y
  mantenga persistencia/HTTP en cada aplicación.
- Añadir Google-only signup requiere separar credenciales password opcionales y
  un flujo de provisioning; hágalo en otro plan.
- Revise periódicamente scopes, redirect URIs y client secrets registrados en
  Google Cloud.
