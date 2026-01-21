# 🎯 多文件测试使用指南

## 📋 功能概览

测试框架现在支持：
- ✅ 运行单个 YAML 文件
- ✅ 运行目录中的所有 YAML 文件
- ✅ 递归搜索子目录
- ✅ 自定义文件匹配模式
- ✅ 汇总所有测试结果
- ✅ 分别导出每个文件的结果

## 🚀 使用方式

### 1. 运行单个文件

```bash
go run cmd/apitest/main.go \
  -config testcases/user_api_test.yaml
```

### 2. 运行目录中的所有 YAML 文件

```bash
go run cmd/apitest/main.go \
  -config testcases/
```

### 3. 递归搜索子目录

```bash
# 默认开启递归搜索
go run cmd/apitest/main.go \
  -config testcases/ \
  -recursive=true
```

### 4. 使用自定义文件模式

```bash
# 只运行以 _test.yaml 结尾的文件
go run cmd/apitest/main.go \
  -config testcases/ \
  -pattern "*_test.yaml"

# 只运行以 api_ 开头的文件
go run cmd/apitest/main.go \
  -config testcases/ \
  -pattern "api_*.yaml"

# 运行特定模块的测试
go run cmd/apitest/main.go \
  -config testcases/ \
  -pattern "user_*.yaml"
```

### 5. 完整示例（带所有选项）

```bash
go run cmd/apitest/main.go \
  -config testcases/ \
  -pattern "*_api_test.yaml" \
  -recursive=true \
  -db "user:pass@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True" \
  -url http://test-server:8080 \
  -export test_results/
```

## 📂 推荐目录结构

```
project/
├── testcases/
│   ├── user/
│   │   ├── user_api_test.yaml          # 用户 API 测试
│   │   ├── user_auth_test.yaml         # 用户认证测试
│   │   └── user_permission_test.yaml   # 用户权限测试
│   ├── product/
│   │   ├── product_crud_test.yaml      # 产品 CRUD 测试
│   │   └── product_search_test.yaml    # 产品搜索测试
│   ├── order/
│   │   ├── order_create_test.yaml      # 订单创建测试
│   │   └── order_payment_test.yaml     # 订单支付测试
│   └── integration/
│       └── full_flow_test.yaml         # 完整业务流程测试
└── test_results/
    ├── user_api_test_results.json
    ├── user_auth_test_results.json
    └── ...
```

## 🎯 命令行参数详解

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `-config` | string | **必填** | 测试配置文件或目录路径 |
| `-pattern` | string | `*.yaml` | 文件匹配模式 |
| `-recursive` | bool | `true` | 是否递归搜索子目录 |
| `-db` | string | `""` | 数据库 DSN（用于数据清理） |
| `-url` | string | `http://localhost:8080` | API 基础 URL |
| `-export` | string | `""` | 导出结果的目录路径 |

## 📊 输出示例

### 运行多个文件时的输出

```
════════════════════════════════════════════════════════
🧪 API Integration Test Runner
════════════════════════════════════════════════════════
📁 Config Path: testcases/
🔍 Pattern: *.yaml
🗄️  Database: Connected (cleanup enabled)
📋 Found 5 test file(s):
   1. testcases/user/user_api_test.yaml
   2. testcases/user/user_auth_test.yaml
   3. testcases/product/product_crud_test.yaml
   4. testcases/order/order_create_test.yaml
   5. testcases/integration/full_flow_test.yaml
════════════════════════════════════════════════════════


╔═══════════════════════════════════════════════════════╗
║ 📄 Running: user_api_test.yaml                        ║
╚═══════════════════════════════════════════════════════╝

🚀 Running test suite: User API Tests
📍 Base URL: http://localhost:8080

🔧 Executing setup...
  ✓ Soft deleted 0 rows from table 't_user'
✓ Setup completed

📦 Scenario: User Lifecycle
   测试用户的完整生命周期，遵循幂等性设计
   ✓ Lifecycle - Create User (0.12s)
   ✓ Lifecycle - Get User After Create (0.05s)
   ✓ Lifecycle - Update User (0.08s)
   ✓ Lifecycle - Delete User (0.06s)

📦 Scenario: Parameter Validation
   测试请求参数验证
   ✓ Create User - Missing Required Field (0.04s)
   ✓ Create User - Unknown Field (0.04s)

═══════════════════════════════════════════════════════
📊 Test Summary
═══════════════════════════════════════════════════════
Total Tests:     6
✓ Passed:        6
✗ Failed:        0
⏱  Duration:      0.39s
═══════════════════════════════════════════════════════

🎉 All tests passed!


╔═══════════════════════════════════════════════════════╗
║ 📄 Running: user_auth_test.yaml                       ║
╚═══════════════════════════════════════════════════════╝

... (类似输出)


╔═══════════════════════════════════════════════════════╗
║                  📊 OVERALL SUMMARY                    ║
╚═══════════════════════════════════════════════════════╝

📁 Test Files: 5
✅ Total Passed: 45
❌ Total Failed: 2
📈 Success Rate: 95.74%

❌ Failed Tests by File:

  📄 product_crud_test.yaml (2 failed)
     ✗ [Product Management] Create Product with Invalid Price
       assertion failed: data.error should contain "invalid price"
     ✗ [Product Management] Delete Non-existent Product
       status code mismatch: expected 404, got 500

════════════════════════════════════════════════════════

📄 Test results exported to: test_results/
  ✓ Exported: test_results/user_api_test_results.json
  ✓ Exported: test_results/user_auth_test_results.json
  ✓ Exported: test_results/product_crud_test_results.json
  ✓ Exported: test_results/order_create_test_results.json
  ✓ Exported: test_results/full_flow_test_results.json
```

## 🎨 实用场景

### 场景 1: 开发时只测试当前模块

```bash
# 只测试用户相关 API
go run cmd/apitest/main.go \
  -config testcases/user/ \
  -recursive=false
```

### 场景 2: CI/CD 完整回归测试

```bash
# 运行所有测试，导出结果
go run cmd/apitest/main.go \
  -config testcases/ \
  -db "${DB_DSN}" \
  -url "${API_URL}" \
  -export ci_results/ \
  -pattern "*_test.yaml"
```

### 场景 3: 按优先级运行测试

```yaml
# 目录结构
testcases/
├── p0_critical/      # P0 核心功能
├── p1_important/     # P1 重要功能
└── p2_normal/        # P2 普通功能
```

```bash
# 先运行 P0 测试
go run cmd/apitest/main.go -config testcases/p0_critical/

# 如果 P0 通过，再运行 P1
go run cmd/apitest/main.go -config testcases/p1_important/
```

### 场景 4: 按模块分别测试

```bash
# 测试用户模块
go run cmd/apitest/main.go \
  -config testcases/ \
  -pattern "user_*.yaml" \
  -export results/user/

# 测试产品模块
go run cmd/apitest/main.go \
  -config testcases/ \
  -pattern "product_*.yaml" \
  -export results/product/

# 测试订单模块
go run cmd/apitest/main.go \
  -config testcases/ \
  -pattern "order_*.yaml" \
  -export results/order/
```

## 🔧 最佳实践

### 1. 文件命名规范

```
✅ 推荐:
- user_api_test.yaml
- product_crud_test.yaml
- order_payment_test.yaml

❌ 避免:
- test.yaml (太通用)
- t1.yaml (无意义)
- api.yaml (不明确)
```

### 2. 目录组织

```
✅ 按模块组织:
testcases/
├── user/
├── product/
└── order/

✅ 按优先级组织:
testcases/
├── p0_critical/
├── p1_important/
└── p2_normal/

✅ 混合组织:
testcases/
├── smoke/          # 冒烟测试
├── regression/     # 回归测试
└── modules/
    ├── user/
    └── product/
```

### 3. 数据隔离

每个测试文件应该：
- 使用唯一的测试数据前缀
- 在 setup 中清理旧数据
- 在 teardown 中清理新数据

```yaml
# user_api_test.yaml
suite:
  setup:
    - type: soft_delete_cleanup
      table: t_user
      condition: "username LIKE 'test-user-%'"

# product_api_test.yaml
suite:
  setup:
    - type: soft_delete_cleanup
      table: t_product
      condition: "name LIKE 'test-product-%'"
```

## 📈 CI/CD 集成

### GitHub Actions

```yaml
name: API Tests

on: [push, pull_request]

jobs:
  api-test:
    runs-on: ubuntu-latest
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Start Services
        run: docker-compose up -d
      
      - name: Wait for Services
        run: sleep 10
      
      - name: Run P0 Tests
        run: |
          go run cmd/apitest/main.go \
            -config testcases/p0_critical/ \
            -db "${{ secrets.TEST_DB_DSN }}" \
            -export test_results/p0/
      
      - name: Run All Tests
        if: success()
        run: |
          go run cmd/apitest/main.go \
            -config testcases/ \
            -pattern "*_test.yaml" \
            -db "${{ secrets.TEST_DB_DSN }}" \
            -export test_results/all/
      
      - name: Upload Results
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: test-results
          path: test_results/
```

### Jenkins Pipeline

```groovy
pipeline {
    agent any
    
    stages {
        stage('Setup') {
            steps {
                sh 'docker-compose up -d'
                sh 'sleep 10'
            }
        }
        
        stage('Run Tests') {
            steps {
                sh '''
                    go run cmd/apitest/main.go \
                        -config testcases/ \
                        -pattern "*_test.yaml" \
                        -db "${TEST_DB_DSN}" \
                        -export test_results/
                '''
            }
        }
    }
    
    post {
        always {
            archiveArtifacts artifacts: 'test_results/**/*.json'
            sh 'docker-compose down'
        }
    }
}
```

## 🐛 故障排除

### 问题 1: 找不到测试文件

```bash
❌ No test configuration files found in: testcases/
```

**解决方案**:
1. 检查路径是否正确
2. 检查文件扩展名是否匹配 pattern
3. 使用 `-pattern "*.yml"` 如果文件是 `.yml` 扩展名

### 问题 2: 某个文件解析失败

```bash
❌ Failed to load test configuration: failed to parse config
```

**解决方案**:
- 检查 YAML 语法
- 使用 `yamllint` 验证文件
- 单独运行该文件定位问题

### 问题 3: 数据库清理冲突

多个测试文件同时清理相同的表可能导致冲突。

**解决方案**:
- 使用不同的数据前缀
- 在 setup 中添加更精确的条件
- 考虑使用事务隔离（如果支持）

## ✨ 总结

现在你可以：
- ✅ 运行单个测试文件
- ✅ 批量运行多个测试文件
- ✅ 按模块/优先级组织测试
- ✅ 自定义文件匹配模式
- ✅ 获取汇总测试报告
- ✅ 轻松集成到 CI/CD 流程

测试更灵活，维护更简单！🎉