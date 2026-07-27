---
title: Complete Example
weight: 11
description: "A single configuration exercising imports, parameters, tags, decorators and stdlib services together."
---

```yaml
# Import stdlib and base services
imports:
  - github.com/gendi-org/gendi/stdlib/gendi.yaml
  - ./services/base.yaml

# Configuration parameters
parameters:
  app_name: "MyApp"

  db_dsn: "postgres://localhost/myapp"

  http_timeout: "30s"

# Tag definitions
tags:
  handler:
    element_type: "github.com/myapp.Handler"
    sort_by: "priority"
    public: true

# Service definitions
services:
  database:
    constructor:
      func: "github.com/myapp/db.New"
      args:
        - "%db_dsn%"
    shared: true

  user_repo:
    constructor:
      func: "github.com/myapp/repo.NewUserRepository"
      args:
        - "@database"
    shared: true
    public: true

  home_handler:
    constructor:
      func: "github.com/myapp/handlers.NewHome"
      args:
        - "@user_repo"
    tags:
      - name: "handler"
        priority: 10

  api_handler:
    constructor:
      func: "github.com/myapp/handlers.NewAPI"
      args:
        - "@user_repo"
    tags:
      - name: "handler"
        priority: 20

  http_server:
    constructor:
      func: "github.com/myapp/server.New"
      args:
        - "%app_name%"
        - "!spread:!tagged:handler"
    public: true

  # Decorator for logging
  logging_middleware:
    constructor:
      func: "github.com/myapp/middleware.NewLogging"
      args:
        - "@.inner"
        - "@stdlib.logger"
    decorates: http_server
    decoration_priority: 10
```
