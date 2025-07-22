# RejectCountries 插件测试指南

## 1. 单元测试

### 前置条件
确保已安装 Go (版本 1.19+)：
```bash
go version
```

### 运行单元测试
```bash
# 在 plugindemo 目录下运行
go test -v

# 运行特定测试
go test -v -run TestCreateConfig
go test -v -run TestNew_ValidConfig
go test -v -run TestRejectCountries_GetClientIP
```

### 测试覆盖率
```bash
go test -cover
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## 2. 本地 Traefik 集成测试

### 创建测试配置

创建 `traefik.yml` (静态配置):
```yaml
api:
  dashboard: true
  insecure: true

entryPoints:
  web:
    address: ":8080"

experimental:
  localPlugins:
    rejectcountries:
      moduleName: github.com/traefik/plugindemo

providers:
  file:
    filename: dynamic-config.yml
    watch: true
```

创建 `dynamic-config.yml` (动态配置):
```yaml
http:
  routers:
    api:
      rule: Host(`localhost`)
      service: api@internal
      middlewares:
        - reject-gb

    test-service:
      rule: Host(`localhost`) && Path(`/test`)
      service: test-service
      middlewares:
        - reject-gb

  services:
    test-service:
      loadBalancer:
        servers:
          - url: "http://httpbin.org"

  middlewares:
    reject-gb:
      plugin:
        rejectcountries:
          dbPath: "/path/to/GeoLite2-Country.mmdb"  # 需要实际的数据库文件
          preferXForwardedForHeader: true
          matchCountries:
            - "GB"
            - "CN"
          staticResponse:
            statusCode: 403
            headers:
              Content-Type: "text/plain; charset=utf-8"
            body: |
              Sorry, access from your country is not allowed.
```

### 目录结构
确保插件位于正确的目录结构中：
```
./plugins-local/
    └── src/
        └── github.com/
            └── traefik/
                └── plugindemo/
                    ├── reject_countries.go
                    ├── reject_countries_test.go
                    ├── demo.go
                    ├── demo_test.go
                    ├── go.mod
                    └── vendor/
```

### 启动 Traefik
```bash
# 确保在包含 traefik.yml 的目录中
traefik --configfile=traefik.yml
```

## 3. 模拟测试（无需 GeoIP 数据库）

### 创建测试用的模拟插件
创建 `test_reject_countries.go`:
```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    
    // 导入你的插件
    plugin "github.com/traefik/plugindemo"
)

func main() {
    // 创建测试配置
    config := &plugin.RejectCountriesConfig{
        DBPath:                    "/non/existent/path.mmdb", // 故意使用不存在的路径
        PreferXForwardedForHeader: true,
        MatchCountries:            []string{"GB", "US"},
        StaticResponse: plugin.StaticResponseConfig{
            StatusCode: 403,
            Headers: http.Header{
                "Content-Type": []string{"text/plain; charset=utf-8"},
            },
            Body: "Access denied from your country",
        },
    }
    
    // 创建下一个处理器
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Access granted"))
    })
    
    // 创建插件实例
    plugin, err := plugin.New(context.Background(), next, config, "test")
    if err != nil {
        fmt.Printf("Failed to create plugin: %v\n", err)
        return
    }
    
    // 测试场景1: 没有 GeoIP 数据库 - 应该允许通过
    testRequest(plugin, "127.0.0.1", "Test without GeoIP database")
    
    fmt.Println("Testing completed!")
}

func testRequest(handler http.Handler, ip string, description string) {
    fmt.Printf("\n=== %s ===\n", description)
    
    req := httptest.NewRequest("GET", "/test", nil)
    req.RemoteAddr = ip + ":12345"
    
    recorder := httptest.NewRecorder()
    handler.ServeHTTP(recorder, req)
    
    fmt.Printf("Status: %d\n", recorder.Code)
    fmt.Printf("Body: %s\n", strings.TrimSpace(recorder.Body.String()))
    fmt.Printf("Headers: %v\n", recorder.Header())
}
```

### 运行模拟测试
```bash
go run test_reject_countries.go
```

## 4. 使用真实 GeoIP 数据库测试

### 获取 GeoLite2 数据库
1. 注册 MaxMind 账户: https://www.maxmind.com/en/geolite2/signup
2. 下载 GeoLite2 Country 数据库
3. 解压并放置到配置的路径

### 测试不同 IP 地址
创建 `real_geoip_test.go`:
```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "net/http/httptest"
    
    plugin "github.com/traefik/plugindemo"
)

func main() {
    config := &plugin.RejectCountriesConfig{
        DBPath:                    "/path/to/GeoLite2-Country.mmdb", // 真实数据库路径
        PreferXForwardedForHeader: true,
        MatchCountries:            []string{"GB", "CN"},
        StaticResponse: plugin.StaticResponseConfig{
            StatusCode: 403,
            Headers: http.Header{
                "Content-Type": []string{"text/plain; charset=utf-8"},
            },
            Body: "Access denied from your country",
        },
    }
    
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Access granted"))
    })
    
    plugin, err := plugin.New(context.Background(), next, config, "test")
    if err != nil {
        fmt.Printf("Failed to create plugin: %v\n", err)
        return
    }
    
    // 测试不同国家的 IP
    testIPs := map[string]string{
        "8.8.8.8":        "Google DNS (US)",
        "1.1.1.1":        "Cloudflare DNS (US)", 
        "208.67.222.222": "OpenDNS (US)",
        "127.0.0.1":      "Localhost",
        // 添加更多测试 IP
    }
    
    for ip, description := range testIPs {
        testGeoIP(plugin, ip, description)
    }
}

func testGeoIP(handler http.Handler, ip string, description string) {
    fmt.Printf("\n=== Testing %s (%s) ===\n", ip, description)
    
    req := httptest.NewRequest("GET", "/test", nil)
    req.RemoteAddr = ip + ":12345"
    
    recorder := httptest.NewRecorder()
    handler.ServeHTTP(recorder, req)
    
    fmt.Printf("Status: %d\n", recorder.Code)
    fmt.Printf("Body: %s\n", recorder.Body.String())
}
```

## 5. 性能测试

### 基准测试
创建 `reject_countries_bench_test.go`:
```go
package plugindemo

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
)

func BenchmarkRejectCountries_ServeHTTP(b *testing.B) {
    config := &RejectCountriesConfig{
        DBPath:                    "/non/existent/path.mmdb",
        PreferXForwardedForHeader: true,
        MatchCountries:            []string{"GB", "US", "CN"},
        StaticResponse: StaticResponseConfig{
            StatusCode: 403,
            Body:       "Access denied",
        },
    }
    
    next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })
    
    plugin, err := New(context.Background(), next, config, "bench")
    if err != nil {
        b.Fatal(err)
    }
    
    req := httptest.NewRequest("GET", "/test", nil)
    req.RemoteAddr = "127.0.0.1:12345"
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        recorder := httptest.NewRecorder()
        plugin.ServeHTTP(recorder, req)
    }
}
```

运行基准测试：
```bash
go test -bench=BenchmarkRejectCountries_ServeHTTP -benchmem
```

## 6. Docker 集成测试

创建 `docker-compose.test.yml`:
```yaml
version: '3.8'

services:
  traefik:
    image: traefik:v3.0
    command:
      - --api.dashboard=true
      - --api.insecure=true
      - --providers.file.filename=/etc/traefik/dynamic-config.yml
      - --entrypoints.web.address=:80
      - --experimental.localPlugins.rejectcountries.moduleName=github.com/traefik/plugindemo
    ports:
      - "80:80"
      - "8080:8080"
    volumes:
      - ./traefik.yml:/etc/traefik/traefik.yml
      - ./dynamic-config.yml:/etc/traefik/dynamic-config.yml
      - ./plugins-local:/plugins-local
      - /path/to/GeoLite2-Country.mmdb:/mmdb/GeoLite2-Country.mmdb
    
  test-app:
    image: nginx:alpine
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.test.rule=Host(`localhost`)"
      - "traefik.http.routers.test.middlewares=reject-countries"
      - "traefik.http.middlewares.reject-countries.plugin.rejectcountries.matchCountries=GB,CN"
```

启动测试环境：
```bash
docker-compose -f docker-compose.test.yml up
```

## 7. 日志调试

启用详细日志来调试插件行为：
```yaml
# 在 traefik.yml 中添加
log:
  level: DEBUG
  filePath: "/var/log/traefik.log"
```

查看日志：
```bash
tail -f /var/log/traefik.log | grep RejectCountries
```

## 测试检查清单

- [ ] 单元测试全部通过
- [ ] 配置验证测试（空国家列表、无效状态码等）
- [ ] IP 提取测试（X-Forwarded-For 和 RemoteAddr）
- [ ] GeoIP 查询测试（有效和无效 IP）
- [ ] 静态响应测试（状态码、头部、响应体）
- [ ] 错误处理测试（GeoIP 数据库不可用）
- [ ] 性能基准测试
- [ ] 实际 Traefik 集成测试

通过这些测试方法，您可以全面验证 RejectCountries 插件的功能和性能。 