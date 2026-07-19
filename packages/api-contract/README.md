# @deeix/api-contract

TypeScript data contracts generated from the Go backend's Swagger 2.0 document.

The source of truth is the backend Go DTOs and Swagger annotations. The generator uses the pinned `swag` tool from `backend/go.mod`, writes the backend Swagger artifacts, and then derives TypeScript from the same generated `swagger.json`. Runtime HTTP behavior, authentication, retries, and error handling remain in the frontend API layer; this package contains types only.

From the workspace root:

```bash
pnpm api:generate
pnpm api:check
```

`pnpm api:generate` updates `backend/docs/{docs.go,swagger.json,swagger.yaml}` and `packages/api-contract/src/types.generated.ts` together. `cd backend && make swagger` invokes this same pipeline.

DTO field conventions:

- Fields serialized on every response are required by default; do not repeat their requiredness in frontend types.
- Request or response fields that may be omitted must use `json:",omitempty"`.
- A pointer response field whose key is always present but whose value may be `null` must use `extensions:"x-nullable,!x-omitempty"`.
- An optional nullable field uses both `json:",omitempty"` and `extensions:"x-nullable"`.
- Required request booleans and numbers that accept `false` or `0` use pointers with `binding:"required"`, then are dereferenced after successful binding.

Consume the package with type-only imports:

```ts
import type { Admin, UserDataResponse } from "@deeix/api-contract";

type CreateUserBody = Admin.UsersCreate.RequestBody;
type CreateUserResult = Admin.UsersCreate.ResponseBody;
```

`pnpm api:check` regenerates both layers in a temporary directory and fails when Go, Swagger, or TypeScript contracts have drifted. `pnpm check` runs the same drift check and TypeScript validation through Turborepo. Never edit generated Swagger or TypeScript contract files manually.
