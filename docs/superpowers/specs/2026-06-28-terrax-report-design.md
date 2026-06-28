# terrax report — Design Spec

**Date:** 2026-06-28
**Status:** Approved

## Overview

`terrax report` es un nuevo subcomando que lee los JSON plan files generados por Terragrunt
(`--json-out-dir`) y produce un reporte detallado de cambios pendientes por stack, equivalente
a `terraform show` pero derivado puramente del JSON plan — sin requerir el binario de plan ni
`.terragrunt-cache`. Complementa a `terrax summary` (que solo muestra conteos) y a
`terrax review` (que abre la TUI interactiva).

## Interfaz CLI

```
terrax report [flags]

Flags:
  --dir <path>        Directorio de trabajo (default: cwd)
  --plans-dir <path>  Override de plan.json_out_dir en .terrax.yaml
  --format <fmt>      Formato de salida: text | markdown  (default: text)
  --output <file>     Archivo de salida (default: stdout)
  --all               Incluir stacks sin cambios (default: solo stacks con cambios)
```

Ejemplos:

```bash
terrax report                                          # texto a stdout, solo cambios
terrax report --format markdown                        # markdown a stdout
terrax report --format markdown --output plan-report.md
terrax report --all                                    # incluye stacks sin cambios
```

## Archivos nuevos

| Archivo | Propósito |
|---------|-----------|
| `internal/plan/reporter.go` | Lógica de renderizado (text y markdown) |
| `internal/plan/reporter_test.go` | Tests tabla-driven |
| `cmd/report.go` | Subcomando Cobra — solo orquestación |

Sin modificaciones a archivos existentes.

## Arquitectura

```
cmd/report.go
  → resuelve flags (--dir, --plans-dir, --format, --output, --all)
  → plan.CollectFromJSONDir(ctx, jsonDir, runDir)   ← ya existe
  → plan.Report(report, ReportOptions{...})
        → renderText(w, report, opts)
        → renderMarkdown(w, report, opts)
              → renderStackText / renderStackMarkdown
                    → diffAttributes(before, after)
```

`cmd/report.go` no contiene lógica de negocio. Toda la transformación vive en
`internal/plan/reporter.go`, que solo depende del paquete `internal/plan` (tipos ya
definidos en `models.go`).

## Componente: `internal/plan/reporter.go`

### Tipos

```go
type Format string

const (
    FormatText     Format = "text"
    FormatMarkdown Format = "markdown"
)

type ReportOptions struct {
    Format  Format
    ShowAll bool      // si false, omite stacks sin cambios
    Writer  io.Writer // destino de salida
}
```

### API pública

```go
func Report(report *PlanReport, opts ReportOptions) error
```

### Lógica de diffAttributes

- Recibe `before interface{}` y `after interface{}` (ya parseados por el collector como
  `map[string]interface{}`).
- Compara top-level keys únicamente (sin recursión en objetos anidados — se serializan
  como JSON compacto para esta versión).
- Clasifica cada atributo en: added (solo en after), removed (solo en before), changed
  (distinto en ambos), unchanged (igual — omitido).
- Los campos marcados como `unknown` en `ResourceChange.Unknown` se muestran como
  `(computed)`.
- Los valores `null` en before/after se omiten salvo que el campo cambie de/a null.

## Formatos de salida

### Text

```
workloads/dev/acm                                    +2 ~1 -0
────────────────────────────────────────────────────────────
  + aws_acm_certificate.main (aws_acm_certificate)
      domain_name:               "new.example.com"
      subject_alternative_names: ["new.example.com"]

  ~ aws_route53_record.validation (aws_route53_record)
      name:  "old.example.com" → "new.example.com"
      type:  (computed)

  - aws_s3_bucket.old (aws_s3_bucket)
      bucket: "my-bucket"

────────────────────────────────────────────────────────────
Summary: 3 stacks · 2 with changes · +3 ~1 -1
```

Colores Lipgloss:
- `+` recursos nuevos → verde
- `~` recursos modificados → amarillo
- `-` recursos eliminados → rojo
- Atributos → gris/dim
- Separadores → gris

### Markdown

````markdown
## `workloads/dev/acm` — +2 ~1 -0

### + `aws_acm_certificate.main`
| Attribute | Value |
|-----------|-------|
| `domain_name` | `"new.example.com"` |

### ~ `aws_route53_record.validation`
| Attribute | Before | After |
|-----------|--------|-------|
| `name` | `"old.example.com"` | `"new.example.com"` |
| `type` | | *(computed)* |

### - `aws_s3_bucket.old`
| Attribute | Value |
|-----------|-------|
| `bucket` | `"my-bucket"` |

---
**Summary:** 3 stacks · 2 with changes · +3 ~1 -1
````

## Manejo de errores

| Situación | Comportamiento |
|-----------|---------------|
| Stack con JSON inválido | Advertencia inline (`⚠ <path>: <error>`), continúa |
| `--output` no escribible | Error fatal antes de procesar |
| Ningún plan encontrado | Mensaje informativo, exit 0 |
| Formato desconocido | Error de validación de flag, exit 1 |

## Tests (`internal/plan/reporter_test.go`)

Table-driven, inline JSON, sin filesystem real. Casos mínimos:

- Create: atributos de `after` mostrados
- Update: diff before → after, campos sin cambio omitidos
- Delete: atributos de `before` mostrados
- Replace: mismo que update pero marcado como replace
- No-op: omitido en output por defecto; incluido con `--all`
- Stack con error: advertencia visible en output
- `ShowAll: false` (default) vs `ShowAll: true`
- `FormatText` vs `FormatMarkdown`
- Valores computed (`unknown: true`)

`cmd/report.go` no tiene tests propios (es delegación pura).

## Decisiones de diseño

- **Top-level diff únicamente (v1):** Objetos anidados se serializan como JSON compacto.
  Suficiente para el caso de uso principal; recursión completa es trabajo futuro.
- **Sin nuevo paquete:** El renderer vive en `internal/plan/` para aprovechar los tipos
  existentes sin cross-package imports adicionales.
- **Colores solo en FormatText:** Markdown no usa estilos Lipgloss; produce texto plano
  compatible con GitHub, GitLab y cualquier visor.
- **`--output` abre/cierra el archivo en `cmd/report.go`:** El `io.Writer` inyectado
  mantiene `reporter.go` libre de dependencias de OS.
