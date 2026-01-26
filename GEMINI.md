# Gemini Context for `cron`

## Project Overview

**cron** (v3) is a robust cron library for the Go programming language. It implements a cron spec parser and job runner, allowing developers to schedule tasks to run at specific times or intervals.

**Key Features:**
*   **Cron Spec Parsing:** Supports standard cron format (minute-level) by default, with an option to enable seconds (Quartz-compatible).
*   **Job Management:** Allows adding and removing jobs, inspecting running entries.
*   **Interceptors (JobWrappers):** Provides a chainable middleware pattern for cross-cutting concerns like logging, panic recovery, and job synchronization (skip-if-still-running, delay-if-still-running).
*   **Extensible Logging:** Supports key/value logging interfaces (compatible with `go-logr/logr`).

## Building and Running

This project follows standard Go conventions.

### Prerequisites
*   Go 1.11 or later (Go 1.20 is used in CI).

### Key Commands

*   **Build:**
    ```bash
    go build -v ./...
    ```

*   **Test:**
    ```bash
    go test -v ./...
    ```

*   **Install (as dependency):**
    ```bash
    go get github.com/robfig/cron/v3@v3.0.0
    ```

## Development Conventions

*   **Project Structure:** Flat structure with core logic in root (`cron.go`, `parser.go`, `spec.go`).
*   **Testing:** Comprehensive unit tests are co-located with source files (`*_test.go`).
*   **Versioning:** Uses Go Modules (`go.mod`). This is version 3 (`v3`) of the library.
*   **CI/CD:** Uses GitHub Actions for building and testing (workflow defined in `.github/workflows/go.yml`).

## Quick Usage Example

```go
package main

import (
	"fmt"
	"github.com/robfig/cron/v3"
)

func main() {
	c := cron.New()
	c.AddFunc("@every 1s", func() { fmt.Println("tick") })
	c.Start()

	// ... keep the program running ...
	select {}
}
```
