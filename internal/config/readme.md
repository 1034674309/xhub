# Startup configuration and model definitions

[简体中文](readme_cn.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Load configuration at process startup, then use it to initialize IAM, Store, and gateway.New. Loading validates YAML and PostgreSQL URL syntax, without testing service connectivity.

| Entry point or resource | Usage and result |
| --- | --- |
| Load(path) | Read a file and return *Config,error |
| ModelEntry.ParamString(key, fallback) | Read a connection parameter with fallback |
| ModelEntry.Disabled() | Read an explicit disabled flag |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo /tmp/xhub-demo.yaml` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/config"
	"os"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	if len(os.Args) != 2 {
		panic("usage: demo <config.yaml>")
	}
	cfg, err := config.Load(os.Args[1])
	if err != nil {
		panic(err)
	}
	fmt.Println(cfg.RouterSettings.RoutingStrategy, cfg.RouterSettings.NumRetries, cfg.RouterSettings.Timeout)
}
```

First save this fixture as /tmp/xhub-demo.yaml:

```yaml
general_settings:
  database_url: postgres://localhost/demo
```

Pass the YAML path as an argument. A file containing only general_settings.database_url: postgres://localhost/demo prints random 2 60. No database connection is opened.

### Errors, state, and side effects

Unset os.environ/NAME references expand to empty strings; zero values trigger defaults. Do not print the entire Config because it may contain credentials.

## Overview

The config package loads the gateway YAML startup document into typed process settings while retaining undeclared router and general-setting keys for later database overlays. It defines model deployments, routing defaults, database and Redis locations, bootstrap administrator settings, the optional emergency master credential, and spend-log switches. Loading reads files and environment variables; it does not connect to services or modify the file.

## External interfaces

- `Load(path string) (*Config, error)` reads YAML, expands supported environment references, validates the database URL, applies defaults, and resolves string deployment parameters. File and YAML errors pass through. A missing database URL or a scheme other than postgres/postgresql returns a validation error. Process startup calls it before constructing gateway, router, IAM, and data-plane services.
- `ModelEntry.Disabled() bool` is true only when `model_info.disabled` is the boolean true; missing or differently typed values remain enabled. Discovery and routing use it when selecting deployments.
- `ModelEntry.ParamString(key, fallback string) string` returns a non-empty string from `litellm_params` or the fallback, avoiding unsafe type assertions in router and provider selection.
- `Config` exposes typed model, router, LiteLLM, guardrail, and general settings. `RouterRaw` and `GeneralRaw` preserve YAML maps for per-key database overlays; preservation does not imply runtime support for every key.

The package has no HTTP boundary. Commands and tests load it; gateway, router, and provider packages consume the result.

## Core implementation

Load decodes YAML twice: once into typed structures and once into raw maps. It resolves `os.environ/NAME` for master key, database and Redis URLs, bootstrap administrator fields, and string values in each deployment's `litellm_params`. An unset variable becomes empty. It then requires PostgreSQL and assigns defaults when typed values are zero: routing strategy random, two retries, and a 60-second timeout. Explicit zero therefore cannot be distinguished from omission for those fields.

Administrator credentials seed an absent account only in gateway startup; later YAML changes do not rewrite a stored password. `disable_env_credential_login` suppresses seeding. The optional master key is an emergency API credential, not a console login. Resolved secrets remain in memory and must not be logged.

### Test dependencies

`config_test.go` covers defaults, raw maps, environment expansion, administrator fields, optional master key, and disabled deployments. `url_test.go` covers database schemes. Run `go test ./internal/config -count=1`; tests use temporary files and environment variables without a live database.

## Source responsibilities and entry points

- `config.go` contains all production types and operations: Config, ModelEntry, RouterSettings, GeneralSettings, Load, environment resolution, disabled-deployment interpretation, and string parameter access.
- `config_test.go` verifies parsing and defaults; `url_test.go` isolates URL validation.
