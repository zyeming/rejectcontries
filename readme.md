# RejectCountries Plugin

这个仓库包含了一个 RejectCountries 插件的实现，用于根据客户端 IP 的地理位置拒绝来自特定国家的请求。

## RejectCountries 插件功能

该插件使用 MaxMind GeoIP2 数据库来确定客户端的国家/地区，并为匹配的国家返回静态响应。

### 配置参数

- `dbPath`: GeoLite2 Country 数据库文件路径 (默认: "/mmdb/GeoLite2-Country.mmdb")
- `preferXForwardedForHeader`: 是否优先使用 X-Forwarded-For 头来获取客户端 IP (默认: true)
- `matchCountries`: 需要拒绝的国家代码列表 (ISO 3166-1 alpha-2 格式，如 "GB", "US")
- `staticResponse`: 当匹配到拒绝国家时返回的静态响应配置
  - `statusCode`: HTTP 状态码 (默认: 200)
  - `headers`: 响应头
  - `body`: 响应内容

### 配置示例

```yaml
# Dynamic configuration
http:
  middlewares:
    reject-countries:
      plugin:
        example:
          dbPath: "/mmdb/GeoLite2-Country.mmdb"
          preferXForwardedForHeader: true
          matchCountries:
            - "GB"
            - "US"
          staticResponse:
            statusCode: 403
            headers:
              Content-Type: "text/plain; charset=utf-8"
            body: |
              Access denied from your country
```

---

# Original Plugin Demo

This repository includes an example plugin, `demo`, for you to use as a reference for developing your own plugins.

[![Build Status](https://github.com/traefik/plugindemo/workflows/Main/badge.svg?branch=master)](https://github.com/traefik/plugindemo/actions)

The existing plugins can be browsed into the [Plugin Catalog](https://plugins.traefik.io).

# Developing a Traefik plugin

[Traefik](https://traefik.io) plugins are developed using the [Go language](https://golang.org).

A [Traefik](https://traefik.io) middleware plugin is just a [Go package](https://golang.org/ref/spec#Packages) that provides an `http.Handler` to perform specific processing of requests and responses.

Rather than being pre-compiled and linked, however, plugins are executed on the fly by [Yaegi](https://github.com/traefik/yaegi), an embedded Go interpreter.

## Usage

For a plugin to be active for a given Traefik instance, it must be declared in the static configuration.

Plugins are parsed and loaded exclusively during startup, which allows Traefik to check the integrity of the code and catch errors early on.
If an error occurs during loading, the plugin is disabled.

For security reasons, it is not possible to start a new plugin or modify an existing one while Traefik is running.

Once loaded, middleware plugins behave exactly like statically compiled middlewares.
Their instantiation and behavior are driven by the dynamic configuration.

Plugin dependencies must be [vendored](https://golang.org/ref/mod#vendoring) for each plugin.
Vendored packages should be included in the plugin's GitHub repository. ([Go modules](https://blog.golang.org/using-go-modules) are not supported.)

### Configuration

For each plugin, the Traefik static configuration must define the module name (as is usual for Go packages).

The following declaration (given here in YAML) defines a plugin:

```yaml
# Static configuration

experimental:
  plugins:
    example:
      moduleName: github.com/traefik/plugindemo
      version: v0.2.1
```

Here is an example of a file provider dynamic configuration (given here in YAML), where the interesting part is the `http.middlewares` section:

```yaml
# Dynamic configuration

http:
  routers:
    my-router:
      rule: host(`demo.localhost`)
      service: service-foo
      entryPoints:
        - web
      middlewares:
        - my-plugin

  services:
   service-foo:
      loadBalancer:
        servers:
          - url: http://127.0.0.1:5000
  
  middlewares:
    my-plugin:
      plugin:
        example:
          headers:
            Foo: Bar
```

### Local Mode

Traefik also offers a developer mode that can be used for temporary testing of plugins not hosted on GitHub.
To use a plugin in local mode, the Traefik static configuration must define the module name (as is usual for Go packages) and a path to a [Go workspace](https://golang.org/doc/gopath_code.html#Workspaces), which can be the local GOPATH or any directory.

The plugins must be placed in `./plugins-local` directory,
which should be in the working directory of the process running the Traefik binary.
The source code of the plugin should be organized as follows:

```
./plugins-local/
    └── src
        └── github.com
            └── traefik
                └── plugindemo
                    ├── demo.go
                    ├── demo_test.go
                    ├── go.mod
                    ├── LICENSE
                    ├── Makefile
                    └── readme.md
```

```yaml
# Static configuration

experimental:
  localPlugins:
    example:
      moduleName: github.com/traefik/plugindemo
```

(In the above example, the `plugindemo` plugin will be loaded from the path `./plugins-local/src/github.com/traefik/plugindemo`.)

```yaml
# Dynamic configuration

http:
  routers:
    my-router:
      rule: host(`demo.localhost`)
      service: service-foo
      entryPoints:
        - web
      middlewares:
        - my-plugin

  services:
   service-foo:
      loadBalancer:
        servers:
          - url: http://127.0.0.1:5000
  
  middlewares:
    my-plugin:
      plugin:
        example:
          headers:
            Foo: Bar
```

## Defining a Plugin

A plugin package must define the following exported Go objects:

- A type `type Config struct { ... }`. The struct fields are arbitrary.
- A function `func CreateConfig() *Config`.
- A function `func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error)`.

```go
// Package example a example plugin.
package example

import (
	"context"
	"net/http"
)

// Config the plugin configuration.
type Config struct {
	// ...
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		// ...
	}
}

// Example a plugin.
type Example struct {
	next     http.Handler
	name     string
	// ...
}

// New created a new plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	// ...
	return &Example{
		// ...
	}, nil
}

func (e *Example) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// ...
	e.next.ServeHTTP(rw, req)
}
```

## Logs

Currently, the only way to send logs to Traefik is to use `os.Stdout.WriteString("...")` or `os.Stderr.WriteString("...")`.

In the future, we will try to provide something better and based on levels.

## Plugins Catalog

Traefik plugins are stored and hosted as public GitHub repositories.

Every 30 minutes, the Plugins Catalog online service polls Github to find plugins and add them to its catalog.

### Prerequisites

To be recognized by Plugins Catalog, your repository must meet the following criteria:

- The `traefik-plugin` topic must be set.
- The `.traefik.yml` manifest must exist, and be filled with valid contents.

If your repository fails to meet either of these prerequisites, Plugins Catalog will not see it.

### Manifest

A manifest is also mandatory, and it should be named `.traefik.yml` and stored at the root of your project.

This YAML file provides Plugins Catalog with information about your plugin, such as a description, a full name, and so on.

Here is an example of a typical `.traefik.yml`file:

```yaml
# The name of your plugin as displayed in the Plugins Catalog web UI.
displayName: Name of your plugin

# For now, `middleware` is the only type available.
type: middleware

# The import path of your plugin.
import: github.com/username/my-plugin

# A brief description of what your plugin is doing.
summary: Description of what my plugin is doing

# Medias associated to the plugin (optional)
iconPath: foo/icon.png
bannerPath: foo/banner.png

# Configuration data for your plugin.
# This is mandatory,
# and Plugins Catalog will try to execute the plugin with the data you provide as part of its startup validity tests.
testData:
  Headers:
    Foo: Bar
```

Properties include:

- `displayName` (required): The name of your plugin as displayed in the Plugins Catalog web UI.
- `type` (required): For now, `middleware` is the only type available.
- `import` (required): The import path of your plugin.
- `summary` (required): A brief description of what your plugin is doing.
- `testData` (required): Configuration data for your plugin. This is mandatory, and Plugins Catalog will try to execute the plugin with the data you provide as part of its startup validity tests.
- `iconPath` (optional): A local path in the repository to the icon of the project.
- `bannerPath` (optional): A local path in the repository to the image that will be used when you will share your plugin page in social medias.

There should also be a `go.mod` file at the root of your project. Plugins Catalog will use this file to validate the name of the project.

### Tags and Dependencies

Plugins Catalog gets your sources from a Go module proxy, so your plugins need to be versioned with a git tag.

Last but not least, if your plugin middleware has Go package dependencies, you need to vendor them and add them to your GitHub repository.

If something goes wrong with the integration of your plugin, Plugins Catalog will create an issue inside your Github repository and will stop trying to add your repo until you close the issue.

## Troubleshooting

If Plugins Catalog fails to recognize your plugin, you will need to make one or more changes to your GitHub repository.

In order for your plugin to be successfully imported by Plugins Catalog, consult this checklist:

- The `traefik-plugin` topic must be set on your repository.
- There must be a `.traefik.yml` file at the root of your project describing your plugin, and it must have a valid `testData` property for testing purposes.
- There must be a valid `go.mod` file at the root of your project.
- Your plugin must be versioned with a git tag.
- If you have package dependencies, they must be vendored and added to your GitHub repository.
