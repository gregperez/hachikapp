# Ficha pública: diseño

- **Fecha:** 2026-10-18
- **Tarjeta Trello:** "Ficha publica" (App Pets)
- **Repos:** `hachikapp-backend` (Go) y `hachikapp-frontend` (Next.js)
- **Estado:** aprobado por partes en conversación; pendiente la revisión de este documento.

## 1. Objetivo

Cuando alguien encuentra una mascota y escanea su Hachikapp Tag (NFC o QR), ve una página pública con la información de emergencia que el dueño decidió mostrar. Desde ahí puede avisar a la familia sin conocer su número, llamar si el dueño lo permite y compartir su ubicación. El dueño recibe un aviso por email y en el panel en cada escaneo.

Criterios de éxito:

- La página carga rápido en el celular de un desconocido, sin cuenta ni app.
- Ningún dato oculto (por la visibilidad elegida o por regla de privacidad) sale del servidor.
- Cada escaneo real genera exactamente un aviso al dueño (sin duplicados por recargas ni falsos por vistas previas de enlaces).
- Ningún aviso se pierde si el proveedor de email falla o el servidor se reinicia.

## 2. Decisiones tomadas

| Tema | Decisión |
|---|---|
| Alcance | Perfil público + contacto + aviso al dueño + ubicación de quien escanea |
| Canal de aviso | Email + aviso en el panel |
| Contacto | Mensaje mediado (principal) y "Llamar / WhatsApp" que pide el número al servidor solo al tocarlo |
| Código | Por placa. Cada placa (NFC + QR con el mismo enlace) tiene un código propio que se activa |
| Creación de códigos | Admin por lotes (CSV para fábrica) + "placa digital" creada por el dueño (solo QR) |
| Envío de avisos | Enfoque A: tabla de envíos pendientes en la misma transacción + proceso en segundo plano con reintentos |

Fuera de alcance:

- **Modo perdido.** Es otra tarjeta; mientras no exista, los campos `lost_only` se tratan como ocultos.
- Push del navegador, WhatsApp saliente y chips NTAG 424 DNA (enlaces firmados).
- PIN de activación y captcha.

## 3. Modelo de datos (migración 000019 en adelante)

### `tag_batches`
`id uuid`, `label varchar(80)`, `quantity int (1..1000)`, `created_by uuid → users`, `created_at`.

### `pet_tags`
| Columna | Notas |
|---|---|
| `id uuid` | PK |
| `code char(10)` | Único. Alfabeto Crockford base32 sin I, L, O, U (32^10 ≈ 1,1·10^15) |
| `kind` | `physical` \| `digital` |
| `batch_id uuid null` | Solo físicas de fábrica |
| `status` | `unassigned` \| `active` \| `disabled` |
| `pet_id uuid null → pets ON DELETE SET NULL` | |
| `owner_id uuid null → users` | Quien la activó; se conserva al liberarla |
| `activated_at`, `disabled_at`, `created_at`, `updated_at` | |

Restricciones:

- `status = 'active'` implica `pet_id` y `owner_id` no nulos.
- Máximo 5 placas activas por mascota (lo valida el servicio).
- Un trigger o el servicio, al borrar la mascota, deja la placa en `unassigned` y conserva `owner_id`.

### `tag_scans`
`id uuid`, `tag_id`, `pet_id`, `scanned_at`, `visitor_hash char(64)` (SHA-256 del id anónimo del visitante + pepper del servidor), `scan_token_hash char(64)`, `ip_prefix varchar(64)` (/24 o /48), `user_agent_family varchar(40)`, `latitude/longitude numeric(9,6) null`, `accuracy_m int null`, `location_at null`, `phone_revealed_at null`.

Retención: las coordenadas se ponen en `NULL` a los 30 días y la fila se borra a los 180 días (proceso de limpieza).

### `tag_messages`
`id uuid`, `scan_id → tag_scans`, `tag_id`, `pet_id`, `body varchar(500)`, `finder_name varchar(80)`, `finder_phone varchar(20)`, `created_at`. Se borran a los 180 días.

### `notifications`
`id uuid`, `user_id → users`, `kind` (`tag_scanned` \| `location_shared` \| `finder_message` \| `phone_revealed`), `pet_id`, `tag_id`, `scan_id null`, `message_id null`, `payload jsonb` (resumen para mostrar; sin secretos), `read_at null`, `created_at`. Índice `(user_id, created_at desc, id desc)`.

### `notification_deliveries`
`id uuid`, `notification_id`, `channel` (`email`), `status` (`pending` \| `sent` \| `failed`), `attempts int`, `next_attempt_at`, `last_error varchar(300)`, `sent_at null`, `created_at`, `updated_at`. Índice parcial sobre `next_attempt_at WHERE status = 'pending'`.

## 4. Placas: ciclo de vida

- **Enlace:** `https://<PUBLIC_BASE_URL>/t/<code>`. El NFC (NTAG213/215, registro URI) y el QR impreso llevan el mismo enlace.
- **Lote (admin, permiso nuevo `TAGS`):** `POST /v1/admin/tag-batches {label, quantity ≤ 1000}` crea las placas `unassigned`. `GET /v1/admin/tag-batches/{id}/codes.csv` devuelve `code,url`. El listado de lotes se pagina con cursor.
- **Activación:** con la placa `unassigned`, la página pública muestra "Esta placa aún no está activada". El dueño inicia sesión y llama a `POST /v1/tags/{code}/activate {petId}`. Reglas:
  - Funciona si la placa está `unassigned` y (`owner_id` es nulo o igual al usuario).
  - Si la placa está `active` o `disabled`, responde 409.
  - Si la mascota no es del usuario, responde 404.
  - Si la mascota ya tiene 5 placas activas, responde 409.
- **Placa digital:** `POST /v1/pets/{id}/tags` crea una placa `digital`, `active`, ya vinculada.
- **Desactivar:** `POST /v1/pets/{id}/tags/{tagId}/disable` cambia el estado a `disabled`. No hay reactivación en esta entrega (YAGNI).
- **Listado:** `GET /v1/pets/{id}/tags`.
- **Generación de códigos:** usa `crypto/rand`; si choca con el índice único, se reintenta.

## 5. Página pública

### API: `GET /v1/public/tags/{code}` (sin login)
Responde `{ state: "active" | "unassigned" | "disabled", profile? }`. Un código con formato inválido o inexistente responde 404. Cabeceras: `Cache-Control: no-store` y `X-Robots-Tag: noindex`.

Proyección `domain.PublicProfileOf(pet, lostMode=false)`:

- `name`: siempre.
- `photoUrl`: `/v1/public/tags/{code}/photo?v=…`, solo si `photo` es visible. La ruta pública de la foto aplica la misma regla.
- `species`, `breed`, `color`, `sex`, `size`, `distinctiveFeatures`, `allergies`, `specialConditions`, `surgeries` y `temperament`: según su visibilidad.
- `age`: texto aproximado ("5 años", "8 meses"); nunca la fecha.
- `ownerName` y `secondOwnerName`: **solo el nombre de pila** (primera palabra).
- `comuna`: según su visibilidad. **Nunca se muestra si contiene dígitos.**
- `canContact`: `contactButton` visible. Nunca el teléfono.
- `lost: false`. Un campo es visible si su valor es `always`, o si es `lost_only` y `lost` es verdadero.

Cambio relacionado en la ficha: la validación de `owner.comuna` rechaza dígitos (código `no_digits`).

### Frontend: `/t/[code]`
Ruta pública fuera de `/dashboard`, renderizada en el servidor con `fetch` a `BACKEND_URL` y sin cookies. Lleva la metaetiqueta `robots noindex`, está pensada primero para celular y usa los colores del manual. En orden:

1. Foto y "Hola, soy {name}".
2. Tarjeta destacada con alergias y condiciones especiales, si son visibles.
3. Botón principal "Avisar a mi familia".
4. Botones secundarios "Llamar / WhatsApp" (si `canContact`) y "Compartir mi ubicación".
5. El resto de los datos visibles.
6. Al pie: "Quien encuentre a tu mascota puede contactarte. Tú decides qué ve."

Estados especiales:

- **`unassigned`:** "Esta placa aún no está activada" + botón "Activar", que lleva a `/login?from=/t/{code}/activar`. La página `/t/[code]/activar` pide sesión y muestra la lista de mascotas del dueño.
- **`disabled`:** mensaje neutro, sin datos.
- **404:** "No encontramos esta placa".
- **Error del backend:** "Algo falló, intenta de nuevo", con los botones deshabilitados.

## 6. Escaneos, ubicación y contacto

### Escaneo
Lo registra el cliente cuando la página ya cargó (no en el render del servidor), para que las vistas previas de WhatsApp o iMessage y los buscadores no generen avisos falsos.

- `POST /v1/public/tags/{code}/scans {visitorId}`: `visitorId` es un UUID guardado en `localStorage`; si no se puede guardar, se usa uno por sesión.
- Respuesta: `{ scanId, scanToken }`. El token solo se guarda como hash.
- Si el mismo `visitor_hash` escanea la misma placa dentro de 10 minutos, se reutiliza el escaneo y no se crea otro aviso. Se devuelve un token nuevo para ese escaneo.
- Solo se registra si la placa está `active`.

### Ubicación
"Compartir mi ubicación" primero explica para qué sirve y luego usa `navigator.geolocation`.

- `PUT /v1/public/scans/{scanId}/location {lat, lng, accuracy}`, con la cabecera `X-Scan-Token`. Valida rangos y acepta una sola actualización por escaneo (devuelve 409 si ya hay ubicación).
- Genera el aviso `location_shared`. Si el envío del aviso del escaneo sigue pendiente, se actualiza ese mismo aviso y no se crea otro.
- Si la persona rechaza el permiso: "Sin problema, igual puedes dejar un mensaje".

### Mensaje
`POST /v1/public/scans/{scanId}/messages {body, finderName?, finderPhone?, website}`, con `X-Scan-Token`.

- `website` es un campo trampa: si viene con contenido, se responde 202 y no se guarda nada.
- Validación: `body` de 1 a 500 caracteres, nombre hasta 80 y teléfono con las mismas reglas de la ficha.
- Genera el aviso `finder_message`.

### Llamar / WhatsApp
`POST /v1/public/scans/{scanId}/contact`, con `X-Scan-Token`.

- Responde `{ phone, whatsappUrl }` solo si `contactButton` es visible; si no, 404.
- Devuelve únicamente `owner.phone`, nunca `phoneAlt`.
- Marca `phone_revealed_at` y genera el aviso `phone_revealed`, uno por escaneo.

### Límites (limitador en memoria existente; para varias instancias habrá que pasar a Redis)
| Ruta | Límite |
|---|---|
| `GET /v1/public/tags/{code}` y su foto | 60 por minuto por IP |
| `POST …/scans` | 20 por hora por IP |
| `POST …/messages` | 5 por hora por IP y placa |
| `POST …/contact` | 3 por hora por IP y placa |
| `PUT …/location` | 10 por hora por IP |

Al superar el límite se responde 429 con un mensaje claro.

## 7. Avisos

### Creación
El escaneo, la ubicación, el mensaje y el contacto crean `notifications` y `notification_deliveries(email, pending)` **en la misma transacción** que el evento.

### Proceso de envío (`NotificationDispatcher`)
Es un lifecycle registrado como los purgers existentes (arranca después de la base de datos y se detiene antes de cerrarla).

- Cada 5 s toma hasta 20 envíos con `status = 'pending' AND next_attempt_at <= now()` usando `FOR UPDATE SKIP LOCKED`, y los envía por el puerto `Mailer`.
- **Si el envío funciona:** marca `sent`.
- **Si falla:** suma un intento y programa el siguiente a 1 min, 5 min, 30 min, 2 h y 6 h. Después del sexto intento lo marca `failed`. El aviso sigue visible en el panel.
- **Agrupación:** al crear los avisos `location_shared` o `finder_message` para un escaneo cuyo email de escaneo sigue `pending`, se actualiza el `payload` de esa notificación y no se crea un envío nuevo. La notificación del panel se crea igual.

### Puerto `Mailer`
`Send(ctx, Email{To, Subject, Text, HTML}) error`, con dos adaptadores:

- **SMTP:** `net/smtp` con STARTTLS. Se configura con `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD` y `SMTP_FROM`.
- **Log:** escribe en el log y es el valor por defecto si no hay SMTP. En producción, la validación exige SMTP cuando `notifications.enabled` está activo.

### Email
- **Asunto:** "Alguien escaneó el Hachikapp Tag de {name}".
- **Cuerpo:** hora en `America/Santiago`, enlace `https://maps.google.com/?q=lat,lng` si hay ubicación, mensaje y contacto de quien la encontró si existen, y un botón "Ver en Hachikapp". Va en texto plano y en HTML simple con la paleta del manual.
- **Tono:** cálido y sin alarmismo, según la sección 12 del manual.
- **Destinatario:** `users.email` del dueño.

### API del panel
- `GET /v1/notifications?limit&cursor`: paginación keyset, igual que usuarios.
- `GET /v1/notifications/unread-count`.
- `POST /v1/notifications/{id}/read` y `POST /v1/notifications/read-all`.
- `GET /v1/pets/{id}/scans?limit&cursor`: escaneos con ubicación y mensajes.

### Frontend del panel
- Campana en la barra superior con el contador de no leídos, que lleva a `/dashboard/avisos`.
- En la ficha de la mascota:
  - sección "Placas", con lista, estado, "Crear placa digital" (QR descargable como SVG/PNG) y "Desactivar";
  - sección "Últimos escaneos".
- Página de admin "Placas" (permiso `TAGS`) para crear lotes y descargar el CSV.

## 8. Errores

- **Público:** nunca muestra un error técnico; usa los mensajes de la sección 5. Si falla el registro del escaneo, el perfil se muestra igual. Las respuestas públicas no incluyen detalles internos.
- **Dueño y admin:** `application/problem+json` con errores por campo, como la ficha.
  - Placa ya vinculada: 409.
  - Límite de 5 placas: 409.
  - Lote de más de 1000: 422.
  - Mascota o placa ajena: 404.
- **Garantía del test HTTP:** las respuestas públicas, salvo `/contact`, nunca contienen el teléfono, la dirección, el número de chip ni el apellido del dueño.

## 9. Pruebas (TDD)

- **Dominio:**
  - `PublicProfileOf` en cada combinación de visibilidad, con `lost_only` oculto mientras `lost=false`;
  - solo nombre de pila;
  - comuna con dígitos;
  - edad aproximada;
  - generador de códigos (alfabeto, largo, sin repetidos en N muestras);
  - reglas de transición de estado de las placas.
- **Servicios:**
  - activación (propietario previo, límite de 5);
  - desactivación;
  - agrupación de escaneos en 10 minutos;
  - agrupación de avisos pendientes;
  - teléfono solo con `contactButton`;
  - validación del token del escaneo.
- **HTTP:**
  - las rutas públicas no exponen datos ocultos (asserts sobre el JSON completo);
  - los límites responden 429;
  - el campo trampa;
  - código inválido devuelve 404.
- **Integración con PostgreSQL:**
  - migraciones (subir, bajar, subir);
  - `SKIP LOCKED` con dos procesos;
  - limpieza a los 30 y 180 días;
  - `ON DELETE SET NULL` de las placas.
- **Proceso de envío:** reintentos y escalera de espera con `Mailer` falso y reloj controlado.
- **Frontend:**
  - página pública en sus 4 estados;
  - mensaje;
  - "Llamar" pide el número solo al tocarlo;
  - ubicación aceptada y rechazada;
  - campana;
  - sección Placas;
  - página de admin;
  - capturas en celular y escritorio.

## 10. Dependencias nuevas

- **Frontend:** `qrcode` (MIT) para generar el QR de la placa digital. Hay que instalarla en el equipo del dueño (`npm install qrcode @types/qrcode`) antes de esa tarea, porque este entorno no tiene acceso a npm.
- **Backend:** ninguna. Se usan `net/smtp`, `crypto/rand` y `crypto/sha256` de la biblioteca estándar.

## 11. Configuración nueva

`PUBLIC_BASE_URL` (para armar enlaces y el CSV), `TAG_VISITOR_PEPPER` (secreto), `SMTP_*` y `NOTIFICATIONS_ENABLED`. Se documentan en `properties.example.yml` y `.env.example`.

## 12. Entrega

Dos partes, cada una en su rama por repo, con revisión de código antes de `main`:

1. **Placas, página pública y contacto:** secciones 3 a 6, sin la ubicación.
2. **Avisos (email y panel) y ubicación:** sección 7 y la ubicación de la sección 6.
