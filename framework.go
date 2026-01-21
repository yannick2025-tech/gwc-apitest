// Package apitest 提供自动生成go api测试用例的功能
package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yannick2025-tech/gwc-db"
	"github.com/yannick2025-tech/gwc-safejson"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// TestSuite 测试套件配置
type TestSuite struct {
	Suite     SuiteConfig `yaml:"suite"`
	Variables Variables   `yaml:"variables"`
	Scenarios []Scenario  `yaml:"scenarios"`
}

// SuiteConfig 套件配置
type SuiteConfig struct {
	Name     string        `yaml:"name"`
	BaseURL  string        `yaml:"base_url"`
	Setup    []SetupAction `yaml:"setup"`
	Teardown []SetupAction `yaml:"teardown"`
}

// SetupAction 设置/清理动作
type SetupAction struct {
	Type      string         `yaml:"type"`      // cleanup, soft_delete_cleanup, sql, api_call
	Table     string         `yaml:"table"`     // 表名
	Condition string         `yaml:"condition"` // WHERE 条件
	SQL       string         `yaml:"sql"`       // 自定义 SQL
	Request   *RequestConfig `yaml:"request"`   // API 调用配置
}

// Variables 变量定义
type Variables map[string]any

// Scenario 测试场景（业务流程分组）
type Scenario struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	TestCases   []TestCase `yaml:"testcases"`
}

// TestCase 测试用例
type TestCase struct {
	Name      string            `yaml:"name"`
	DependsOn string            `yaml:"depends_on"`
	Request   RequestConfig     `yaml:"request"`
	Expect    ExpectConfig      `yaml:"expect"`
	Save      map[string]string `yaml:"save"`
	Retry     *RetryConfig      `yaml:"retry"`
}

// RequestConfig 请求配置
type RequestConfig struct {
	Method  string            `yaml:"method"`
	Path    string            `yaml:"path"`
	Headers map[string]string `yaml:"headers"`
	Body    map[string]any    `yaml:"body"`
	Query   map[string]string `yaml:"query"`
}

// ExpectConfig 期望配置
type ExpectConfig struct {
	StatusCode   int            `yaml:"status_code"`
	ResponseBody map[string]any `yaml:"response_body"` // 用于校验 code 等字段
	Assertions   []Assertion    `yaml:"assertions"`
}

// Assertion 断言配置
type Assertion struct {
	Path     string `yaml:"path"`
	Operator string `yaml:"operator"`
	Value    any    `yaml:"value"`
}

// RetryConfig 重试配置
type RetryConfig struct {
	Times    int `yaml:"times"`
	Interval int `yaml:"interval"` // 毫秒
}

// TestRunner 测试运行器
type TestRunner struct {
	suite     *TestSuite
	client    *http.Client
	variables Variables
	results   []TestResult
	cleanup   CleanupHandler
	dbAdapter db.DBAdapter // 数据库适配器，用于软删除清理
}

// TestResult 测试结果
type TestResult struct {
	Scenario string        `json:"scenario"`
	Name     string        `json:"name"`
	Passed   bool          `json:"passed"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
	Response *ResponseData `json:"response,omitempty"`
}

// ResponseData 响应数据
type ResponseData struct {
	StatusCode int                 `json:"status_code"`
	Body       map[string]any      `json:"body"`
	Headers    map[string][]string `json:"headers"`
}

// CleanupHandler 清理处理器接口
type CleanupHandler interface {
	Execute(ctx context.Context, action SetupAction) error
}

// NewTestRunner 创建测试运行器
func NewTestRunner(configPath string, dbAdapter db.DBAdapter, cleanup CleanupHandler) (*TestRunner, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var suite TestSuite
	if err := yaml.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &TestRunner{
		suite:     &suite,
		client:    &http.Client{Timeout: 30 * time.Second},
		variables: suite.Variables,
		cleanup:   cleanup,
		dbAdapter: dbAdapter,
	}, nil
}

// Run 运行所有测试
func (r *TestRunner) Run(ctx context.Context) error {
	fmt.Printf("🚀 Running test suite: %s\n", r.suite.Suite.Name)
	fmt.Printf("📍 Base URL: %s\n\n", r.suite.Suite.BaseURL)

	// 执行 setup
	if err := r.executeSetup(ctx); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	// 执行测试场景
	for _, scenario := range r.suite.Scenarios {
		fmt.Printf("📦 Scenario: %s\n", scenario.Name)
		if scenario.Description != "" {
			fmt.Printf("   %s\n", scenario.Description)
		}

		for _, tc := range scenario.TestCases {
			result := r.runTestCase(ctx, scenario.Name, tc)
			r.results = append(r.results, result)

			if result.Passed {
				fmt.Printf("   ✓ %s (%.2fs)\n", result.Name, result.Duration.Seconds())
			} else {
				fmt.Printf("   ✗ %s (%.2fs): %s\n", result.Name, result.Duration.Seconds(), result.Error)
			}
		}
		fmt.Println()
	}

	// 执行 teardown
	if err := r.executeTeardown(ctx); err != nil {
		fmt.Printf("⚠️  Warning: teardown failed: %v\n", err)
	}

	// 打印摘要
	r.printSummary()

	return nil
}

// runTestCase 运行单个测试用例
func (r *TestRunner) runTestCase(ctx context.Context, scenario string, tc TestCase) TestResult {
	start := time.Now()
	result := TestResult{
		Scenario: scenario,
		Name:     tc.Name,
		Passed:   false,
	}

	// 检查依赖
	if tc.DependsOn != "" && !r.isDependencyPassed(tc.DependsOn) {
		result.Error = fmt.Sprintf("dependency '%s' not passed", tc.DependsOn)
		result.Duration = time.Since(start)
		return result
	}

	// 构建请求
	req, err := r.buildRequest(tc.Request)
	if err != nil {
		result.Error = fmt.Sprintf("build request failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	// 发送请求（支持重试）
	var resp *http.Response
	retryTimes := 1
	retryInterval := 0

	if tc.Retry != nil {
		retryTimes = tc.Retry.Times
		retryInterval = tc.Retry.Interval
	}

	for i := 0; i < retryTimes; i++ {
		resp, err = r.client.Do(req)
		if err == nil {
			break
		}

		if i < retryTimes-1 {
			time.Sleep(time.Duration(retryInterval) * time.Millisecond)
			// 重新构建请求（因为 Body 已经被读取）
			req, _ = r.buildRequest(tc.Request)
		}
	}

	if err != nil {
		result.Error = fmt.Sprintf("request failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("read response failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	// 🔧 修改点1: 使用 safejson 解析响应，避免大整数精度丢失
	var respData map[string]any
	if len(body) > 0 {
		respData, err = safejson.SafeUnmarshalToMap(body)
		if err != nil {
			result.Error = fmt.Sprintf("parse response failed: %v", err)
			result.Duration = time.Since(start)
			return result
		}
	}

	result.Response = &ResponseData{
		StatusCode: resp.StatusCode,
		Body:       respData,
		Headers:    resp.Header,
	}

	// 验证期望
	if err := r.validateExpectation(tc.Expect, resp.StatusCode, respData); err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(start)
		return result
	}

	// 保存变量
	if tc.Save != nil {
		r.saveVariables(tc.Save, respData)
	}

	result.Passed = true
	result.Duration = time.Since(start)
	return result
}

// buildRequest 构建 HTTP 请求
func (r *TestRunner) buildRequest(cfg RequestConfig) (*http.Request, error) {
	// 替换路径中的变量
	path := r.replaceVariables(cfg.Path)
	url := r.suite.Suite.BaseURL + path

	// 构建请求体
	var body io.Reader
	if cfg.Body != nil {
		bodyData := r.replaceMapVariables(cfg.Body)

		// 使用自定义 JSON 编码器，确保 int64 不会被序列化为科学计数法
		var buf bytes.Buffer
		encoder := json.NewEncoder(&buf)
		encoder.SetEscapeHTML(false)

		if err := encoder.Encode(bodyData); err != nil {
			return nil, fmt.Errorf("failed to encode request body: %w", err)
		}

		body = &buf
	}

	req, err := http.NewRequest(cfg.Method, url, body)
	if err != nil {
		return nil, err
	}

	// 设置请求头
	for k, v := range cfg.Headers {
		req.Header.Set(k, r.replaceVariables(v))
	}

	// 设置查询参数
	if cfg.Query != nil {
		q := req.URL.Query()
		for k, v := range cfg.Query {
			q.Add(k, r.replaceVariables(v))
		}
		req.URL.RawQuery = q.Encode()
	}

	return req, nil
}

// replaceVariables 替换字符串中的变量
func (r *TestRunner) replaceVariables(s string) string {
	result := s

	// 替换 UUID
	for strings.Contains(result, "{{uuid}}") {
		result = strings.Replace(result, "{{uuid}}", uuid.New().String(), 1)
	}

	// 替换自定义变量
	for k, v := range r.variables {
		placeholder := fmt.Sprintf("{{%s}}", k)
		if strings.Contains(result, placeholder) {
			// 🔧 修改点2: 根据变量类型进行格式化，支持更多整数类型
			var strValue string
			switch val := v.(type) {
			case int64:
				strValue = fmt.Sprintf("%d", val)
			case uint64:
				strValue = fmt.Sprintf("%d", val)
			case int:
				strValue = fmt.Sprintf("%d", val)
			case int32:
				strValue = fmt.Sprintf("%d", val)
			case uint:
				strValue = fmt.Sprintf("%d", val)
			case uint32:
				strValue = fmt.Sprintf("%d", val)
			case float64:
				// float64 检查是否为整数
				if val == float64(int64(val)) {
					strValue = fmt.Sprintf("%d", int64(val))
				} else {
					strValue = fmt.Sprintf("%f", val)
				}
			default:
				strValue = fmt.Sprint(v)
			}
			result = strings.ReplaceAll(result, placeholder, strValue)
		}
	}

	return result
}

// replaceMapVariables 替换 map 中的变量
func (r *TestRunner) replaceMapVariables(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		switch val := v.(type) {
		case string:
			// 字符串类型，尝试替换变量
			replaced := r.replaceVariables(val)

			// 🔧 关键修复: 检查是否是变量替换（原值和替换后不同）
			if replaced != val {
				// 如果是变量替换的结果，尝试智能转换为正确的类型

				// 1. 检查原始变量值的类型
				varName := extractVarName(val)
				if varName != "" {
					if varValue, exists := r.variables[varName]; exists {
						// 直接使用变量的原始类型
						result[k] = varValue
						continue
					}
				}

				// 2. 如果找不到变量，尝试从字符串解析数字
				if isNumericString(replaced) {
					if num, ok := parseNumber(replaced); ok {
						result[k] = num
						continue
					}
				}
			}

			// 如果不是变量替换，或无法转换，保持字符串
			result[k] = replaced

		case map[string]any:
			result[k] = r.replaceMapVariables(val)
		case []any:
			// 处理数组类型
			arr := make([]any, len(val))
			for i, item := range val {
				if itemStr, ok := item.(string); ok {
					replaced := r.replaceVariables(itemStr)
					if replaced != itemStr && isNumericString(replaced) {
						if num, ok := parseNumber(replaced); ok {
							arr[i] = num
							continue
						}
					}
					arr[i] = replaced
				} else {
					arr[i] = item
				}
			}
			result[k] = arr
		default:
			result[k] = v
		}
	}
	return result
}

// extractVarName 从占位符中提取变量名
// 例如: "{{test_user_id}}" -> "test_user_id"
func extractVarName(s string) string {
	if !strings.HasPrefix(s, "{{") || !strings.HasSuffix(s, "}}") {
		return ""
	}

	varName := s[2 : len(s)-2]
	varName = strings.TrimSpace(varName)

	// 确保只包含一个变量，没有其他文本
	if strings.Contains(varName, "{{") || strings.Contains(varName, "}}") {
		return ""
	}

	return varName
}

// isNumericString 检查字符串是否表示数字
func isNumericString(s string) bool {
	if s == "" {
		return false
	}
	// 检查科学计数法或纯数字
	for i, c := range s {
		if i == 0 && c == '-' {
			continue
		}
		if (c < '0' || c > '9') && c != '.' && c != 'e' && c != 'E' && c != '+' {
			return false
		}
	}
	return true
}

// parseNumber 将字符串解析为数字
func parseNumber(s string) (any, bool) {
	// 尝试解析为 int64
	var i int64
	if _, err := fmt.Sscanf(s, "%d", &i); err == nil {
		return i, true
	}

	// 尝试解析为 float64
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
		// 检查是否可以安全转换为 int64
		if f == float64(int64(f)) {
			return int64(f), true
		}
		return f, true
	}

	return nil, false
}

// validateExpectation 验证期望结果
func (r *TestRunner) validateExpectation(expect ExpectConfig, statusCode int, respData map[string]any) error {
	fmt.Printf("DEBUG: validateExpectation - Expect.StatusCode: %d, Actual Status Code: %d\n", expect.StatusCode, statusCode) // ADDED DEBUG
	// 验证状态码
	if expect.StatusCode != 0 && expect.StatusCode != statusCode {
		return fmt.Errorf("status code mismatch: expected %d, got %d", expect.StatusCode, statusCode)
	}

	// 验证 response_body 中的字段（code、data 等）
	if expect.ResponseBody != nil {
		for key, expectedValue := range expect.ResponseBody {
			actualValue, ok := respData[key]
			if !ok {
				return fmt.Errorf("field '%s' not found in response", key)
			}

			// 特殊处理 null 值
			if expectedValue == nil {
				if actualValue != nil {
					return fmt.Errorf("field '%s' expected null, got %v", key, actualValue)
				}
				continue
			}

			// 比较值
			if fmt.Sprint(actualValue) != fmt.Sprint(expectedValue) {
				return fmt.Errorf("field '%s' mismatch: expected %v, got %v", key, expectedValue, actualValue)
			}
		}
	}

	// 执行断言
	for _, assertion := range expect.Assertions {
		if err := r.executeAssertion(assertion, respData); err != nil {
			return err
		}
	}

	return nil
}

// executeAssertion 执行断言
func (r *TestRunner) executeAssertion(assertion Assertion, data map[string]any) error {
	value := r.getValueByPath(assertion.Path, data)
	expectedValue := assertion.Value

	// 如果期望值是字符串,替换变量
	if strVal, ok := expectedValue.(string); ok {
		expectedValue = r.replaceVariables(strVal)
	}

	switch assertion.Operator {
	case "equals":
		// 🔧 修改点3: 使用类型安全的比较
		if !valuesEqual(value, expectedValue) {
			return fmt.Errorf("assertion failed: %s should equal %v (type: %T), got %v (type: %T)",
				assertion.Path, expectedValue, expectedValue, value, value)
		}
	case "notEquals":
		if valuesEqual(value, expectedValue) {
			return fmt.Errorf("assertion failed: %s should not equal %v", assertion.Path, expectedValue)
		}
	case "contains":
		str := fmt.Sprint(value)
		substr := fmt.Sprint(expectedValue)
		if !strings.Contains(str, substr) {
			return fmt.Errorf("assertion failed: %s should contain %s, got %s", assertion.Path, substr, str)
		}
	case "startsWith":
		str := fmt.Sprint(value)
		prefix := fmt.Sprint(expectedValue)
		if !strings.HasPrefix(str, prefix) {
			return fmt.Errorf("assertion failed: %s should start with %s, got %s", assertion.Path, prefix, str)
		}
	case "notEmpty":
		if value == nil || value == "" {
			return fmt.Errorf("assertion failed: %s should not be empty", assertion.Path)
		}
	case "isArray":
		if _, ok := value.([]any); !ok {
			return fmt.Errorf("assertion failed: %s should be array", assertion.Path)
		}
	case "greaterThan":
		numVal, ok := toFloat64(value)
		if !ok {
			return fmt.Errorf("assertion failed: %s should be number", assertion.Path)
		}
		expectedNum, ok := toFloat64(expectedValue)
		if !ok {
			return fmt.Errorf("assertion failed: expected value should be number")
		}
		if numVal <= expectedNum {
			return fmt.Errorf("assertion failed: %s should be greater than %v, got %v", assertion.Path, expectedNum, numVal)
		}
	case "greaterThanOrEqual":
		numVal, ok := toFloat64(value)
		if !ok {
			return fmt.Errorf("assertion failed: %s should be number", assertion.Path)
		}
		expectedNum, ok := toFloat64(expectedValue)
		if !ok {
			return fmt.Errorf("assertion failed: expected value should be number")
		}
		if numVal < expectedNum {
			return fmt.Errorf("assertion failed: %s should be >= %v, got %v", assertion.Path, expectedNum, numVal)
		}
	case "lessThan":
		numVal, ok := toFloat64(value)
		if !ok {
			return fmt.Errorf("assertion failed: %s should be number", assertion.Path)
		}
		expectedNum, ok := toFloat64(expectedValue)
		if !ok {
			return fmt.Errorf("assertion failed: expected value should be number")
		}
		if numVal >= expectedNum {
			return fmt.Errorf("assertion failed: %s should be less than %v, got %v", assertion.Path, expectedNum, numVal)
		}
	default:
		return fmt.Errorf("unknown operator: %s", assertion.Operator)
	}

	return nil
}

// 🔧 新增函数: 类型安全的值比较
func valuesEqual(a, b any) bool {
	// 如果类型完全相同，直接比较
	if fmt.Sprintf("%T", a) == fmt.Sprintf("%T", b) {
		return fmt.Sprint(a) == fmt.Sprint(b)
	}

	// 尝试将两个值都转换为 int64 进行比较
	aInt, aIsInt := toInt64(a)
	bInt, bIsInt := toInt64(b)

	if aIsInt && bIsInt {
		return aInt == bInt
	}

	// 尝试浮点数比较
	aFloat, aIsFloat := toFloat64(a)
	bFloat, bIsFloat := toFloat64(b)

	if aIsFloat && bIsFloat {
		return aFloat == bFloat
	}

	// 字符串比较
	return fmt.Sprint(a) == fmt.Sprint(b)
}

// 🔧 新增函数: 转换为 int64
func toInt64(v any) (int64, bool) {
	switch val := v.(type) {
	case int64:
		return val, true
	case uint64:
		if val <= 9223372036854775807 { // max int64
			return int64(val), true
		}
	case int:
		return int64(val), true
	case int32:
		return int64(val), true
	case uint:
		return int64(val), true
	case uint32:
		return int64(val), true
	case float64:
		if val == float64(int64(val)) {
			return int64(val), true
		}
	case string:
		var i int64
		if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
			return i, true
		}
	}
	return 0, false
}

// toFloat64 将 any 转换为 float64
func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case uint64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint32:
		return float64(val), true
	default:
		return 0, false
	}
}

// getValueByPath 通过路径获取值
func (r *TestRunner) getValueByPath(path string, data map[string]any) any {
	parts := strings.Split(path, ".")
	var current any = data

	for _, part := range parts {
		// 处理数组索引 data[0]
		if strings.Contains(part, "[") {
			arrayPart := part[:strings.Index(part, "[")]
			indexPart := part[strings.Index(part, "[")+1 : strings.Index(part, "]")]

			if m, ok := current.(map[string]any); ok {
				current = m[arrayPart]
			}

			if arr, ok := current.([]any); ok {
				var index int
				fmt.Sscanf(indexPart, "%d", &index)
				if index < len(arr) {
					current = arr[index]
				} else {
					return nil
				}
			}
		} else {
			if m, ok := current.(map[string]any); ok {
				current = m[part]
			} else {
				return nil
			}
		}
	}

	return current
}

// saveVariables 保存变量
func (r *TestRunner) saveVariables(save map[string]string, respData map[string]any) {
	for varName, path := range save {
		value := r.getValueByPath(path, respData)

		// 🔧 修改点4: safejson 已经自动将大整数转换为 int64/uint64
		// 不需要手动转换，只保留调试日志

		r.variables[varName] = value
		fmt.Printf("    💾 Saved variable: %s = %v (type: %T)\n", varName, value, value)
	}
}

// isDependencyPassed 检查依赖是否通过
func (r *TestRunner) isDependencyPassed(name string) bool {
	for _, result := range r.results {
		if result.Name == name {
			return result.Passed
		}
	}
	return false
}

// executeSetup 执行 setup
func (r *TestRunner) executeSetup(ctx context.Context) error {
	fmt.Println("🔧 Executing setup...")
	for _, action := range r.suite.Suite.Setup {
		if err := r.executeAction(ctx, action); err != nil {
			return err
		}
	}
	fmt.Println("✓ Setup completed")
	return nil
}

// executeTeardown 执行 teardown
func (r *TestRunner) executeTeardown(ctx context.Context) error {
	fmt.Println("\n🔧 Executing teardown...")
	for _, action := range r.suite.Suite.Teardown {
		if err := r.executeAction(ctx, action); err != nil {
			return err
		}
	}
	fmt.Println("✓ Teardown completed")
	return nil
}

func (r *TestRunner) SetBaseURL(url string) {
	r.suite.Suite.BaseURL = url
}

// executeAction 执行清理动作
func (r *TestRunner) executeAction(ctx context.Context, action SetupAction) error {
	switch action.Type {
	case "soft_delete_cleanup":
		return r.softDeleteCleanup(ctx, action.Table, action.Condition)
	case "cleanup", "sql":
		if r.cleanup != nil {
			return r.cleanup.Execute(ctx, action)
		}
		return nil
	case "api_call":
		if action.Request != nil {
			return r.executeAPICall(ctx, *action.Request)
		}
		return fmt.Errorf("api_call action requires request configuration")
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// softDeleteCleanup 执行软删除清理
func (r *TestRunner) softDeleteCleanup(ctx context.Context, table, condition string) error {
	if r.dbAdapter == nil {
		return fmt.Errorf("dbAdapter is required for soft_delete_cleanup")
	}

	if table == "" {
		return fmt.Errorf("table name is required")
	}

	// 构造更新语句，设置 soft_deleted = 1 和 deleted_at
	sql := fmt.Sprintf("UPDATE %s SET soft_deleted = 1, deleted_at = NOW() WHERE soft_deleted = 0", table)
	if condition != "" {
		sql += fmt.Sprintf(" AND %s", condition)
	}

	// 使用反射获取底层引擎执行原生 SQL
	// 这里需要根据你的 XormAdapter 提供原生 SQL 执行方法
	xormEngine := r.dbAdapter.(*db.XormAdapter).GetEngine()

	result, err := xormEngine.Exec(sql)
	if err != nil {
		return fmt.Errorf("soft delete cleanup failed: %w", err)
	}

	// 获取影响行数
	if sqlResult, ok := result.(interface{ RowsAffected() (int64, error) }); ok {
		rows, _ := sqlResult.RowsAffected()
		fmt.Printf("  ✓ Soft deleted %d rows from table '%s'\n", rows, table)
	} else {
		fmt.Printf("  ✓ Soft delete cleanup executed on table '%s'\n", table)
	}

	return nil
}

// executeAPICall 执行 API 调用（用于 setup/teardown）
func (r *TestRunner) executeAPICall(ctx context.Context, reqCfg RequestConfig) error {
	req, err := r.buildRequest(reqCfg)
	if err != nil {
		return fmt.Errorf("build request failed: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API call failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// printSummary 打印测试摘要
func (r *TestRunner) printSummary() {
	passed := 0
	failed := 0
	totalDuration := time.Duration(0)

	for _, result := range r.results {
		totalDuration += result.Duration
		if result.Passed {
			passed++
		} else {
			failed++
		}
	}

	fmt.Printf("═══════════════════════════════════════════════════════\n")
	fmt.Printf("📊 Test Summary\n")
	fmt.Printf("═══════════════════════════════════════════════════════\n")
	fmt.Printf("Total Tests:     %d\n", len(r.results))
	fmt.Printf("✓ Passed:        %d\n", passed)
	fmt.Printf("✗ Failed:        %d\n", failed)
	fmt.Printf("⏱  Duration:      %.2fs\n", totalDuration.Seconds())
	fmt.Printf("═══════════════════════════════════════════════════════\n")

	if failed > 0 {
		fmt.Printf("\n❌ Failed Tests:\n")
		for _, result := range r.results {
			if !result.Passed {
				fmt.Printf("  [%s] %s\n", result.Scenario, result.Name)
				fmt.Printf("    Error: %s\n", result.Error)
			}
		}
	} else {
		fmt.Printf("\n🎉 All tests passed!\n")
	}
}

// GetResults 获取测试结果
func (r *TestRunner) GetResults() []TestResult {
	return r.results
}

// ExportResults 导出测试结果为 JSON
func (r *TestRunner) ExportResults(filepath string) error {
	data, err := json.MarshalIndent(r.results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, data, 0644)
}

// SetResults 设置测试结果（用于批量导出）
func (r *TestRunner) SetResults(results []TestResult) {
	r.results = results
}
