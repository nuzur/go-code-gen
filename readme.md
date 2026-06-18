# nuzur Go Code Generator (`go-code-gen`)

`go-code-gen` is the official backend code generation engine for [nuzur](https://nuzur.com). It turns visual data models and schemas into fully functional, production-ready Go backend services.

---

## Features

* **Entity & Model Generation**: Produces data model definitions, validation checks, and mapping utilities between Go structures and Protobuf definitions.
* **Core Service Modules**: Automatically generates the data access layer using `sqlc`, alongside paginated list queries (`LIMIT PageSize+1` optimization) and CRUD modules.
* **Uber Fx Integration**: Generates the application bootstrap and wires up dependencies (servers, mappers, repositories) cleanly using the Uber Fx container.
* **Optional Fx Logger Injection**: Logger injection is optional; it defaults to a no-op `zap.NewNop()` logger if a custom logger is not provided.
* **Protobuf & gRPC Server Wireup**: Syncs proto definitions to the schema and generates gRPC handler boilerplate.
* **Standard Authentication Integration**: Supports JWT authentication servers and Keycloak middleware configurations out of the box.
* **Containerization & Deployment**: Builds standard `Dockerfile` and Helm charts matching your project specifications.
* **AI & LLM Integration Guidelines**: Automatically generates an `AI.md` file at the root of generated projects, providing context, directory structures, and guardrails for AI coding assistants.

---

## Codebase Architecture

```text
├── ai/                # AI.md assistant guidelines generation
├── auth/              # JWT & Keycloak middleware templates
├── config/            # Project ports and DB config templates
├── core/              # Core business modules and DB repo templates
├── docker/            # Dockerfile generation
├── entities/          # Entity structure and mapper templates
├── files/             # Embedding and template file utilities
├── githubactions/     # CI pipeline generation templates
├── helm/              # Helm chart templates
├── main/              # main.go generator
├── project/           # Project initialization models & parameters
├── proto/             # Protobuf schema and handler templates
├── strings/           # String casing manipulation helpers
├── templatefuncs/     # Go text/template function helpers
└── v1/                # Master Generate runner and integration tests
```

---

## Usage

You can import `go-code-gen` directly into your Go extension or generator workflow:

```go
import (
	"context"
	gocodegen "github.com/nuzur/go-code-gen/v1"
	"github.com/nuzur/go-code-gen/project"
)

func main() {
	params := &project.ProjectParams{
		Project:        proj,         // *nemgen.Project
		ProjectVersion: version,      // *nemgen.ProjectVersion
		RootPath:       "./output",
		Identifier:     "myproject",
		Module:         "github.com/example/myproject",
		// Configure your core, entity, auth, and proto generator blocks...
	}

	ctx := context.Background()
	err := gocodegen.Generate(ctx, params)
	if err != nil {
		panic(err)
	}
}
```

---

## Development & Testing

### Running Tests Locally

This repository includes unit tests for helper string packages, static syntax validation of all Go templates, and a full integration pipeline test that builds a mock schema and asserts file generation outputs.

To run the complete test suite:

```bash
go test -v ./...
```

---

## License

This software is licensed under the **Personal & Non-Commercial Use License (with Attribution)**. You may use this package for personal, educational, and non-commercial projects. Commercial distribution, deployment, or usage within a business entity is strictly prohibited without prior written consent from **nuzur, LLC**. See the [LICENSE](LICENSE) file for the full text.