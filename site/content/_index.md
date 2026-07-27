---
layout: hextra-home
---

{{< hextra/hero-headline >}}
  Compile-time dependency injection&nbsp;<br class="sm:hx-block hx-hidden" />for Go
{{< /hextra/hero-headline >}}

<div class="hx-mt-6 hx-mb-6">
{{< hextra/hero-subtitle >}}
  Declare services in YAML, get a type-checked container in Go&nbsp;<br class="sm:hx-block hx-hidden" />with zero runtime reflection.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx-mb-12">
{{< hextra/hero-button text="Get Started" link="docs" >}}
</div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="Fails at generation time"
    subtitle="Missing dependencies, type mismatches, circular references and unconvertible parameter defaults are errors before the program is built — each reported with the offending YAML line."
  >}}
  {{< hextra/feature-card
    title="No reflection, no autowiring"
    subtitle="Generated containers use direct calls and type assertions. Every dependency is written down; inference is limited to types, never to wiring."
  >}}
  {{< hextra/feature-card
    title="Composable configuration"
    subtitle="Imports with overrides, tagged collections, decorators with priorities, and custom compiler passes for project-specific conventions."
  >}}
{{< /hextra/feature-grid >}}
