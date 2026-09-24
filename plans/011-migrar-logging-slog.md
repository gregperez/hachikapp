# Plan 011 — Migrar logging a log/slog

## Objetivo

Sustituir `github.com/sirupsen/logrus` por `log/slog` (stdlib) en call sites de
base-golang, con niveles, atributos y correlación de `request_id` equivalentes.

- Prioridad: P2
- Esfuerzo: M
- Depende de: plan 009 (política de deps) y 010 (cierre)
- Rama sugerida: `codex/011-logging-slog`

## Motivación

El plan 009 dejó Logrus en `v1.9.4` (patch-only) y documentó diferir `v1.10+`
porque introduce interop slog y cambios de formateo. Este plan es el seguimiento
explícito (O-027).

## Alcance

- Reemplazar imports de Logrus en `cmd/` e `internal/`.
- Definir un logger de proceso inyectable (sin global mutable).
- Actualizar Dependabot para dejar de ignorar minors de Logrus (o eliminarlo).
- Pruebas de smoke de niveles y atributos en recovery/access log.

## Fuera de alcance

- Migrar authBase/initModules.
- Cambiar backend de telemetría (OTel) en el mismo PR.

## Criterios de aceptación

- [ ] Ningún import productivo de Logrus permanece.
- [ ] `go.mod` no requiere Logrus (salvo transitivo justificado).
- [ ] `make verify` verde.
- [ ] O-027 marcada Resuelto con commit.
