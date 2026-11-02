# TransitAI 手动更新（现有 Docker Compose 部署）

这份说明适用于现有容器 `sub2api`、`sub2api-postgres`、`sub2api-redis`，以及本地构建的 TransitAI 镜像。更新包含数据库迁移，先完成备份。

## 1. 确认部署目录

在服务器终端执行，每次只粘贴一个代码块。若终端提示符变成 `>`，先按 `Ctrl+C` 退出未闭合的命令。

```bash
docker compose ls
```

记录 Sub2API 对应的项目名称与配置文件。后面的重建命令必须沿用该项目的原始配置文件、项目名称和 `.env`；如果原来使用了额外的 `--env-file`，也要沿用。

## 2. 备份数据库、应用数据和当前镜像

以下变量需要保留在同一个终端会话中。

```bash
UPDATE_STAMP=$(date +%Y%m%d-%H%M%S)
BACKUP_DIR=/root/transitai-backup-$UPDATE_STAMP
mkdir -m 700 "$BACKUP_DIR"
docker exec sub2api-postgres sh -eu -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$BACKUP_DIR/database.dump"
test -s "$BACKUP_DIR/database.dump" && echo '数据库备份已生成'
docker cp sub2api:/app/data "$BACKUP_DIR/app-data"
APP_IMAGE=$(docker inspect sub2api --format '{{.Config.Image}}')
APP_IMAGE_ID=$(docker inspect sub2api --format '{{.Image}}')
docker tag "$APP_IMAGE_ID" "transitai-sub2api-backup:$UPDATE_STAMP"
```

确认每条命令都成功，数据库备份文件有内容。再把原始 Compose 配置文件和 `.env` 复制到这个权限受限的备份目录。

## 3. 获取更新后的 TransitAI 源码并构建

新建源码目录，方便保留服务器上现有的源码与部署配置。

```bash
SOURCE_DIR=/root/transitai-source-$UPDATE_STAMP
git clone --depth 1 --branch main https://github.com/136465311/sub2api.git "$SOURCE_DIR"
```

克隆成功后执行构建。使用现有服务的镜像标签，Compose 才能找到这次构建的结果。

```bash
cd "$SOURCE_DIR"
docker build --build-arg COMMIT="$(git rev-parse HEAD)" -t "$APP_IMAGE" .
```

构建成功前，现有容器仍然继续运行。

## 4. 重建应用容器

先切换到第 1 步确认的部署目录。下面的 `原项目名` 和 `/实际路径/compose.yml` 必须替换成第 1 步的实际值；多个配置文件需要按原顺序使用多个 `-f`。

```bash
docker compose -p 原项目名 -f /实际路径/compose.yml up -d --no-deps --no-build --pull never sub2api
```

这条命令只重建应用服务，使用刚刚构建的 TransitAI 镜像。数据库和 Redis 沿用现有容器与数据。

## 5. 检查上线结果

```bash
docker ps
docker logs --tail 80 sub2api
docker inspect sub2api --format '{{json .State.Health}}'
```

等待健康检查显示 `healthy`，再打开网站确认登录、AI 聊天和绘图页面。若启动失败，保留日志、备份和旧镜像；若新版本已经执行数据库迁移，恢复旧版本时需要评估数据库备份的恢复，不能只切换镜像。

本地验证不包含服务器上的 Docker 镜像构建和上线健康检查，这两项以服务器执行结果为准。
