# Plan 017: Añadir agilidad de claves JWT y eventos de seguridad

> **Instrucciones para el agente ejecutor**: Mantenga el constructor HMAC actual
> compatible y agregue una ruta explícita para signer/verifier rotables. Diseñe
> primero los contratos y la migración de tokens antes de cambiar defaults. No
> registre material criptográfico. Actualice la fila 017 del índice.
>
> **Control de deriva**:
>
> ```bash
> git -C /Users/wilsonsayago/workspace/golang/authBase diff --stat b5c0a2c..HEAD -- core/services/token_manager.go core/services/authenticationservice.go infra/config/properties/jwt.go docs
> git -C /Users/wilsonsayago/workspace/golang/base-golang diff --stat 59aeb1f..HEAD -- internal/infra/config internal/infra/bootstrap internal/infra/audit internal/infra/primary docs
> ```

## Estado

- **Estado**: Verificado (`authBase/v4.3.0`; separación criptográfica y Gitleaks verdes vía 018)
- **Prioridad**: P2
- **Esfuerzo**: L
- **Riesgo**: HIGH
- **Depende de**: planes 012, 013 y 015
- **Categoría**: security / architecture
- **Planeado en**: `base-golang@59aeb1f`, `authBase@b5c0a2c`, 2026-09-25

## Por qué importa

authBase fija HS256 y secretos estáticos. En una topología con varios resource
servers, todos los verificadores que conocen el HMAC también pueden emitir
tokens. Sin `kid` y claves previas no existe rotación gradual. Se necesitan
signer/verifier separados, algoritmo allowlisted, tipado JWT explícito y eventos
de seguridad sin acoplar authBase a un backend de observabilidad.

## Estado actual

- `authBase/core/services/token_manager.go:147` crea
  `jwt.NewWithClaims(jwt.SigningMethodHS256, claims)`.
- `ParseAccess/ParseRefresh` seleccionan secretos desde `properties.Jwt` y
  allowlistan HS256.
- `properties.Jwt` contiene `SecretKey` y `RefreshSecret`; no hay `kid`, claves
  anteriores ni algoritmos alternativos.
- `TokenType` diferencia access/refresh dentro de claims, pero el header `typ`
  no se establece/valida explícitamente.
- base-golang inyecta config y logger; no debe transferir claves privadas a
  resource servers.

## Comandos necesarios

| Propósito | Comando | Resultado esperado |
|---|---|---|
| Auth gate | `cd /Users/wilsonsayago/workspace/golang/authBase && GOWORK=off make verify` | Sale 0 |
| Base gate | `cd /Users/wilsonsayago/workspace/golang/base-golang && DATABASE_TEST_URL='<dsn-efimero>' GOWORK=off make verify` | Sale 0 |
| Compat | consumidor temporal con `NewTokenManager(properties.Jwt{...})` | Compila y valida tokens HMAC legacy |

## Alcance

**Dentro de alcance**:

- Contratos `TokenSigner`/`TokenVerifier` y key IDs.
- Adaptador HMAC compatible y adaptador asimétrico Ed25519 recomendado.
- Key ring con clave activa y claves de verificación anteriores.
- `typ`, `kid`, algorithm allowlist y transición de tokens legacy.
- Event sink opcional tipado en authBase y adaptador slog/métricas en base.
- Documentación y tests de rotación.

**Fuera de alcance**:

- Implementar un authorization server OAuth completo.
- KMS/HSM específico de un cloud; sólo puertos/adapters locales.
- Cifrar JWT/JWE, DPoP o mTLS.
- Denylist distribuida de access tokens.
- Incluir claves privadas en JWKS o logs.

## Flujo Git

- authBase: `codex/017-jwt-key-agility`.
- base-golang: `codex/017-jwt-key-agility`.
- Mantenga HMAC como compatibilidad; active asimétrico en base sólo después de
  pruebas de solapamiento.

## Pasos

### Paso 1: caracterizar tokens actuales

Añada fixtures generados en test para access/refresh HMAC con/sin `kid`; no
commitee secretos reales ni tokens de producción. Pruebe issuer, audience,
token type, family ID y leeway. Estos tests definen compatibilidad durante la
rotación.

### Paso 2: extraer signer y verifier

Defina interfaces pequeñas que reciban claims/tipo y devuelvan/verifiquen JWT
sin exponer secretos. `TokenManager` debe depender de ellas. Mantenga
`NewTokenManager(properties.Jwt)` creando el adaptador HMAC actual. Añada un
constructor explícito con dependencias criptográficas para nuevos consumidores.

El verifier debe seleccionar sólo claves configuradas localmente por `kid`; no
seguir `jku`/`x5u` recibidos del token ni consultar URLs arbitrarias.

### Paso 3: tipar y restringir tokens

Establezca `typ` distintos y documentados para access y refresh, mantenga el
claim `token_type` y valide ambos. Verifique que algoritmo del header coincide
con la clave/algoritmo configurados. Un `kid` desconocido, vacío fuera del modo
legacy o duplicado debe fallar cerrado.

### Paso 4: implementar Ed25519 y key ring

Añada signer Ed25519 con una clave activa y verifier con conjunto de claves
públicas activas/anteriores. Separe por completo access y refresh: no reutilice
la misma key ni `kid`. Parseo de config debe validar permisos/formato sin incluir
PEM/JWK en errores.

Proporcione un JWKS sólo con claves públicas de access cuando se configure; no
publique material refresh. Si base-golang expone `/.well-known/jwks.json`, añada
cache headers, ETag y tests de que sólo aparecen keys públicas esperadas.

### Paso 5: diseñar transición sin corte abrupto

Permita una ventana documentada:

1. verificar tokens HMAC existentes mientras se emiten tokens asimétricos;
2. esperar como mínimo la expiración máxima relevante;
3. retirar verifier legacy;
4. rotar active → previous mediante `kid` sin reinicio inseguro.

No acepte tokens sin `kid` indefinidamente. La decisión de invalidar tokens debe
ser explícita si no puede mantenerse la ventana.

### Paso 6: agregar eventos de seguridad

Defina `SecurityEventSink` opcional, no un logger, con enums y campos acotados:
login success/failure, refresh success/replay, family/user revoke, token reject,
key rotation y OIDC si el plan 016 existe. No incluya JWT, password, hash,
secreto, email crudo ni error interno arbitrario. El sink no debe controlar la
decisión de autenticación; fallos del sink siguen una política documentada.

En base-golang implemente slog y contadores de baja cardinalidad. Use user ID sólo
en logs auditables, nunca como label de métricas.

### Paso 7: actualizar configuración y operación

Documente generación, permisos, almacenamiento, rotación, rollback y respuesta
a compromiso. Producción debe rechazar algoritmos ambiguos, keys repetidas,
private key ausente para emisión y combinación de claves access/refresh.

## Plan de pruebas

- Compat HMAC constructor/token fixtures.
- Ed25519 round-trip access/refresh, algoritmo incorrecto, `kid` desconocido,
  typ/claim cruzados, key access usada para refresh y claims inválidos.
- Rotación: old+new verifican durante overlap; sólo new después del retiro.
- JWKS no contiene `d`/material privado ni claves refresh.
- Event sink recibe tipos/campos esperados y nunca material sensible; sink con
  error/panic controlado no filtra autenticación ni derriba el proceso.
- Fuzz de parser JWT con inputs arbitrarios y límites razonables.

## Criterios de terminado

- [x] Constructor HMAC existente conserva compatibilidad.
- [x] Nuevos signers/verifiers validan algoritmo, `typ` y `kid` explícitamente.
- [x] Access y refresh usan claves separadas.
- [x] Rotación overlap está probada y documentada.
- [x] JWKS, si existe, expone sólo claves públicas de access.
- [x] Eventos son tipados, acotados y libres de secretos/PII cruda.
- [x] Gates de ambos repositorios pasan.

## Condiciones de parada

- La refactorización rompe el constructor/API v4 existente sin plan major.
- No está definida la duración de tokens necesaria para el overlap.
- Se pretende resolver `jku`/`x5u` desde headers no confiables.
- Se pretende publicar una clave privada o refresh key en JWKS.
- No existe almacenamiento seguro para claves privadas de producción.

## Notas de mantenimiento

- Revise algoritmos y tamaños de clave periódicamente conforme a RFC 8725 y
  políticas de plataforma.
- Cada nueva clase de token debe tener `typ`, audience y verifier separados.
- KMS/HSM puede implementarse detrás de `TokenSigner` sin cambiar core.
