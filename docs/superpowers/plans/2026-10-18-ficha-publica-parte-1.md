# Ficha pública, parte 1: plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Placas NFC/QR activables, una página pública `/t/<código>` que muestra solo lo permitido, y contacto mediado (mensaje + "Llamar" bajo demanda) registrado por escaneo, visible para el dueño en su ficha.

**Architecture:** Backend hexagonal existente. Dominio puro (`tag.go`, `publicprofile.go`, `tagscan.go`), puertos `TagPort`/`TagScanPort` con adaptador PostgreSQL `tagstore`, servicios `TagService` (dueño y admin) y `PublicTagService` (sin login), y tres controllers Gin. Frontend Next: ruta pública renderizada en el servidor, componentes cliente para escaneo y contacto, y secciones nuevas en la ficha y en la administración.

**Tech Stack:** Go 1.27 / Gin / pgx / golang-migrate / swag; Next.js 15 / React 19 / TypeScript; paquete npm `qrcode`.

**Spec:** `docs/superpowers/specs/2026-10-18-ficha-publica-design.md` (secciones 3 a 6 sin ubicación, 8, 9, 10 y 11).

## Global Constraints

- Sin dependencias Go nuevas: `crypto/rand`, `crypto/hmac`, `crypto/sha256` y `encoding/csv` de la biblioteca estándar.
- Única dependencia npm nueva: `qrcode` + `@types/qrcode`. La instala el dueño en su equipo (este entorno no tiene npm).
- Código de placa: 10 caracteres del alfabeto Crockford `0123456789ABCDEFGHJKMNPQRSTVWXYZ`.
- Máximo de 5 placas activas por mascota. Lotes de 1 a 1000 códigos.
- Ventana de agrupación de escaneos: 10 minutos.
- Mensaje: cuerpo de 1 a 500 caracteres, nombre hasta 80, teléfono con las reglas de `owner.phone` de la ficha.
- Límites por IP:

| Ruta | Límite |
|---|---|
| Ver perfil y su foto | 60 por minuto |
| Escaneos | 20 por hora |
| Mensajes | 5 por hora y placa |
| Contacto | 3 por hora y placa |

- Las respuestas públicas, salvo `/contact`, nunca contienen teléfono, dirección, número de chip ni apellidos.
- Los campos `lost_only` son ocultos (`lost=false` fijo en esta parte).
- Fuera de esta parte: avisos (correo y campana), ubicación de quien escanea y la limpieza de retención a los 30 y 180 días. Van en la parte 2, junto con el proceso en segundo plano.
- Errores con `application/problem+json`. Textos de cara al usuario en español y con el tono del manual.
- Migraciones solo hacia adelante. La última existente es la 000018.
- Commits con el trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Rama `feature/ficha-publica` en ambos repos.

## Review Focus

1. **Código tecleado a mano en minúsculas, con espacios o guiones, o con `O`/`I`/`L` en vez de `0`/`1`:** debe encontrar la placa. Lo cubren `NormalizeTagCode` y el test en la tarea 1.
2. **Navegador en modo privado sin `localStorage`:** el escaneo se registra igual, con un id por sesión. Lo cubre el test en la tarea 11.
3. **Quien encuentra la mascota recarga la página varias veces:** se registra un solo escaneo. Lo cubre el test en la tarea 5.
4. **La mascota se borra mientras su placa está activa:** la página pública muestra "sin activar", no un error 500. Lo cubre el test de integración en la tarea 3.
5. **El dueño desactiva el botón de contacto después de que la página ya cargó:** `/contact` responde 404 y la interfaz explica que el dueño no comparte su número. Lo cubren los tests en las tareas 8 y 11.

---

### Task 1: Dominio de placas

**Files:**
- Create: `internal/core/domain/tag.go`
- Create: `internal/core/domain/tag_test.go`

**Interfaces:**
- Produces:
  - `type TagKind string`: `TagKindPhysical`, `TagKindDigital`.
  - `type TagStatus string`: `TagStatusUnassigned`, `TagStatusActive`, `TagStatusDisabled`.
  - Constantes: `TagCodeLength = 10`, `TagCodeAlphabet`, `MaxActiveTagsPerPet = 5`, `MaxTagBatchQuantity = 1000`.
  - `type PetTag struct { ID, Code string; Kind TagKind; Status TagStatus; BatchID, PetID, OwnerID string; ActivatedAt, DisabledAt *time.Time; CreatedAt time.Time }`.
  - `type TagBatch struct { ID, Label string; Quantity int; CreatedBy string; CreatedAt time.Time }`.
  - `func NewTagCode(r io.Reader) (string, error)`.
  - `func NormalizeTagCode(raw string) (string, bool)`.
  - `func (t PetTag) CheckActivatableBy(userID string) error`.
  - `func ValidateTagBatch(label string, quantity int) error`: devuelve un `*PetValidationError` con los campos `label` y `quantity`.
  - Errores: `ErrTagNotAssignable`, `ErrTagLimitReached`, `ErrDuplicateTagCode`.

- [ ] **Step 1: Write the failing tests** en `tag_test.go`:
  - `TestNewTagCodeUsesCrockfordAlphabet`: 1000 códigos; todos de largo 10, solo caracteres de `TagCodeAlphabet` y sin repetidos.
  - `TestNormalizeTagCode`: tabla `"k7m2-q9xr 4p"→"K7M2Q9XR4P",true`, `"K7M2Q9XR4O"→"K7M2Q9XR40",true` (O→0), `"I"/"L"→"1"`, `"ABC"→"",false`, `"K7M2Q9XR4U"→"",false` (la U no es válida).
  - `TestCheckActivatableBy`:
    - unassigned sin dueño → nil;
    - unassigned con el mismo dueño → nil;
    - unassigned con otro dueño → `ErrTagNotAssignable`;
    - active → `ErrTagNotAssignable`;
    - disabled → `ErrTagNotAssignable`.
  - `TestValidateTagBatch`: cantidad 0 y 1001 → campo `quantity`, código `range`; etiqueta vacía → `label`, `required`; etiqueta de más de 80 → `label`, `max`; `("Lote A", 1000)` → nil.
- [ ] **Step 2:** Run `go test ./internal/core/domain/ -run 'Tag'` → FAIL (símbolos no definidos).
- [ ] **Step 3: Implement** lo declarado en Interfaces.
  - `NewTagCode` lee bytes de `r` y descarta los valores ≥ 224 (7×32), para que la distribución sea uniforme.
  - `NormalizeTagCode` pasa a mayúsculas, quita espacios y guiones y mapea O→0 e I/L→1.
- [ ] **Step 4:** Run el mismo comando → PASS.
- [ ] **Step 5: Commit** `feat: add tag domain (codes, states, batches)`.

### Task 2: Perfil público y comuna sin números

**Files:**
- Create: `internal/core/domain/publicprofile.go`, `internal/core/domain/publicprofile_test.go`
- Modify: `internal/core/domain/pet.go` (validación de `owner.comuna`), `internal/core/domain/pet_test.go`

**Interfaces:**
- Produces:
  - `type PublicProfile struct { Name string; HasPhoto bool; PhotoVersion int64; Species, Breed, Color, Sex, Size, DistinctiveFeatures, Temperament string; AgeMonths *int; Allergies, SpecialConditions, Surgeries []string; OwnerFirstName, SecondOwnerFirstName, Comuna string; CanContact, Lost bool }`.
  - `func PublicProfileOf(pet Pet, lost bool, now time.Time) PublicProfile`.
  - `func FirstName(full string) string`.
  - `func ComunaIsSafe(value string) bool`.

- [ ] **Step 1: Write the failing tests:**
  - `TestPublicProfileDefaultsHideLostOnly`: con `DefaultPetVisibility()` y `lost=false` muestra nombre, foto, especie, alergias, condiciones y temperamento, y `CanContact=true`; raza, color, sexo, tamaño, rasgos, comuna y `OwnerFirstName` vacíos; `AgeMonths` nil (age=hidden).
  - `TestPublicProfileLostShowsLostOnly`: con `lost=true` aparecen raza, comuna y `OwnerFirstName`.
  - `TestPublicProfileHonoursHidden`: con `allergies=hidden` el resultado es nil o vacío aunque la mascota tenga alergias; con `contactButton=hidden` → `CanContact=false`.
  - `TestPublicProfileOwnerFirstNameOnly`: `"Wilson Sayago"` → `"Wilson"`; segundo dueño `"Andrea Alastre"` → `"Andrea"` con `secondOwner=always`.
  - `TestPublicProfileDropsUnsafeComuna`: comuna `"Av X 123"` con `comuna=always` → `""`.
  - `TestPublicProfileAgeMonths`: nacimiento 2021-03-14 y `now` 2026-10-18 → 67 meses con `age=always`.
  - `TestPublicProfileNeverCarriesPrivateData`: serializa a JSON y comprueba que no contiene el teléfono, la dirección, el chip ni el apellido.
  - En `pet_test.go`, `TestComunaRejectsDigits`: `"Ñuñoa"` válido; `"Av X 123"` → campo `owner.comuna`, código `no_digits`.
- [ ] **Step 2:** Run `go test ./internal/core/domain/` → FAIL.
- [ ] **Step 3: Implement** `PublicProfileOf`.
  - Un campo es visible cuando `v == always || (v == lost_only && lost)`.
  - `HasPhoto` requiere `pet.HasPhoto` y visibilidad de `photo`; `PhotoVersion = pet.PhotoUpdatedAt.UnixMilli()`.
  - Agrega `no_digits` a `ValidatePetDetails` para `owner.comuna`.
- [ ] **Step 4:** Run → PASS, con todo el paquete en verde.
- [ ] **Step 5: Commit** `feat: add public profile projection and safe comuna`.

### Task 3: Migración y adaptador PostgreSQL de placas

**Files:**
- Create: `internal/resources/sql/migrations/000019_pet_tags.up.sql` / `.down.sql`
- Create: `internal/core/ports/tag.go`
- Create: `internal/infra/secondary/repositories/postgres/tagstore/store.go`, `store_test.go`
- Modify: `Makefile` (agrega `tagstore` a `test-db`)

**Interfaces:**
- Consumes: los tipos de la tarea 1.
- Produces: `ports.TagPort` con estos métodos:
  - `CreateBatch(ctx, batch domain.TagBatch, codes []string) (domain.TagBatch, error)`: devuelve `ErrDuplicateTagCode` si choca con el índice único.
  - `ListBatches(ctx, page authBaseDomain.PageRequest) (authBaseDomain.Page[domain.TagBatch], error)`.
  - `BatchTags(ctx, batchID string) ([]domain.PetTag, error)`.
  - `GetByCode(ctx, code string) (domain.PetTag, error)`.
  - `ListByPet(ctx, ownerID, petID string) ([]domain.PetTag, error)`.
  - `CountActiveByPet(ctx, petID string) (int, error)`.
  - `Activate(ctx, code, ownerID, petID string, at time.Time) (domain.PetTag, error)`: `UPDATE … WHERE status='unassigned' AND (owner_id IS NULL OR owner_id=$owner)`; si no afecta filas → `ErrTagNotAssignable`.
  - `CreateDigital(ctx, ownerID, petID, code string, at time.Time) (domain.PetTag, error)`.
  - `Disable(ctx, ownerID, petID, tagID string, at time.Time) (domain.PetTag, error)`.

- [ ] **Step 1: Write the migration.**
  - Tablas `tag_batches` y `pet_tags`, según la especificación (sección 3), con todos los `CHECK`.
  - Índice `pet_tags(pet_id) WHERE status='active'`.
  - Función y trigger `BEFORE DELETE ON pets` que dejan `status='unassigned', pet_id=NULL` y conservan `owner_id`.
  - Tablas `tag_scans` (con las columnas de ubicación como nulos, para la parte 2) y `tag_messages`, ambas con `pets ON DELETE CASCADE`.
  - Trigger `set_entity_timestamps` en `pet_tags`.
  - El `down` borra todo en orden inverso.
- [ ] **Step 2: Write the failing integration tests:**
  - `TestTagStoreBatchAndActivation`: crea un lote de 3, activa uno para la mascota A del dueño A; activar de nuevo → `ErrTagNotAssignable`; `ListByPet(A)` devuelve 1 placa.
  - `TestTagStoreDuplicateCode`: devuelve `ErrDuplicateTagCode`.
  - `TestTagStoreDisableIsOwnerScoped`: el dueño B no puede desactivar → `ErrNotFound`.
  - `TestTagStorePetDeletionReleasesTag` (Review Focus 4): al borrar la mascota, la placa queda `unassigned`, `pet_id` vacío y `owner_id` igual al dueño A; el dueño B no puede activarla y el dueño A sí puede vincularla a otra mascota.
- [ ] **Step 3:** Run `go run ./.tmp/migratecheck && go test -p 1 ./internal/infra/secondary/repositories/postgres/tagstore/` → FAIL (falta el paquete).
- [ ] **Step 4: Implement** `tagstore.Store` siguiendo el patrón de `petstore`. La paginación keyset de lotes usa `(created_at, id) DESC`, igual que `dao` de usuarios.
- [ ] **Step 5:** Run → PASS. Las migraciones también hacen up, down y up hasta la 19.
- [ ] **Step 6: Commit** `feat: add pet tags schema and store`.

### Task 4: `TagService` (dueño y admin)

**Files:**
- Create: `internal/core/tag_usecase.go`, `internal/core/services/tag_service.go`, `tag_service_test.go`

**Interfaces:**
- Consumes: `ports.TagPort` (tarea 3) y `ports.PetPort.GetForOwner` (existente).
- Produces: `core.TagUseCase` con estos métodos:
  - `CreateBatch(ctx, actorID, label string, quantity int) (domain.TagBatch, error)`;
  - `ListBatches(ctx, page) (Page[TagBatch], error)`;
  - `BatchTags(ctx, batchID) ([]PetTag, error)`;
  - `ListPetTags(ctx, ownerID, petID) ([]PetTag, error)`;
  - `CreateDigital(ctx, ownerID, petID) (PetTag, error)`;
  - `Activate(ctx, ownerID, code, petID) (PetTag, error)`;
  - `Disable(ctx, ownerID, petID, tagID) (PetTag, error)`.
- Constructor: `NewTagService(tags ports.TagPort, pets ports.PetPort, random io.Reader) *TagService`, con el campo `now func() time.Time`.

- [ ] **Step 1: Write the failing tests** (fakes en memoria, como en `pet_service_test.go`):
  - `TestTagServiceActivateRequiresOwnedPet`: mascota ajena → `ErrNotFound`.
  - `TestTagServiceActivateNormalizesCode`: `"k7m2-q9xr4p"` activa `K7M2Q9XR4P`.
  - `TestTagServiceEnforcesFiveActiveTags`: la sexta (activación o digital) → `ErrTagLimitReached`.
  - `TestTagServiceRetriesCodeCollision`: el puerto falla con `ErrDuplicateTagCode` una vez y luego funciona → el lote se crea.
  - `TestTagServiceCreateBatchValidates`: 1001 → `ErrInvalidPet` (validación) y no llama al puerto.
- [ ] **Step 2:** Run `go test ./internal/core/services/ -run TagService` → FAIL.
- [ ] **Step 3: Implement.** Reintenta hasta 3 veces ante `ErrDuplicateTagCode`.
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat: add tag use cases`.

### Task 5: Escaneos y mensajes (dominio, puerto y adaptador)

**Files:**
- Create: `internal/core/domain/tagscan.go`, `tagscan_test.go`
- Modify: `internal/core/ports/tag.go` (agrega `TagScanPort`)
- Create: `internal/infra/secondary/repositories/postgres/tagstore/scans.go`, `scans_test.go`

**Interfaces:**
- Produces (dominio):
  - Tipos `TagScan { ID, TagID, PetID string; ScannedAt time.Time; VisitorHash, ScanTokenHash, IPPrefix, UserAgentFamily string; PhoneRevealedAt *time.Time }`, `TagMessage { ID, ScanID, TagID, PetID, Body, FinderName, FinderPhone string; CreatedAt time.Time }` y `FinderMessageInput { Body, FinderName, FinderPhone string }`.
  - `ScanDedupeWindow = 10 * time.Minute`.
  - Funciones:
    - `KeyedHash(pepper []byte, value string) string` (HMAC-SHA256 en hex);
    - `NewScanToken(r io.Reader) (string, error)` (32 bytes en base64url);
    - `IPPrefix(ip string) string` (/24 o /48);
    - `UserAgentFamily(ua string) string` (`ios`, `android`, `desktop` u `other`);
    - `NormalizeFinderMessage(in) FinderMessageInput`;
    - `ValidateFinderMessage(in) error` (`*PetValidationError`, campos `body`, `finderName` y `finderPhone`).
- Produces (puerto `TagScanPort`):
  - `RecordScan(ctx, scan domain.TagScan, dedupeSince time.Time) (domain.TagScan, bool, error)`: si existe un escaneo con la misma `(tag_id, visitor_hash)` y `scanned_at >= dedupeSince`, actualiza `scan_token_hash` y devuelve `reused=true`.
  - `GetScan(ctx, scanID string) (domain.TagScan, error)`.
  - `AddMessage(ctx, msg domain.TagMessage) (domain.TagMessage, error)`.
  - `MarkPhoneRevealed(ctx, scanID string, at time.Time) (bool, error)`: devuelve `true` solo la primera vez.
  - `ListScansByPet(ctx, ownerID, petID string, page PageRequest) (Page[domain.ScanWithMessages], error)`, con `ScanWithMessages { TagScan; TagCode string; Messages []TagMessage }`.

- [ ] **Step 1: Write the failing tests:**
  - `TestIPPrefix`: `"203.0.113.45"→"203.0.113.0/24"`, `"2001:db8::1"→"2001:db8::/48"` e inválida → `""`.
  - `TestKeyedHashDependsOnPepper`: el mismo valor con distinto pepper da distinto hash.
  - `TestValidateFinderMessage`: cuerpo vacío → `body`, `required`; cuerpo de 501 → `body`, `max`; teléfono `"123"` → `finderPhone`, `phone`; mensaje mínimo válido → nil.
  - Integración `TestRecordScanDedupes` (Review Focus 3): dos `RecordScan` con el mismo visitante dentro de la ventana devuelven el mismo id, con `reused=true` la segunda vez; fuera de la ventana → id nuevo.
  - Integración `TestMarkPhoneRevealedOnce`: `true` y después `false`.
  - Integración `TestListScansByPetIsOwnerScoped`: el dueño B obtiene una lista vacía.
- [ ] **Step 2:** Run los tests de dominio y de integración → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat: add tag scans and finder messages storage`.

### Task 6: `PublicTagService`

**Files:**
- Create: `internal/core/public_tag_usecase.go`, `internal/core/services/public_tag_service.go`, `public_tag_service_test.go`

**Interfaces:**
- Consumes: `TagPort`, `TagScanPort`, `PetPort.GetForOwner(tag.OwnerID, tag.PetID)` (no hace falta un método nuevo en `PetPort`), `PetPhotoPort.Get` y `domain.PublicProfileOf`.
- Produces: `core.PublicTagUseCase` con estos métodos:
  - `View(ctx, code string) (domain.PublicTagView, error)`, con `PublicTagView { State domain.TagStatus; Profile *domain.PublicProfile }`;
  - `Photo(ctx, code string) (domain.PetPhoto, error)`;
  - `RecordScan(ctx, code, visitorID, ip, userAgent string) (domain.ScanReceipt, error)`, con `ScanReceipt { ScanID, Token string }`;
  - `SendMessage(ctx, scanID, token string, in domain.FinderMessageInput) error`;
  - `RevealContact(ctx, scanID, token string) (domain.PublicContact, error)`, con `PublicContact { Phone, WhatsAppURL string }`;
  - `ListPetScans(ctx, ownerID, petID string, page PageRequest) (Page[ScanWithMessages], error)`.
- Constructor: `NewPublicTagService(tags ports.TagPort, scans ports.TagScanPort, pets ports.PetPort, photos ports.PetPhotoPort, pepper []byte, random io.Reader) *PublicTagService`.
- Error nuevo: `domain.ErrInvalidScanToken` (se mapea a 404, para no revelar que el escaneo existe).

- [ ] **Step 1: Write the failing tests:**
  - `TestViewUnknownOrMalformedCode`: devuelve `ErrNotFound`.
  - `TestViewDisabledHasNoProfile`.
  - `TestViewUnassignedAfterPetDeletion` (el pet store devuelve `ErrNotFound`): estado `unassigned`, sin error.
  - `TestRecordScanOnlyForActiveTags`: placa sin activar → `ErrNotFound`.
  - `TestSendMessageRequiresMatchingToken`: token incorrecto → `ErrInvalidScanToken`.
  - `TestRevealContactHonoursContactButton` (Review Focus 5): con `contactButton=hidden` → `ErrNotFound`; con visible → el teléfono de `owner.phone` y `WhatsAppURL = "https://wa.me/56912345678"`, solo dígitos y con prefijo 56 si el número tiene 9 dígitos y empieza en 9.
  - `TestRevealContactNeverReturnsPhoneAlt`.
  - `TestPhotoHiddenWhenPhotoNotVisible`: devuelve `ErrNotFound`.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.** El token se compara en tiempo constante (`subtle.ConstantTimeCompare` sobre los hashes).
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat: add public tag use cases`.

### Task 7: Configuración y límites de intentos

**Files:**
- Modify:
  - `internal/infra/config/appconfig/config.go`, `load.go` y `validate.go` (+ tests): agrega `Public PublicConfig { BaseURL string; VisitorPepper string }`.
  - `internal/core/ports/login_limiter.go`: nuevos scopes `PublicTagView`, `PublicTagScan`, `PublicTagMessage` y `PublicTagContact`.
  - `internal/infra/bootstrap/application.go`: registra esos scopes con los límites de las Global Constraints.
  - `internal/resources/properties.example.yml` y `.env.example`.

**Interfaces:**
- Produces:
  - `cfg.Public.BaseURL`: variable `PUBLIC_BASE_URL`; en desarrollo, `http://localhost:3000` por defecto.
  - `cfg.Public.VisitorPepper`: variable `TAG_VISITOR_PEPPER`; en desarrollo, `dev-only-tag-pepper` por defecto.
  - Los scopes, que usa la tarea 8.

- [ ] **Step 1: Write the failing tests** en `config_test.go`:
  - `TestPublicConfigFromEnv`;
  - `TestProductionRejectsDefaultPepper`: pepper por defecto o de menos de 32 caracteres → error;
  - `TestProductionRequiresHTTPSPublicBaseURL`.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat: add public profile config and rate limit scopes`.

### Task 8: Controllers HTTP, permiso `TAGS` y OpenAPI

**Files:**
- Create:
  - `internal/infra/primary/controller/public_tag_controller.go` (+ `_http_test.go`);
  - `tag_controller.go` (+ test);
  - `tag_admin_controller.go` (+ test);
  - los DTOs en `dto/inputs/tags` y `dto/outputs/tags`;
  - `internal/resources/sql/migrations/000020_tags_permission.up.sql` / `.down.sql`, que agrega `TAGS` (crear y leer) y la ruta `TAGS` al rol admin.
- Modify: `internal/core/authz/entities.go` (`EntityTags = "TAGS"`), `internal/infra/bootstrap/application.go` (servicios y controllers en `routeDeps`), `docs/` (`make openapi-generate`), `docs/CHANGELOG.md`, `docs/ARCHITECTURE.md`.

**Interfaces:**
- Consumes: los use cases de las tareas 4 y 6, `ClientOriginResolver.Origin` y `LoginLimiter`.
- Produces las rutas de la especificación:
  - Públicas, sin JWT:
    - `GET /v1/public/tags/:code`;
    - `GET /v1/public/tags/:code/photo`;
    - `POST /v1/public/tags/:code/scans` (body `{visitorId}`);
    - `POST /v1/public/scans/:scanId/messages`;
    - `POST /v1/public/scans/:scanId/contact`.

    Las dos últimas reciben la cabecera `X-Scan-Token`.
  - Del dueño, con JWT:
    - `GET /v1/pets/:id/tags`;
    - `POST /v1/pets/:id/tags`;
    - `POST /v1/pets/:id/tags/:tagId/disable`;
    - `POST /v1/tags/:code/activate` (body `{petId}`);
    - `GET /v1/pets/:id/scans`.
  - De admin, con `PoliciesGuard TAGS`:
    - `GET /v1/admin/tag-batches`;
    - `POST /v1/admin/tag-batches`;
    - `GET /v1/admin/tag-batches/:id/codes.csv` (`text/csv`, columnas `code,url` con `PUBLIC_BASE_URL/t/<code>`).
  - JSON del perfil público: `{state, profile:{name, photoUrl, species, breed, color, sex, size, ageMonths, distinctiveFeatures, allergies, specialConditions, surgeries, temperament, ownerFirstName, secondOwnerFirstName, comuna, canContact, lost}}`. Los campos ocultos se omiten (`omitempty`).
  - Cabeceras públicas: `Cache-Control: no-store` y `X-Robots-Tag: noindex`.
  - Mensaje con el campo trampa `website` lleno → 202 y no se guarda nada.
  - Mapeos de error: `ErrTagNotAssignable` → 409 "This tag is already linked"; `ErrTagLimitReached` → 409 "The pet already has 5 active tags"; `ErrInvalidScanToken` → 404.

- [ ] **Step 1: Write the failing HTTP tests:**
  - `TestPublicTagResponseNeverLeaksPrivateData`: el cuerpo no contiene el teléfono, la dirección, el chip ni el apellido;
  - `TestPublicTagHeaders`;
  - `TestPublicRoutesRateLimited`: la tercera llamada a `/contact` dentro de la ventana → 429 con `Retry-After`;
  - `TestHoneypotMessageIsDropped`;
  - `TestContactHiddenReturns404` (Review Focus 5);
  - `TestActivateConflict409`;
  - `TestAdminBatchRequiresTagsPermission` (403 sin el permiso);
  - `TestBatchCSV`: encabezado `code,url` y las URL con `PUBLIC_BASE_URL`.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.** Sigue el patrón de `pet_controller.go`: `writeTagError`, `requireIdentity` y `BindAndWrite`. Para los límites, aplica a todas las rutas públicas el mismo helper que `auth_rate_limit.go`, con la clave `KeyedHash(pepper, origin+"|"+code)`.
- [ ] **Step 4:** Run `go build ./... && go vet ./... && go test ./internal/... ./docs` → PASS. Las migraciones también hacen up, down y up hasta la 20.
- [ ] **Step 5:** Run `make openapi-generate`. Actualiza CHANGELOG y ARCHITECTURE con las rutas públicas, el permiso `TAGS` y el reemplazo del limitador por Redis si hay varias instancias.
- [ ] **Step 6: Commit** `feat: add tag, public profile and admin batch endpoints`.

### Task 9: Frontend: tipos, cliente y proxy

**Files:**
- Create: `src/lib/tags.ts`, `src/lib/tags.test.ts`
- Modify: `src/lib/types.ts`, `src/lib/api.ts` (traducciones nuevas) y `src/lib/navigation.ts` (`ENTITY_IDS.TAGS`, ítem "Placas" con `requiredPermission TAGS read`).

**Interfaces:**
- Produces:
  - Tipos `PetTag`, `TagBatch`, `PublicProfile`, `PublicTagView`, `ScanReceipt`, `PublicContact` y `ScanWithMessages`, que reflejan el JSON de la tarea 8.
  - `getVisitorId(storage?: Storage): string`: `localStorage` envuelto en try/catch; si falla, usa un id en memoria por sesión.
  - `recordScan(code)`, `sendFinderMessage(scanId, token, input)` y `revealContact(scanId, token)`: llaman a `/api/backend/v1/public/...` con `X-Scan-Token`.
  - `tagUrl(code: string): string`, que usa `NEXT_PUBLIC_PUBLIC_BASE_URL` (o `window.location.origin`) + `/t/` + código.
  - `formatAgeMonths(months?: number): string`, que reutiliza el formato de `petAge` ("5 años", "8 meses", "Menos de 1 mes").

- [ ] **Step 1: Write the failing tests:**
  - `getVisitorId` reutiliza el id guardado;
  - si `setItem` lanza error, devuelve un id estable en la sesión (Review Focus 2, lado de la librería);
  - `formatAgeMonths(67) === "5 años"`;
  - `revealContact` envía la cabecera `X-Scan-Token` (fetch espiado con `vi.stubGlobal`);
  - nav: "Placas" visible solo con `TAGS.read`;
  - traducciones de los 409 de placas.
- [ ] **Step 2:** Run el runner de tests → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat: add tag client helpers and types`.

### Task 10: Frontend: página pública `/t/[code]`

**Files:**
- Create:
  - `src/app/t/[code]/page.tsx` (componente de servidor);
  - `src/components/public-profile.tsx` (cliente);
  - `src/components/public-profile.test.tsx`;
  - `src/app/t/layout.tsx` (sin `AppShell`).
- Modify: `src/app/globals.css`, con estilos `.public-*` en tokens de marca y diseño pensado primero para celular.

**Interfaces:**
- Consumes: `PublicTagView` y las funciones de la tarea 9.
- Produces: `PublicProfileView({ code, view }: { code: string; view: PublicTagView | null })`, que renderiza los estados `active`, `unassigned` (botón "Activar" hacia `/login?from=/t/{code}/activar`), `disabled` y `null` (no encontrada o error del backend).
- El componente de servidor hace `fetch(`${BACKEND_URL}/v1/public/tags/${code}`, { cache: "no-store" })`: con 404 pasa `view=null`, con 5xx un estado de error, y exporta `metadata.robots = { index: false }`.

- [ ] **Step 1: Write the failing tests** (`public-profile.test.tsx`):
  - activa: muestra "Hola, soy Luna", la tarjeta de alergias y los dos botones; registra el escaneo una sola vez al montar;
  - sin activar: botón "Activar" con el `href` correcto;
  - desactivada: no muestra ningún dato;
  - `null`: "No encontramos esta placa";
  - el registro del escaneo falla → el perfil se sigue viendo;
  - "Llamar" no aparece en el DOM antes de tocarlo; al tocarlo llama a `revealContact` y muestra el enlace `tel:`;
  - `revealContact` responde 404 → "Su familia prefiere que le dejes un mensaje" (Review Focus 5);
  - mensaje: el botón queda deshabilitado mientras envía y muestra "¡Gracias! Su familia recibirá tu mensaje".
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.** Textos exactos:
  - pie: "Quien encuentre a tu mascota puede contactarte. Tú decides qué ve.";
  - botón principal: "Avisar a mi familia";
  - botón secundario: "Llamar / WhatsApp".
- [ ] **Step 4:** Run → PASS. También pasan typecheck y lint.
- [ ] **Step 5: Commit** `feat: add public tag page`.

### Task 11: Frontend: activación de placas

**Files:**
- Create: `src/app/t/[code]/activar/page.tsx`, `page.test.tsx`
- Modify: `src/middleware.ts` (agrega `/t/:code/activar` al matcher que exige sesión)

**Interfaces:**
- Consumes: `GET v1/pets` (existente), `POST v1/tags/{code}/activate` y `normalizeTagCode` (función en `tags.ts`, equivalente a la del backend; la agrega esta tarea con su test).

- [ ] **Step 1: Write the failing tests:**
  - sin mascotas → enlace "Crear la ficha de tu mascota";
  - activación correcta → redirige a `/dashboard/pets/{id}?tag=activated`;
  - un 409 muestra "Esta placa ya está vinculada";
  - `normalizeTagCode("k7m2-q9xr 4o") === "K7M2Q9XR40"`.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat: add tag activation flow`.

### Task 12: Frontend: placas y escaneos en la ficha, y lotes para el admin

**Prerequisite:** el dueño ejecutó `npm install qrcode @types/qrcode` en `hachikapp-frontend`. Si no, deténte y pídeselo.

**Files:**
- Create:
  - `src/components/pet-tags.tsx` (+ test);
  - `src/components/pet-scans.tsx` (+ test);
  - `src/app/dashboard/tags/page.tsx` (+ test).
- Modify: `src/app/dashboard/pets/[id]/page.tsx` (agrega las dos secciones y el aviso `?tag=activated`), `src/app/globals.css`.

**Interfaces:**
- Consumes: las rutas de dueño y de admin de la tarea 8 y `tagUrl` de la tarea 9.
- Produces:
  - `PetTags({ petId })`: lista con estado, "Crear placa digital", "Descargar QR" (SVG de `QRCode.toString(tagUrl(code), { type: "svg", errorCorrectionLevel: "M" })`) y "Desactivar" con confirmación.
  - `PetScans({ petId })`: fecha en hora de Chile, mensajes con el nombre y teléfono de quien encontró la mascota, y "Vio tu número" si corresponde.

- [ ] **Step 1: Write the failing tests:**
  - con 5 placas activas, "Crear placa digital" queda deshabilitado con su explicación;
  - el SVG del QR codifica `tagUrl(code)` (se comprueba que se llama a `QRCode.toString` con esa URL);
  - "Desactivar" pide confirmación;
  - escaneos sin datos → "Aún nadie ha escaneado sus placas";
  - admin: crea un lote de 10 y muestra el enlace al CSV; con 1001 muestra el error del campo.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4:** Run → PASS. También pasan typecheck y lint.
- [ ] **Step 5: Commit** `feat: add tags and scans to the pet page and admin batches`.

### Task 13: Verificación final

- [ ] **Step 1:** Backend: `go build ./... && go vet ./... && go test ./internal/... ./docs` (sin el test colgado de `dao`), migraciones up, down y up hasta la 20, e integración de `petstore` y `tagstore` → todo en verde.
- [ ] **Step 2:** Frontend: typecheck, lint y la suite completa en el runner → todo en verde. Pide al dueño que corra `npm test` real.
- [ ] **Step 3:** Capturas de `/t/<code>` (activa, sin activar, desactivada y no encontrada) en 390 px y 1360 px, y de las secciones nuevas de la ficha. Revísalas visualmente.
- [ ] **Step 4:** `superpowers:requesting-code-review` sobre la rama completa de ambos repos.
