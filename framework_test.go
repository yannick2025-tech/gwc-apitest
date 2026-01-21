package apitest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Helper function to setup a mock HTTP server
func setupMockServer() *httptest.Server {
	mux := http.NewServeMux()

	// Handler for specific user ID (e.g., GET /users/1)
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/1") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, `{"id": 1, "name": "John Doe", "email": "john.doe@example.com"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound) // Default for other /users/ paths or methods
	})

	// Handler for /users (e.g., POST /users)
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "username") { // Basic check for body content
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				io.WriteString(w, `{"id": 2, "message": "User created"}`)
			} else {
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, `{"error": "Invalid request body"}`)
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	return httptest.NewServer(mux)
}

// TestMain sets up and tears down common resources for all tests in this package.
func TestMain(m *testing.M) {
	// Initialize a logger for tests, pointing to a dummy config or a minimal one
	// This prevents panics if the logger is accessed during tests
	// Run tests
	code := m.Run()

	os.Exit(code)
}

func TestNewTestRunner(t *testing.T) {
	tempDir := t.TempDir()

	// Create a dummy valid YAML file
	validYamlPath := filepath.Join(tempDir, "valid.yaml")
	os.WriteFile(validYamlPath, []byte(`
suite:
  name: "Test Suite"
  baseURL: "http://localhost"
scenarios:
  - name: "Default"
    testcases:
      - name: "Test Case 1"
        request: { method: "GET", path: "/test" }
        expect: { status_code: 200 }
`), 0644)

	// Create a dummy invalid YAML file (missing 'response' for example)
	invalidYamlPath := filepath.Join(tempDir, "invalid.yaml")
	os.WriteFile(invalidYamlPath, []byte(`
suite:
  name: "Test Suite"
  baseURL: "http://localhost"
scenarios:
  - name: "Default"
    testcases:
      - name: "Test Case 1"
        request: { method: "GET", path: "/test" }
        expect: { status_code: 200 # This line makes it syntactically invalid, with unclosed bracket
`), 0644)

	// Test case 1: Valid configuration
	runner, err := NewTestRunner(validYamlPath, nil, &MockCleanupHandler{})
	if err != nil {
		t.Fatalf("NewTestRunner failed for valid YAML: %v", err)
	}
	if runner == nil {
		t.Fatal("NewTestRunner returned nil for valid YAML")
	}
	if runner.suite.Suite.Name != "Test Suite" {
		t.Errorf("Expected suite name 'Test Suite', got '%s'", runner.suite.Suite.Name)
	}
	totalTestCases := 0
	for _, s := range runner.suite.Scenarios {
		totalTestCases += len(s.TestCases)
	}
	if totalTestCases != 1 {
		t.Errorf("Expected 1 test case, got %d", totalTestCases)
	}

	// Test case 2: Invalid YAML syntax/structure
	invalidContent, readErr := os.ReadFile(invalidYamlPath) // Read with error handling
	if readErr != nil {
		t.Fatalf("Failed to read invalid YAML file: %v", readErr)
	}
	t.Logf("Invalid YAML file content: \n%s", string(invalidContent))
	runner, err = NewTestRunner(invalidYamlPath, nil, &MockCleanupHandler{})
	if err == nil {
		t.Fatal("NewTestRunner should have failed for invalid YAML")
	}
	if runner != nil {
		t.Fatal("NewTestRunner should return nil for invalid YAML")
	}

	// Test case 3: Non-existent file
	runner, err = NewTestRunner("non_existent_file.yaml", nil, &MockCleanupHandler{})
	if err == nil {
		t.Fatal("NewTestRunner should have failed for non-existent file")
	}
	if runner != nil {
		t.Fatal("NewTestRunner should return nil for non-existent file")
	}
}

func TestTestRunnerRun(t *testing.T) {
	server := setupMockServer()
	defer server.Close()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")
	t.Logf("Server URL for TestTestRunnerRun: %s", server.URL)
	os.WriteFile(configPath, []byte(`
suite:
  name: "Integration Test Suite"
  base_url: "`+server.URL+`"
scenarios: # Changed from 'tests' to 'scenarios'
  - name: "User Flow"
    testcases:
      - name: "GET User Endpoint Success"
        request: { method: "GET", path: "/users/1" }
        expect: { status_code: 200, body: { contains: ["John Doe"] } }
      - name: "POST User Endpoint Success"
        request: { method: "POST", path: "/users", body: { username: "test_user" } }
        expect: { status_code: 201, body: { contains: ["User created"] } }
  - name: "Error Handling"
    testcases:
      - name: "GET User Endpoint Not Found"
        request: { method: "GET", path: "/users/999" }
        expect: { status_code: 404 }
      - name: "POST User Endpoint Invalid Body (Expected 400)"
        request: { method: "POST", path: "/users", body: { invalid_field: "value" } }
        expect: { status_code: 400, body: { contains: ["Invalid request body"] } }
  - name: "Negative Test"
    testcases:
      - name: "POST User Endpoint Validation Fail (Expected 201, got 400)"
        request: { method: "POST", path: "/users", body: { invalid_field: "value" } }
        expect: { status_code: 201 }
`), 0644)

	runner, err := NewTestRunner(configPath, nil, &MockCleanupHandler{})
	if err != nil {
		t.Fatalf("Failed to create TestRunner: %v", err)
	}
	t.Logf("BaseURL in runner.suite.Suite: %s", runner.suite.Suite.BaseURL) // Added debug log

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := runner.Run(ctx); err != nil {
		t.Fatalf("TestRunner.Run failed: %v", err)
	}

	results := runner.GetResults()
	if len(results) != 5 {
		t.Errorf("Expected 5 test results, got %d", len(results))
	}

	// Check individual test results
	if !results[0].Passed {
		t.Errorf("Test '%s' failed unexpectedly: %s", results[0].Name, results[0].Error)
	}
	if !results[1].Passed {
		t.Errorf("Test '%s' failed unexpectedly: %s", results[1].Name, results[1].Error)
	}
	if !results[2].Passed {
		t.Errorf("Test '%s' failed unexpectedly: %s", results[2].Name, results[2].Error)
	}
	if !results[3].Passed {
		t.Errorf("Test '%s' failed unexpectedly: %s", results[3].Name, results[3].Error)
	}
	if results[4].Passed {
		t.Errorf("Test '%s' passed unexpectedly", results[4].Name)
	}
	if results[4].Error == "" {
		t.Errorf("Expected error for failed test '%s', but got none", results[4].Name)
	}

	// Test with setup/teardown in config
	configWithCleanupPath := filepath.Join(tempDir, "test_config_with_cleanup.yaml")
	os.WriteFile(configWithCleanupPath, []byte(`
suite:
  name: "Cleanup Test Suite"
  base_url: "`+server.URL+`"
  setup:
    - type: "cleanup"
  teardown:
    - type: "cleanup"
scenarios:
  - name: "Default"
    testcases:
      - name: "Simple GET"
        request: { method: "GET", path: "/users/1" }
        expect: { status_code: 200 }
`), 0644)

	mockCleanup := &MockCleanupHandler{}
	setupCalled := false
	teardownCalled := false
	mockCleanup.ExecuteFunc = func(ctx context.Context, action SetupAction) error {
		if action.Type == "cleanup" {
			setupCalled = true
			teardownCalled = true
		}
		return nil
	}

	runnerWithCleanup, err := NewTestRunner(configWithCleanupPath, nil, mockCleanup)
	if err != nil {
		t.Fatalf("Failed to create TestRunner with cleanup: %v", err)
	}

	if err := runnerWithCleanup.Run(ctx); err != nil {
		t.Fatalf("TestRunner.Run with cleanup failed: %v", err)
	}

	if !setupCalled {
		t.Errorf("Expected setup action to be called, but it wasn't")
	}
	if !teardownCalled {
		t.Errorf("Expected teardown action to be called, but it wasn't")
	}
}

func TestTestRunnerExportResults(t *testing.T) {
	tempDir := t.TempDir()
	exportFilePath := filepath.Join(tempDir, "results.json")

	// Create a dummy runner with some results
	runner := &TestRunner{
		suite: &TestSuite{
			Suite: SuiteConfig{Name: "Dummy Suite"},
		},
		results: []TestResult{
			{Name: "Test 1", Passed: true},
			{Name: "Test 2", Passed: false, Error: "mock error"},
		},
	}

	err := runner.ExportResults(exportFilePath)
	if err != nil {
		t.Fatalf("ExportResults failed: %v", err)
	}

	// Verify the file was created and contains some content
	fileInfo, err := os.Stat(exportFilePath)
	if err != nil {
		t.Fatalf("Exported file not found: %v", err)
	}
	if fileInfo.Size() == 0 {
		t.Errorf("Exported file is empty")
	}
}
