# Startup configuration and model definitions

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
