# `gwc-apitest`: API Integration Test Runner

`gwc-apitest` is a powerful and flexible command-line interface (CLI) tool designed for running API integration tests. It allows developers to define complex test scenarios using YAML configuration files, execute them against live API endpoints, and obtain detailed results for validation. This package was extracted from the `gwc` monorepo to be an independent, reusable library for robust API testing.

## ✨ Features

-   **YAML-based Test Definitions:** Define comprehensive API test cases, including requests, assertions, and setup/teardown actions, in human-readable YAML files.
-   **Flexible Test Execution:**
    -   Run single test files.
    -   Execute all tests within a directory.
    -   Recursively search subdirectories for test files.
    -   Apply custom file matching patterns (e.g., `*_api_test.yaml`).
-   **Database Cleanup Integration:** Includes a `DBCleanupHandler` for performing soft deletes or executing custom SQL to ensure a clean test environment. Supports mock cleanup for scenarios without database access.
-   **Comprehensive Reporting:** Provides an overall summary of test results, including passed, failed, and success rates.
-   **Result Export:** Export detailed test results to JSON files for further analysis or integration with other tools.
-   **Configurable Parameters:** Customize test runs via command-line flags for database connection, API base URL, configuration paths, and more.
-   **CI/CD Friendly:** Designed for seamless integration into continuous integration and continuous deployment pipelines, with examples for GitHub Actions and Jenkins.

## 🚀 Getting Started

### Installation

To use `gwc-apitest` as a standalone tool, clone this repository and build it:

```bash
git clone https://github.com/yannick2025-tech/gwc-apitest.git
cd gwc-apitest
go build -o apitest ./cmd/apitest
```

### Usage

The `apitest` executable (or `go run ./cmd/apitest`) accepts several command-line arguments to control test execution.

```bash
./apitest -config [config_path] [options...]
# or
go run ./cmd/apitest -config [config_path] [options...]
```

**Required Argument:**

-   `-config <path>`: Specifies the path to a test configuration file (e.g., `user_api_test.yaml`) or a directory containing multiple test files.

**Common Options:**

-   `-pattern <glob>`: (Default: `*.yaml`) Glob pattern to match test files when a directory is specified in `-config`.
-   `-recursive <bool>`: (Default: `true`) Whether to recursively search subdirectories for test files.
-   `-db <dsn>`: Database DSN for cleanup operations (e.g., `"user:pass@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True"`). If not provided, a mock cleanup handler is used.
-   `-url <url>`: API base URL to override what's specified in test configurations. (Default: `http://localhost:9090`)
-   `-export <path>`: Directory path to export detailed JSON test results.

### Examples

**1. Run a single test file:**

```bash
go run ./cmd/apitest -config testcases/user_api_test.yaml
```

**2. Run all tests in a directory (recursively):**

```bash
go run ./cmd/apitest -config testcases/
```

**3. Run tests with a specific pattern and database cleanup:**

```bash
go run ./cmd/apitest \
  -config testcases/ \
  -pattern "*_regression_test.yaml" \
  -db "root:password@tcp(127.0.0.1:3306)/my_app_test?charset=utf8mb4&parseTime=True" \
  -url http://localhost:8080 \
  -export results/
```

Refer to the `userguid.MD` in the `docs` directory for more detailed usage examples, recommended directory structures, and advanced scenarios.

## 📂 Project Structure (within `gwc-apitest` module)

```
gwc-apitest
├── cleanup.go
├── framework_test.go
├── framework.go
├── go.mod
├── go.sum
├── LICENSE
├── logs
├── readme.MD
├── testdata
│   ├── invalid_test.yaml
│   ├── log.yaml
│   └── valid_test.yaml
└── userguid.MD
```

## 🛠️ Development

### Prerequisites

-   Go (version 1.25.0 or later recommended)

### Running Tests

If this module had its own internal unit/integration tests, you would run them using `go test ./...`. For the API integration tests defined as YAML, you would use this tool itself.

## 🤝 Contributing

Contributions are welcome! Please feel free to open issues or submit pull requests.

## 📄 License

This project is licensed under the Apache License, Version 2.0 - see the [LICENSE](LICENSE) file for details.
Copyright 2026 Yannick.Xiong
