# Ficha pública, parte 2: plan de implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que el dueño se entere de cada escaneo real de su placa, por correo y con una campana en el panel. Además, quien encuentra la mascota podrá compartir su ubicación, y los datos de los escaneos se borrarán solos según la retención acordada.

**Architecture:**
- **Mismo commit:** el escaneo, el mensaje, el contacto y la ubicación crean la notificación y su envío pendiente en la misma transacción que el evento. Lo hace `tagstore` llamando a un helper SQL de `notifystore`.
- **Envío:** `NotificationDispatcher`, un proceso del ciclo de vida igual a los purgers, toma los envíos vencidos con un lease de `FOR UPDATE SKIP LOCKED` y los manda por el puerto `Mailer` (adaptador SMTP o Log).
- **Limpieza:** la hace `TagRetentionPurger`.
- **Frontend:** agrega "Compartir mi ubicación" en la página pública, la campana con la página `/dashboard/avisos` y la ubicación en "Últimos escaneos".

**Tech Stack:** Go 1.27, Gin, pgx, golang-migrate, `net/smtp`, `html/template`; Next.js 15, React 19, TypeScript.

**Spec:** `docs/superpowers/specs/2026-10-18-ficha-publica-design.md`. Esta parte cubre las secciones 3 (`notifications`, `notification_deliveries` y la retención), 6 ("Ubicación" y su límite), 7 completa y 11 (`SMTP_*`, `NOTIFICATIONS_ENABLED`).

**Parte 1 (ya en `main`):** placas, página pública, escaneos, mensajes y contacto. Los archivos clave son `internal/core/services/public_tag_service.go`, `internal/infra/secondary/repositories/postgres/tagstore/{store,scans}.go` y `internal/infra/primary/controller/public_tag_controller.go`. En el frontend: `src/components/public-profile.tsx`, `src/components/pet-scans.tsx` y `src/lib/tags.ts`.

## Global Constraints

- **Dependencias:** ninguna nueva en Go ni en npm.
- **Tipos de aviso:** `tag_scanned`, `location_shared`, `finder_message` y `phone_revealed`.
- **Envío:**
  - ciclo cada 5 s, hasta 20 envíos por ciclo;
  - reintentos a 1 min, 5 min, 30 min, 2 h y 6 h; si falla el sexto intento, queda `failed`.
- **Retención:**
  - coordenadas en `NULL` a los 30 días, en `tag_scans` y en el `payload` de los avisos;
  - escaneos, mensajes y avisos borrados a los 180 días.
- **Ubicación:**
  - `lat` entre −90 y 90, `lng` entre −180 y 180, `accuracy` entero entre 0 y 100000 m;
  - una sola vez por escaneo (409 si ya existe);
  - límite de 10 por hora por IP.
- **Asunto del correo de escaneo:** "Alguien escaneó el Hachikapp Tag de {name}".
- **Hora en el correo:** en `America/Santiago`.
- **Enlace al mapa:** `https://maps.google.com/?q=lat,lng`.
- **Destinatario:** `users.email` del dueño de la placa.
- **Textos de la página pública:**
  - rechazo del permiso de ubicación: "Sin problema, igual puedes dejar un mensaje";
  - nunca un error técnico.
- **Configuración nueva:**
  - `NOTIFICATIONS_ENABLED`: por defecto `true`;
  - `SMTP_HOST`, `SMTP_PORT` (por defecto 587), `SMTP_USER`, `SMTP_PASSWORD` (secreto, nunca en logs) y `SMTP_FROM`.
  - Sin `SMTP_HOST`, se usa el adaptador Log.
  - En producción con avisos activos, `SMTP_HOST` y `SMTP_FROM` son obligatorios.
- **Migraciones:** solo hacia adelante; la última existente es la 000020.
- **Commits:** autor del usuario y el trailer `Co-Authored-By` de siempre. Rama `feature/ficha-publica-2` en ambos repos.

## Decisiones de este plan (precisan la especificación)

1. **Espera del primer correo:** sale 60 s después del escaneo (`next_attempt_at = now + 60 s`). Así la ubicación, el mensaje o el contacto que lleguen en ese minuto se agrupan en un solo correo.
2. **Agrupación:**
   - Aplica a `location_shared`, `finder_message` y `phone_revealed`. La especificación solo menciona los dos primeros; agrego el contacto para no mandar dos correos en un minuto.
   - Se agrupa cuando el envío del `tag_scanned` de ese escaneo sigue `pending`, con `attempts = 0` y `next_attempt_at > now()`.
   - En ese caso se combina el `payload` de la notificación del escaneo y se crea solo la notificación del panel, sin envío.
3. **Lease del envío:** se reclama en una transacción corta (`SKIP LOCKED` y `next_attempt_at = now + 2 min`) y se envía fuera de ella. Si el proceso muere, otro lo retoma al vencer el lease. La entrega es "al menos una vez".
4. **Asuntos de los correos no agrupados:**
   - `finder_message`: "{name}: te dejaron un mensaje";
   - `location_shared`: "{name}: compartieron dónde la encontraron";
   - `phone_revealed`: "{name}: alguien vio tu número".
5. **`NOTIFICATIONS_ENABLED=false`:** se crean los avisos del panel, pero no los envíos por correo, y el despachador no arranca.

## Review Focus

1. **El proveedor de correo cae varias horas, o el servidor se reinicia:** ningún aviso se pierde y cada uno se reintenta según la escalera. Lo cubren el test de la tarea 6 (`TestDispatcherRetryLadder`) y la integración de la tarea 2 (el estado vive en la base).
2. **Dos despachadores corren a la vez, o uno muere a mitad del envío:** cada envío sale una vez; si el lease vence, sale una vez más. Lo cubren `TestClaimDueSkipLocked` y `TestClaimDueReclaimsExpiredLease` en la tarea 2.
3. **Quien encuentra la mascota escribe HTML o `<script>` en el mensaje:** el correo HTML lo muestra escapado. Lo cubre `TestRenderEscapesFinderContent` en la tarea 1.
4. **Quien encuentra la mascota toca "Compartir mi ubicación" dos veces o en dos pestañas:** se guarda una ubicación y la segunda pestaña ve el mensaje de éxito, no un error. Lo cubren el test del 409 en la tarea 4 y el test de 409 como éxito en la tarea 10.
5. **El dueño tiene más de 99 avisos sin leer:** la campana muestra "99+" y no rompe la barra superior en el celular. Lo cubre el test de la tarea 11.

---

### Task 1: Dominio de avisos y correo

**Files:**
- Create: `internal/core/domain/notification.go` (+ `_test.go`), `internal/core/ports/notification.go`, `internal/core/services/notification_email.go` (+ `_test.go`)

**Interfaces:**
- Produces:
  - Tipos de dominio:
    - `type NotificationKind string`, con las 4 constantes `NotificationTagScanned`, `NotificationLocationShared`, `NotificationFinderMessage` y `NotificationPhoneRevealed`;
    - `type NotificationPayload struct { PetName, TagCode string; ScannedAt time.Time; Device string; Location *ScanLocation; Message *TagMessageSummary; PhoneRevealed bool }`, con tags JSON en camelCase y `omitempty`;
    - `type ScanLocation struct { Lat, Lng float64; AccuracyM int; At time.Time }`;
    - `type TagMessageSummary struct { Body, FinderName, FinderPhone string }`;
    - `type Notification struct { ID, UserID, PetID, TagID, ScanID, MessageID string; Kind NotificationKind; Payload NotificationPayload; ReadAt *time.Time; CreatedAt time.Time }`;
    - `type NotificationDraft struct { UserID, PetID, TagID string; Kind NotificationKind; Payload NotificationPayload; Email bool }`;
    - `type Delivery struct { ID, NotificationID string; Attempts int; Notification Notification; ToEmail string; OwnerName string }`.
  - Funciones de dominio:
    - `func NextDeliveryAttempt(attempts int, now time.Time) (next time.Time, failed bool)`;
    - `func ValidateScanLocation(lat, lng float64, accuracy int) error`, que devuelve un `*PetValidationError` con los campos `lat`, `lng` o `accuracy` y el código `range`;
    - error `ErrLocationAlreadyShared`.
  - Constantes: `FirstEmailDelay = 60 * time.Second`, `DeliveryLease = 2 * time.Minute`, `LocationRetention = 30 * 24 * time.Hour` y `ScanRetention = 180 * 24 * time.Hour`.
  - Puertos:
    - `ports.Mailer interface { Send(ctx, domain.Email) error }`, con `domain.Email{To, Subject, Text, HTML string}`;
    - `ports.NotificationPort`: `List(ctx, userID, page) (Page[Notification], error)`, `UnreadCount(ctx, userID) (int, error)`, `MarkRead(ctx, userID, id string, at time.Time) error` (ajena o inexistente → `ErrNotFound`) y `MarkAllRead(ctx, userID string, at time.Time) (int, error)`;
    - `ports.DeliveryPort`: `ClaimDue(ctx, now time.Time, limit int, lease time.Duration) ([]Delivery, error)`, `MarkSent(ctx, id, at)`, `MarkRetry(ctx, id string, attempts int, next time.Time, lastErr string)` y `MarkFailed(ctx, id string, attempts int, lastErr string)`.
  - Render: `services.RenderNotificationEmail(d domain.Delivery, baseURL string, loc *time.Location) domain.Email`, con texto plano y HTML hecho con `html/template`.

- [ ] **Step 1: Write the failing tests.**
  - `TestNextDeliveryAttemptLadder`: los intentos 1 a 5 dan +1 min, +5 min, +30 min, +2 h y +6 h; el intento 6 da `failed=true`.
  - `TestValidateScanLocation`:
    - (−33.45, −70.66, 20) es válido;
    - lat 91 → campo `lat`, código `range`; lng −181 → `lng`; accuracy −1 y 100001 → `accuracy`;
    - NaN o Inf → `range`.
  - `TestRenderScanEmail`:
    - asunto exacto "Alguien escaneó el Hachikapp Tag de Luna";
    - el texto incluye la hora en Santiago (por ejemplo, `ScannedAt` 2026-10-23T15:04Z se muestra como "12:04");
    - con ubicación incluye `https://maps.google.com/?q=-33.450000,-70.660000`;
    - con mensaje incluye el cuerpo, el nombre y el teléfono;
    - el enlace "Ver en Hachikapp" es `{baseURL}/dashboard/pets/{petId}`;
    - el saludo usa solo el nombre de pila del dueño.
  - `TestRenderEscapesFinderContent` (Review Focus 3): con cuerpo `<script>alert(1)</script>`, el HTML contiene `&lt;script&gt;` y no `<script>`.
  - `TestRenderSubjectsByKind`: los 4 asuntos de la sección de decisiones.
- [ ] **Step 2:** Run `go test ./internal/core/...` → FAIL.
- [ ] **Step 3: Implement.** HTML simple, con los colores del manual tomados de los tokens del frontend (`globals.css`: primario y fondo) en estilos en línea, y un botón "Ver en Hachikapp". Tono cálido: "Hola {nombre}: alguien encontró a {name} y escaneó su placa." y el cierre "Si fuiste tú, puedes ignorar este correo."
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat: add notification domain and email rendering`.

### Task 2: Migración y `notifystore`

**Files:**
- Create: `internal/resources/sql/migrations/000021_notifications.up.sql` / `.down.sql`, `internal/infra/secondary/repositories/postgres/notifystore/{store.go,events.go,store_test.go}`
- Modify: `Makefile` (agrega `notifystore` a `test-db`)

**Interfaces:**
- Consumes: los tipos y puertos de la tarea 1.
- Produces:
  - `notifystore.New(db)`, que implementa `NotificationPort` y `DeliveryPort`;
  - `notifystore.InsertEvent(ctx, tx pgx.Tx, draft domain.NotificationDraft, scanID, messageID string, now time.Time) error`, el helper transaccional que usa `tagstore`.

- [ ] **Step 1: Write the migration.**
  - Tablas según la sección 3 de la especificación, con estas claves foráneas:
    - `user_id → users ON DELETE CASCADE`, `pet_id → pets ON DELETE CASCADE`, `tag_id → pet_tags ON DELETE CASCADE`;
    - `scan_id → tag_scans ON DELETE SET NULL`, `message_id → tag_messages ON DELETE SET NULL`;
    - `notification_deliveries.notification_id → notifications ON DELETE CASCADE`.
  - Restricciones `CHECK` de `kind`, `channel` y `status`.
  - Índices:
    - `notifications(user_id, created_at DESC, id DESC)`;
    - parcial `notifications(user_id) WHERE read_at IS NULL`;
    - parcial `deliveries(next_attempt_at) WHERE status='pending'`.
  - Trigger `set_entity_timestamps` en las deliveries.
- [ ] **Step 2: Write the failing integration tests.**
  - `TestInsertEventScanCreatesPendingEmail`: crea la notificación y una delivery `pending` con `next_attempt_at = now + 60 s`.
  - `TestInsertEventGroupsIntoPendingScanEmail`: un `finder_message` dentro del minuto combina `message` en el `payload` del `tag_scanned`, crea la notificación del panel y no crea una segunda delivery.
  - `TestInsertEventNoGroupingAfterDue`: con la delivery ya vencida o reclamada, el evento crea su propia delivery.
  - `TestInsertEventWithoutEmail`: con `Email=false`, crea la notificación sin delivery.
  - `TestClaimDueSkipLocked` (Review Focus 2): dos `ClaimDue` concurrentes sobre 30 vencidas se reparten sin repetir; con `limit` 20, la suma da 30.
  - `TestClaimDueReclaimsExpiredLease`: un envío reclamado y no marcado vuelve a salir después del lease.
  - `TestMarkRetryAndFailed`.
  - `TestListAndUnreadAreOwnerScoped`: el usuario B ve 0.
  - `TestMarkReadForeignIsNotFound` y `TestMarkAllRead`.
  - `TestClaimDueJoinsOwnerEmail`: `ToEmail` es el `users.email` del dueño y `OwnerName` su nombre completo.
- [ ] **Step 3:** Run `go run ./.tmp/migratecheck && go test -p 1 ./internal/infra/secondary/repositories/postgres/notifystore/` → FAIL.
- [ ] **Step 4: Implement.** La paginación keyset sigue el patrón de `tagstore.ListBatches`. `ClaimDue` hace todo en una transacción:
  1. `SELECT … WHERE status='pending' AND next_attempt_at <= $now ORDER BY next_attempt_at LIMIT $n FOR UPDATE SKIP LOCKED`;
  2. `UPDATE next_attempt_at = $now + lease`;
  3. join con `notifications` y `users`.
- [ ] **Step 5:** Run → PASS. Migraciones up, down y up hasta la 21.
- [ ] **Step 6: Commit** `feat: add notifications schema and store`.

### Task 3: Los eventos de la placa crean avisos en la misma transacción

**Files:**
- Modify: `internal/core/ports/tag.go`, `tagstore/scans.go` (+ tests), `internal/core/services/public_tag_service.go` (+ tests), `internal/infra/bootstrap/application.go`

**Interfaces:**
- Consumes: `notifystore.InsertEvent` (tarea 2) y `NotificationDraft` (tarea 1).
- Produces: cambios de firma en `TagScanPort`, cada uno con un último parámetro `draft *domain.NotificationDraft` (nil = sin aviso):
  - `RecordScan(ctx, scan, dedupeSince, draft)`: inserta el aviso solo cuando el escaneo es nuevo (`reused=false`).
  - `AddMessage(ctx, msg, draft)`.
  - `MarkPhoneRevealed(ctx, scanID, at, draft)`: solo la primera vez.
  - `PublicTagService` gana la opción `WithEmailNotifications(enabled bool)`. Construye el borrador con:
    - `UserID = tag.OwnerID`;
    - el nombre de la mascota, el código de la placa, la hora del escaneo y la etiqueta del dispositivo;
    - el mensaje en `finder_message`;
    - `PhoneRevealed` en `phone_revealed`.

- [ ] **Step 1: Write the failing tests.**
  - Servicio, con fakes:
    - `TestRecordScanBuildsScanDraft` (el dueño recibe el aviso; la etiqueta del dispositivo va traducida);
    - `TestSendMessageBuildsMessageDraft`;
    - `TestRevealContactBuildsPhoneDraft`;
    - `TestEmailFlagOff` (`Email=false` en todos los borradores).
  - Integración en `tagstore`:
    - `TestRecordScanReuseCreatesNoSecondNotification`;
    - `TestEventRollsBackWithNotification`: si la inserción del aviso falla, no queda ni escaneo ni mensaje. Se prueba con un `UserID` inexistente, que viola la clave foránea.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.** Actualiza los fakes de los tests de la parte 1 y el bootstrap, que pasa `cfg.Notifications.Enabled`.
- [ ] **Step 4:** Run `go test ./internal/core/... ./internal/infra/primary/...` y la integración de `tagstore` → PASS.
- [ ] **Step 5: Commit** `feat: notify owners when their tag is scanned`.

### Task 4: Compartir ubicación (backend)

**Files:**
- Modify:
  - `internal/core/ports/{tag.go,login_limiter.go}`;
  - `internal/infra/secondary/ratelimit/public.go` (+ test);
  - `tagstore/scans.go` (+ test);
  - `public_tag_service.go` (+ test);
  - `public_tag_controller.go` (+ HTTP test);
  - DTOs `dto/inputs/tags` y `dto/outputs/tags` (`ScanResponse` agrega `location`);
  - docs OpenAPI.

**Interfaces:**
- Produces:
  - `TagScanPort.SetLocation(ctx, scanID string, loc domain.ScanLocation, draft *domain.NotificationDraft) error`: hace `UPDATE … WHERE id=$1 AND location_at IS NULL`; si no actualiza ninguna fila y el escaneo existe, devuelve `ErrLocationAlreadyShared`.
  - `PublicTagService.ShareLocation(ctx, scanID, token string, lat, lng float64, accuracy int) error`: usa `authorizedScan` (placa activa y token válido) y luego `ValidateScanLocation`.
  - Scope `PublicTagLocationLimit = "public_tag_location"`, de 10 por hora.
  - Ruta `PUT /v1/public/scans/:scanId/location` con el body `{lat, lng, accuracy}` y la cabecera `X-Scan-Token`. Responde 204; 422 con errores por campo; 409 "Location already shared"; 404 igual que `/messages`.
  - `ScanResponse.location?: {lat, lng, accuracyM, at}`.

- [ ] **Step 1: Write the failing tests.**
  - `TestShareLocationValidatesAndStores`.
  - `TestShareLocationTwiceConflict` (Review Focus 4; integración y HTTP 409).
  - `TestShareLocationBadToken404`.
  - `TestLocationRateLimited`: la llamada 11 responde 429 con las cabeceras públicas.
  - `TestOwnerScansIncludeLocation`.
  - `TestPublicScopesTable`: incluye la nueva fila.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.** Si el escaneo todavía tiene pendiente su correo `tag_scanned`, la ubicación se agrupa por `InsertEvent`. Regenera OpenAPI y agrega el texto en español al mapa de traducciones del frontend en la tarea 9.
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat: let finders share their location`.

### Task 5: Configuración y adaptadores `Mailer`

**Files:**
- Modify: `appconfig/{config.go,load.go,validate.go}` (+ tests), `.env.example`, `internal/resources/properties.example.yml`
- Create: `internal/infra/secondary/mailer/{smtp.go,log.go,mailer_test.go}`

**Interfaces:**
- Produces:
  - `cfg.Notifications{Enabled bool; SMTP SMTPConfig{Host string; Port int; User, Password, From string}}`;
  - `mailer.NewSMTP(cfg) ports.Mailer`, con STARTTLS obligatorio salvo en `localhost` para pruebas, y un `multipart/alternative` con texto y HTML codificados en UTF-8 (asunto con encoded-word RFC 2047);
  - `mailer.NewLog(logger) ports.Mailer`, que registra el destinatario y el asunto, nunca el cuerpo;
  - `mailer.FromConfig(cfg, logger) ports.Mailer`.

- [ ] **Step 1: Write the failing tests.**
  - Configuración:
    - `TestNotificationsConfigFromEnv`;
    - `TestProductionRequiresSMTPWhenEnabled`;
    - `TestSMTPPasswordNeverInErrors`.
  - Adaptadores:
    - `TestBuildMessageMultipartUTF8`: el asunto "Alguien escaneó…" va codificado, con ambas partes y `To`/`From` correctos;
    - `TestSMTPSendAgainstFakeServer`: un servidor SMTP falso con `net.Listen` en 127.0.0.1 acepta `EHLO`, `MAIL`, `RCPT`, `DATA` y `QUIT` y recibe el mensaje;
    - `TestLogMailerDoesNotLogBody`.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.** Usa `net/smtp` con `smtp.PlainAuth` solo si hay usuario. Documenta las variables en ambos archivos de ejemplo.
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat: add mailer adapters and notification config`.

### Task 6: `NotificationDispatcher`

**Files:**
- Create: `internal/infra/lifecycle/notification_dispatcher.go` (+ `_test.go`)
- Modify: `internal/infra/bootstrap/application.go` (lo registra si `Enabled` y no `SkipPurger`)

**Interfaces:**
- Consumes: `DeliveryPort`, `Mailer`, `RenderNotificationEmail`, `NextDeliveryAttempt`, `cfg.Public.BaseURL` y `time.LoadLocation("America/Santiago")`.
- Produces: `NewNotificationDispatcher(store ports.DeliveryPort, mailer ports.Mailer, cfg NotificationDispatcherConfig{Interval, BatchSize, Lease, BaseURL}, logger)`, que implementa `initModules.Lifecycle` con el mismo patrón de inicio y parada que `AuditEventPurger`, y `runCycle(ctx)` testeable con un reloj `now` inyectable.

- [ ] **Step 1: Write the failing tests** (fakes de `DeliveryPort` y `Mailer` y reloj controlado):
  - `TestDispatcherSendsAndMarksSent`;
  - `TestDispatcherRetryLadder` (Review Focus 1): un mailer que siempre falla da los `next` en +1 min, +5 min, +30 min, +2 h y +6 h y luego `MarkFailed` en el sexto intento;
  - `TestDispatcherErrorTruncatedTo300`;
  - `TestDispatcherStopIsGraceful`.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.** Un fallo de un envío no detiene los demás del ciclo. El error del mailer queda en `last_error`, recortado a 300 caracteres.
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat: dispatch notification emails with retries`.

### Task 7: Limpieza por retención

**Files:**
- Create: `internal/infra/lifecycle/tag_retention_purger.go` (+ `_test.go`), los métodos `PurgeTagData` en `tagstore` (+ integración)
- Modify: `bootstrap/application.go`

**Interfaces:**
- Produces:
  - `TagScanPort.PurgeTagData(ctx, now time.Time) (clearedLocations, deletedScans, deletedNotifications int, error)`;
  - `NewTagRetentionPurger(store, cfg{Interval: 24h}, logger)`.

- [ ] **Step 1: Write the failing integration test** `TestPurgeTagData`:
  - un escaneo de 31 días queda sin coordenadas, y su aviso queda sin `payload.location`;
  - uno de 29 días no cambia;
  - un escaneo de 181 días se borra junto con sus mensajes;
  - un aviso de 181 días se borra junto con su delivery.
  Agrega además un test unitario del ciclo con un fake.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.** Para el payload: `UPDATE notifications SET payload = payload - 'location' WHERE created_at < now - 30d AND payload ? 'location'`.
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat: purge tag scan data by retention`.

### Task 8: API de avisos del panel

**Files:**
- Create: `internal/core/notification_usecase.go`, `services/notification_service.go` (+ test), `controller/notification_controller.go` (+ HTTP test), DTOs `dto/outputs/notifications`
- Modify: `bootstrap/application.go`, docs OpenAPI, `docs/CHANGELOG.md`, `docs/ARCHITECTURE.md`

**Interfaces:**
- Produces, con JWT y solo para el propio usuario:
  - `GET /v1/notifications?limit&cursor` → página `{limit, hasNext, nextCursor, data:[{id, kind, petId, tagId, scanId?, payload, readAt?, createdAt}]}`, con `payload` en camelCase como en la tarea 1;
  - `GET /v1/notifications/unread-count` → `{data:{count}}`;
  - `POST /v1/notifications/:id/read` → 204; ajena o inexistente → 404;
  - `POST /v1/notifications/read-all` → `{data:{updated}}`.

- [ ] **Step 1: Write the failing tests:**
  - `TestNotificationsOwnerScoped`;
  - `TestUnreadCount`;
  - `TestMarkReadForeign404`;
  - `TestReadAll`;
  - `TestNotificationsRequireAuth` (401).
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.** Regenera OpenAPI. Documenta en ARCHITECTURE el despachador, la retención y la configuración SMTP, y en CHANGELOG los cambios.
- [ ] **Step 4:** Run:
  - `go build ./... && go vet ./...`;
  - las suites unitarias;
  - la integración de `tagstore`, `notifystore` y `internal/test`;
  - las migraciones hasta la 21.

  Todo debe quedar en PASS.
- [ ] **Step 5: Commit** `feat: add notifications API`.

### Task 9: Frontend: tipos y cliente

**Files:**
- Modify: `src/lib/types.ts`, `src/lib/tags.ts` (+ test), `src/lib/api.ts` (+ test)
- Create: `src/lib/notifications.ts` (+ test)

**Interfaces:**
- Produces:
  - tipos `AppNotification`, `NotificationPayload` y `ScanLocation`;
  - `shareLocation(scanId, token, {lat, lng, accuracy})` (PUT con `X-Scan-Token`);
  - `notificationText(n): { title: string; body: string }` en español, por tipo:
    - `tag_scanned`: "Escanearon la placa de {petName}";
    - `finder_message`: "{petName}: te dejaron un mensaje";
    - `location_shared`: "{petName}: compartieron dónde la encontraron";
    - `phone_revealed`: "{petName}: alguien vio tu número";
  - `formatUnread(count)`: 0 → "", y más de 99 → "99+";
  - `mapsUrl(lat, lng)`;
  - la traducción "Location already shared" → "Ya compartiste tu ubicación".

- [ ] **Step 1: Write the failing tests** de cada función, incluido `formatUnread(150) === "99+"` (Review Focus 5).
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat: add notification and location client helpers`.

### Task 10: Frontend: "Compartir mi ubicación"

**Files:**
- Modify: `src/components/public-profile.tsx` (+ test), `src/app/globals.css`

**Interfaces:**
- Consumes: `shareLocation` (tarea 9) y el `withScan` ya existente, con reintento ante un token rotado.

- [ ] **Step 1: Write the failing tests** (con `navigator.geolocation` falso vía `vi.stubGlobal` o `Object.defineProperty`):
  - el botón secundario "Compartir mi ubicación" muestra primero la explicación "Tu ubicación ayuda a su familia a encontrarla. Solo la verá su familia." y el botón "Compartir ahora";
  - con permiso: se hace el PUT con lat, lng y `accuracy` redondeado, y aparece "¡Gracias! Su familia verá dónde la encontraste";
  - con el permiso denegado: "Sin problema, igual puedes dejar un mensaje" y no hay PUT;
  - un 409 se muestra como éxito (Review Focus 4);
  - sin `navigator.geolocation`, el botón no aparece;
  - mientras se pide la ubicación, el botón queda deshabilitado.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.** Usa `getCurrentPosition` con `{ enableHighAccuracy: true, timeout: 15000, maximumAge: 60000 }`. Si se agota el tiempo, muestra "No pudimos obtener tu ubicación. Igual puedes dejar un mensaje".
- [ ] **Step 4:** Run → PASS. También pasan typecheck y lint.
- [ ] **Step 5: Commit** `feat: let finders share their location from the public page`.

### Task 11: Frontend: campana y `/dashboard/avisos`

**Files:**
- Create:
  - `src/components/notification-bell.tsx` (+ test);
  - `src/components/notifications-list.tsx` (+ test);
  - `src/app/dashboard/avisos/page.tsx`.
- Modify: `src/components/app-shell.tsx` (monta la campana en la barra superior), `src/app/globals.css`

**Interfaces:**
- Produces:
  - `NotificationBell()`: consulta `unread-count` al montar, cada 60 s y al volver el foco a la ventana. Es un enlace a `/dashboard/avisos` con `aria-label` "Avisos, {n} sin leer" y una insignia `formatUnread`.
  - `NotificationsList()`:
    - lista paginada con "Ver más";
    - cada aviso muestra el título y el cuerpo de `notificationText`, la fecha en Chile, un enlace "Ver en el mapa" si hay ubicación y el mensaje si existe;
    - los no leídos aparecen resaltados;
    - al hacer clic se marca leído y se navega a `/dashboard/pets/{petId}`;
    - el botón "Marcar todo como leído";
    - si no hay avisos: "Aún no tienes avisos. Te contaremos aquí cuando alguien escanee una placa."

- [ ] **Step 1: Write the failing tests** (con `fetch` falso): la campana muestra 3, la insignia "99+" y nada con 0; la lista muestra los textos por tipo, el enlace al mapa, el clic que marca leído (POST) y `read-all`.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.** El test de `app-shell` ya falla en el runner local (usa `vi.mock`), así que la campana se prueba sola.
- [ ] **Step 4:** Run → PASS. También pasan typecheck y lint.
- [ ] **Step 5: Commit** `feat: add notification bell and inbox`.

### Task 12: Frontend: ubicación en "Últimos escaneos"

**Files:**
- Modify: `src/components/pet-scans.tsx` (+ test), `src/lib/types.ts`

- [ ] **Step 1: Write the failing test:** un escaneo con `location` muestra "Compartió su ubicación" con un enlace "Ver en el mapa" (`mapsUrl`, `target _blank`, `rel noopener noreferrer`); sin ubicación, no aparece.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5: Commit** `feat: show shared location in recent scans`.

### Task 13: Verificación final

- [ ] **Step 1:** Backend:
  - build, vet y todas las suites unitarias;
  - integración de `tagstore`, `petstore`, `notifystore` e `internal/test`;
  - migraciones up, down y up hasta la 21.
- [ ] **Step 2:** Frontend: typecheck, lint y el runner local en todos los archivos nuevos o tocados. Pide al dueño que corra `npm test` real.
- [ ] **Step 3:** Prueba manual guiada para el dueño, con `SMTP_*` sin configurar (adaptador Log): escanear, mensaje y ubicación en el primer minuto → un solo envío en el log; la campana; `/dashboard/avisos`; el mapa en "Últimos escaneos".
- [ ] **Step 4:** `superpowers:requesting-code-review` sobre la rama completa de ambos repos.
