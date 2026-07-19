# DEEIX Chat Backend Documentation

This directory contains generated backend API documentation. Backend HTTP DTOs and Swagger annotations are the source of truth.

## API Documentation

- [Swagger JSON](./swagger.json)
- [Swagger YAML](./swagger.yaml)
- [Generated TypeScript contract](../../packages/api-contract/src/types.generated.ts)

The Swagger files and TypeScript contract are one generated unit. Never edit any of the generated files manually.

After changing a route, HTTP DTO, JSON or validation tag, response document, or Swagger annotation, regenerate from the repository root:

```bash
pnpm api:generate
```

The backend shortcut invokes the same workspace pipeline:

```bash
cd backend
make swagger
```

Check for drift without changing files:

```bash
pnpm api:check
```

Transport requiredness, optionality, and nullability must be fixed in the backend DTO. Frontend code consumes the generated contract and must not compensate for an incorrect Swagger schema.
