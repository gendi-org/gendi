# Tags

## Tag Declaration

Tags can be declared explicitly or created implicitly when referenced by services.

Explicit declaration:
```yaml
tags:
  payment.provider:
    element_type: "example.com/app/payments.Provider"
    sort_by: priority
    public: true
    autoconfigure: false
```

Implicit creation:
```yaml
services:
  provider.stripe:
    constructor:
      func: "app.NewStripeProvider"
    tags:
      - name: payment.provider
        priority: 100
```

Tags support a string shorthand — a plain string is equivalent to `{name: "..."}`:
```yaml
    tags:
      - payment.provider          # same as {name: payment.provider}
```

Only explicitly declared tags can be public.

Supported fields on tag declarations:
- `element_type` (optional, required for public or autoconfigure tags)
- `sort_by` (optional)
- `public` (optional)
- `autoconfigure` (optional)

## Element Type Inference

- `element_type` is optional in tag declarations
- When `public: true`, `element_type` is required
- When omitted, the element type is inferred from constructor arguments that use the tag via `!tagged:tag.name`
- If multiple constructors use the same tag, all must have compatible element types

## Compatibility Validation

- Service type must be assignable to `element_type` (declared or inferred)
- Otherwise, generation fails

## Tagged Injection

- Tagged injection produces `[]element_type`
- Sorting is by `sort_by` attribute if enabled, descending order (100 before 10)
- The attribute is read as an integer (numeric scalar or decimal string); a
  service whose tag omits it sorts as `0`, and equal values are ordered by
  service ID
- Without `sort_by`, the collection is ordered by service ID

## Public Tag Getters

Declared tags can expose a public getter when `public: true` is set:

```yaml
tags:
  payment.provider:
    element_type: "example.com/app/payments.Provider"
    public: true
```

Generated signature:
```go
func (c *Container) GetTaggedWithPaymentProvider() ([]payments.Provider, error)
```

## Auto-tagging

Auto-tagging is enabled on a tag by setting `autoconfigure: true`. The generator will add the tag to every service whose final type implements the tag's `element_type`.

Rules:
- `autoconfigure: true` is only valid on explicitly declared tags
- `element_type` is required and must be an interface type
- `autoconfigure: true` cannot be combined with `sort_by`
- Auto-tagging runs after decorator expansion
- Alias services are excluded
- Decorator inner services are created with `autoconfigure: false` and do not participate
- Explicit tags remain; auto-tagging only adds missing services
- Result ordering is deterministic (stable by service ID)
- Services can opt out via `autoconfigure: false` (or in `_default`)
- Auto-tagged collections may be empty

Decorator example:
```yaml
tags:
  handlers:
    element_type: "github.com/acme/app.Handler"
    autoconfigure: true

services:
  base:
    constructor:
      func: "github.com/acme/app.NewBaseHandler"
  decorated:
    constructor:
      func: "github.com/acme/app.NewDecoratedHandler"
      args:
        - "@.inner"
    decorates: "base"
    decoration_priority: 10
```

Only `decorated` is auto-tagged (if its final type implements `Handler`). `base` is not auto-tagged, and `.inner` services never participate.
