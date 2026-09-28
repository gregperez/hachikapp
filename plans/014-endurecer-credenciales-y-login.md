# Plan 014: Endurecer credenciales, passwords y abuso del login

> **Instrucciones para el agente ejecutor**: Implemente este plan en authBase y
> base-golang conservando respuestas genéricas. No integre servicios externos de
> contraseñas comprometidas en el request path. Actualice la fila 014 del índice.
>
> **Control de deriva**:
>
> ```bash
> git -C /Users/wilsonsayago/workspace/golang/authBase diff --stat b5c0a2c..HEAD -- core/services/authenticationservice.go core/port/validationport.go infra/secundary/validationservice.go
> git -C /Users/wilsonsayago/workspace/golang/base-golang diff --stat 59aeb1f..HEAD -- internal/core/domain/password.go internal/infra/primary/controller/auth_controller.go internal/infra/primary/controller/dto/inputs/users internal/infra/bootstrap internal/infra/config
> ```

## Estado

- **Estado**: Verificado (`authBase/v4.2.0`; política NIST, Argon2id y throttling)
- **Prioridad**: P0
- **Esfuerzo**: L
- **Riesgo**: MED
- **Depende de**: planes 012 y 013
- **Categoría**: security
- **Planeado en**: `base-golang@59aeb1f`, `authBase@b5c0a2c`, 2026-09-25

## Por qué importa

El login no limita intentos y la ruta de usuario inexistente evita el coste de
bcrypt, creando una diferencia temporal. La política local exige composición
por mayúscula/minúscula/dígito, mientras los estándares actuales priorizan
longitud, blocklists y rate limiting. El adaptador incluido genera bcrypt para
credenciales nuevas aunque Argon2id es preferible para greenfield.

## Estado actual

- `authBase/core/services/authenticationservice.go:71-84` retorna al recibir
  `ErrNotFound` antes de ejecutar `CheckPassword`; password incorrecta sí ejecuta
  el hash.
- `authBase/infra/secundary/validationservice.go` usa `bcrypt.DefaultCost`.
- `base-golang/internal/core/domain/password.go:8-30` exige 12 caracteres,
  mayúscula, minúscula y dígito, y rechaza espacios en extremos.
- `base-golang/internal/infra/primary/controller/auth_controller.go:49-55`
  registra login/refresh/logout sin limitador.
- Los errores de login ya convergen en `core.ErrInvalidCredentials`; preserve
  ese contrato y el mismo status/body.

## Referencias normativas

- NIST SP 800-63B: passwords de factor único con mínimo 15 caracteres,
  máximo permitido de al menos 64, sin reglas de composición, blocklist y
  throttling.
- OWASP Password Storage: Argon2id preferido; bcrypt debe respetar su límite de
  72 bytes y una estrategia de migración.
- OWASP Authentication: respuesta y tiempo no deben revelar si la cuenta existe.

## Comandos necesarios

| Propósito | Comando | Resultado esperado |
|---|---|---|
| Auth gate | `cd /Users/wilsonsayago/workspace/golang/authBase && GOWORK=off make verify` | Sale 0 |
| Base gate | `cd /Users/wilsonsayago/workspace/golang/base-golang && DATABASE_TEST_URL='<dsn-efimero>' GOWORK=off make verify` | Sale 0 |
| Focus auth | `GOWORK=off go test -race -count=20 ./core/services ./infra/secundary` en authBase | Sale 0 |

## Alcance

**Dentro de alcance**:

- Comparación de coste uniforme en authBase.
- Hash Argon2id PHC para credenciales nuevas con verificación compatible bcrypt.
- Política de password local alineada con NIST.
- Blocklist inyectable/offline.
- Rate limiting login por cuenta normalizada e IP, configuración, HTTP 429,
  pruebas y documentación operativa.

**Fuera de alcance**:

- CAPTCHA, scoring comercial, biometría, MFA, OIDC o recuperación de password.
- Consultar HIBP u otra API externa sin decisión de privacidad/disponibilidad.
- Bloqueo permanente automático de cuentas.
- Registrar email crudo, password, hash, token o dirección IP completa como label
  de métricas.
- Rehashear toda la tabla en una migración: no existe plaintext para hacerlo.

## Flujo Git

- authBase: `codex/014-password-hardening`.
- base-golang: `codex/014-login-throttling`.
- Commit por unidad: `security: equalize login checks`, `security: add argon2id`,
  `security: throttle authentication endpoints`.

## Pasos

### Paso 1: igualar el trabajo de verificación

Construya una credencial ficticia una vez al crear `AuthenticationService`,
usando el mismo `ValidationPort`. Cuando el usuario no exista, ejecute
`CheckPassword` contra ese hash antes de devolver `ErrInvalidCredentials`.
Para cuentas inactivas, ejecute una verificación equivalente y devuelva el mismo
error. No use sleeps: son inestables y consumen workers.

**Verificar**: tests con un ValidationPort contador prueban exactamente una
verificación para usuario inexistente, inactivo, password incorrecta y éxito.

### Paso 2: implementar Argon2id compatible con bcrypt

En el adaptador `infra/secundary`, genere hashes nuevos en formato PHC
`$argon2id$...` con parámetros explícitos, salt aleatorio criptográfico y
comparación constant-time. `CheckPassword` debe reconocer Argon2id y bcrypt
existente; entradas desconocidas o malformadas retornan `false`, nunca panic.
Use límites estrictos al parsear parámetros para impedir hashes que agoten RAM.

No cambie `ValidationPort` si no es necesario. Si necesita señalar rehash,
añada una interfaz de capacidad opcional y no rompa implementadores existentes.

**Verificar**: golden round-trip, salts distintos, password incorrecta,
malformados, parámetros excesivos, bcrypt legado y race detector.

### Paso 3: actualizar la política de passwords

Cambie `ValidatePassword` y los tags HTTP para:

- contar runas, no bytes;
- mínimo 15 para el modo password-only actual;
- aceptar al menos 64 y fijar un máximo defensivo documentado (recomendado 128
  runas y un límite adicional razonable de bytes);
- permitir espacios y Unicode;
- eliminar requisitos de clases de caracteres;
- rechazar entradas de una blocklist inyectable y términos contextuales.

No envíe passwords a servicios remotos. Proporcione una interfaz de checker y
un adaptador offline/configurable; producción debe documentar cómo proveer la
lista. Mensajes externos no deben revelar si el rechazo vino de una lista
comprometida más allá de una indicación segura para elegir otra password.

**Verificar**: tabla de tests con 14/15/64/128 runas, Unicode, espacios,
password común, muy larga y cancelación.

### Paso 4: normalizar la clave del limitador

Defina un `LoginLimiter` en la capa de aplicación/HTTP de base-golang. La clave
por cuenta debe derivarse de email normalizado y almacenarse como hash; combine
una cuota por cuenta y otra por IP/origen confiable. Defina explícitamente la
política de proxies confiables; no confíe ciegamente en `X-Forwarded-For`.

El adaptador en memoria debe tener expiración/bounds para no crecer sin límite.
Documente un adaptador distribuido como requisito para más de una réplica; no
añada Redis si no existe decisión operativa.

### Paso 5: aplicar el limitador a login y refresh

En login, aplique las cuotas de origen y cuenta normalizada antes del hash de
password. En refresh, aplique primero la cuota de origen y, después de validar
estructural y criptográficamente el token, una cuota derivada de un hash de
`family_id`/usuario; no intente extraer un email inexistente del request ni use
claims sin verificar como autoridad. Registre éxito/fallo sin PII cruda y
reinicie/reduzca la penalización después de éxito según la política. Responda
RFC 9457 con 429 y `Retry-After`, preservando 401 genérico para credenciales
inválidas.

**Verificar**: pruebas HTTP para límite exacto, ventanas, éxito, cuentas
independientes, IP independiente, refresh y headers.

### Paso 6: documentar parámetros y migración

Actualice docs de seguridad con parámetros Argon2id, soporte bcrypt legado,
política de rate limit, estrategia multi-réplica y cómo rotar hacia hashes
nuevos. No afirme que todos los hashes existentes se migran hasta que ocurra un
cambio/login con rehash implementado.

## Plan de pruebas

- authBase unit/race: equivalencia de llamadas y parsers PHC hostiles.
- base domain: matriz NIST y blocklist.
- base HTTP: 401 indistinguible, 429, Retry-After y ausencia de PII en logs.
- Benchmark informativo de hashing para documentar coste; no convierta un tiempo
  exacto de CI compartida en assertion frágil.

## Criterios de terminado

- [x] Todos los resultados de login ejecutan trabajo de verificación comparable.
- [x] Hashes nuevos usan Argon2id; bcrypt legado sigue verificando.
- [x] PHC malformado/excesivo falla cerrado sin panic ni asignaciones desmedidas.
- [x] Política permite passphrases/Unicode y elimina composición obligatoria.
- [x] Login y refresh tienen límites por cuenta e IP con memoria acotada.
- [x] 429 incluye `Retry-After`; 401 conserva cuerpo genérico.
- [x] Gates completos pasan en ambos repositorios.

## Condiciones de parada

- El cambio exige romper `ValidationPort` para todos los consumidores v4.
- No se puede determinar una política de proxies confiables para producción.
- Se pretende llamar una API externa con password o hash completo.
- La configuración Argon2id excede de forma sostenida los recursos objetivo.
- Se propone una migración que necesite recuperar plaintext de hashes existentes.

## Notas de mantenimiento

- Recalibre Argon2id en el hardware real y revise parámetros anualmente.
- El plan 016 debe reutilizar el rate limit y no crear una ruta de autenticación
  menos protegida.
- MFA/passkeys pueden permitir una política diferente, pero no deben debilitar el
  camino password-only sin una decisión de assurance explícita.
