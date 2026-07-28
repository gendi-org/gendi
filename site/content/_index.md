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

<div class="hx-mb-6">
{{< hextra/hero-button text="Get Started" link="docs" >}}
</div>

<div class="hx-mb-12 hx-text-sm hx-text-gray-500 dark:hx-text-gray-400">
Writing configuration with an AI agent? Point it at <a href="/llms.txt"><code>gendi.dev/llms.txt</code></a> — one fetch, the whole syntax.
</div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="No hand-written wiring"
    subtitle="The service graph lives in configuration, so main stops growing with every new service and the diff for a feature contains the feature, not the plumbing."
  >}}
  {{< hextra/feature-card
    title="Libraries can ship their wiring"
    subtitle="A package publishes its own gendi.yaml and consumers import it by module path. It travels as data: no container types in the library's API, nothing added to anyone else's dependency graph."
  >}}
  {{< hextra/feature-card
    title="A container you can just read"
    subtitle="One build function per service, direct calls, concrete types, no reflection. What is injected where is written out rather than inferred — legible in a diff and in a stack trace."
  >}}
  {{< hextra/feature-card
    title="Written as easily by tools as by hand"
    subtitle="A small declarative surface with a published JSON schema, where a wrong guess fails at generation with the offending line instead of compiling into something subtly wrong."
  >}}
  {{< hextra/feature-card
    title="Fails before the build"
    subtitle="Missing dependencies, type mismatches, circular references and unconvertible parameter defaults are all generation errors, each with a caret under the offending token."
  >}}
  {{< hextra/feature-card
    title="Composable configuration"
    subtitle="Imports with overrides, tagged collections, decorators with priorities, and custom compiler passes for project-wide conventions."
  >}}
{{< /hextra/feature-grid >}}
