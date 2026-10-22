# 构建与开发

本项目包含 Go/Gin 后端、Vue 3 前端、Docker 构建资源和 VitePress 文档。

## 文档站

首次安装依赖：

```sh
npm --prefix docs install
```

本地预览：

```sh
npm --prefix docs run docs:dev
```

构建静态文档：

```sh
npm --prefix docs run docs:build
```

预览构建产物：

```sh
npm --prefix docs run docs:preview
```

构建产物位于 `docs/.vitepress/dist`。

## 后端

后端使用 Go 1.26.10。`backend/go.mod` 和 `go.work` 声明相同的最低版本；Docker 构建镜像固定为 `golang:1.26.10-alpine`，Windows 发布工作流从 `backend/go.mod` 读取版本。升级补丁版本时需同步这些配置。

构建前可用 `go version` 确认实际工具链版本。

格式化：

```sh
gofmt -w backend
```

编译检查：

```sh
cd backend
go build .
```

运行 `go -C backend test ./...` 可执行后端测试。RAR/ZIP 核心样本由测试在临时目录中生成；额外的压缩、加密、分卷和 7z 样本保存在被 Git 忽略的 `backend/controller/testdata/archive-fixtures.zip`，也可通过 `L4D2_ARCHIVE_FIXTURE_ZIP` 指定本地样本包。缺少样本包时这些扩展用例会明确跳过，显式指定的路径无效则测试失败。

本地运行后端时建议以 `backend/` 为工作目录，确保能读取到 `ip2region_*.xdb`、`preset.yaml` 和 `static/`。

## 前端

```sh
cd frontend
npm install
npm run build
```

Windows 发布包需要先构建前端，把静态资源放入后端 `static/`，再交叉编译 Go 后端。

## Docker 镜像

管理器镜像：

```sh
docker build -f manifest/docker/manager.Dockerfile -t l4d2-manager-next:local .
```

游戏服镜像：

```sh
docker build -f manifest/docker/l4d2.Dockerfile -t l4d2-pure:local .
```

## 发布工作流

`.github/workflows/` 中包含：

- `manager-image.yml`：构建并发布管理器镜像。
- `windows-release.yml`：构建 Windows 发布包。
- `l4d2-image.yml`：构建游戏服镜像，当前为手动触发。

推送 `v*` 标签会触发管理器镜像和 Windows 发布流程。
