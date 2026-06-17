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

格式化：

```sh
gofmt -w backend
```

编译检查：

```sh
cd backend
go build .
```

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
