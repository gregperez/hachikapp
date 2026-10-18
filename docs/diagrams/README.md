# Diagramas (Archify)

Diagramas interactivos de `base-golang`. Abra los `.html` en un navegador para
usar tema claro/oscuro, pan/zoom, búsqueda y vistas guiadas.

> El contenido está autorado en español. La UI fija del visor Archify permanece
> en inglés (`html lang` fallback).

- Arquitectura hexagonal:
  [HTML](./base-golang-architecture.html) ·
  [JSON](./base-golang-architecture.json)
- Arranque y apagado:
  [HTML](./base-golang-startup.html) ·
  [JSON](./base-golang-startup.workflow.json)
- Listado protegido con cursor:
  [HTML](./base-golang-paginated-request.html) ·
  [JSON](./base-golang-paginated-request.sequence.json)

## Regenerar

Desde la raíz del repositorio:

```sh
ARCHIFY="$HOME/.agents/skills/archify/bin/archify.mjs"
REPO="$(pwd)"
DIR=docs/diagrams

node "$ARCHIFY" deliver architecture "$DIR/base-golang-architecture.json" \
  "$DIR/base-golang-architecture.html" --quality showcase --repo-root "$REPO"

node "$ARCHIFY" deliver workflow "$DIR/base-golang-startup.workflow.json" \
  "$DIR/base-golang-startup.html" --quality showcase

node "$ARCHIFY" deliver sequence "$DIR/base-golang-paginated-request.sequence.json" \
  "$DIR/base-golang-paginated-request.html" --quality showcase
```
