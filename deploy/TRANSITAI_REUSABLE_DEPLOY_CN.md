# TransitAI 通用手动部署与更新步骤

本文依据历史对话中确认的服务器配置整理，适用于现有 Docker Compose 源码构建部署。本次只编写说明，没有执行服务器部署；首次使用或服务器配置变更后，先核对第 1 步。

参考对话：[帮我更新一下最新代码并使用@Chrome插件部署一下 我已经在浏览器登录上服务器了](codex://threads/01a19ca3-186d-7db3-8edf-7f56c5f897ae)。

## 0. 固定配置与执行规则

| 项目 | 历史对话确认的值 |
| --- | --- |
| 服务器部署目录 | /home/ubuntu/transitai-sub2api |
| 生产 Compose 文件 | docker-compose.prod.yml |
| Compose 项目名 | transitai-sub2api |
| 应用服务名 / 容器名 | sub2api / sub2api |
| PostgreSQL 容器名 | sub2api-postgres |
| Redis 容器名 | sub2api-redis |
| 应用镜像标签 | transitai-sub2api-sub2api（以实际查询为准） |
| 源码仓库 | https://github.com/136465311/sub2api.git |
| 服务器 Git 远程 / 分支 | origin / main |
| 环境文件 | 部署目录内的 .env |

- 在服务器的 Linux Bash 终端执行，Chrome 中的服务器控制台或 SSH 均可；不要在本机 Windows PowerShell 执行。
- 以下默认以 root 操作。普通用户先执行 sudo -i，再开始下面的步骤。
- 每个代码框单独粘贴，控制台粘贴不稳定时逐行执行。任何命令报错都先停下处理，再继续后面的步骤。
- 保持同一个终端会话以沿用变量。断线后按第 8 节恢复，不要继续使用空变量。
- 如果提示符变成单独的 >，先按 Ctrl+C，检查引号后重新输入整条命令。
- 开发端必须先完成更新、验证并推送到自己的仓库。服务器只能拉取并部署已经提交的代码。
- 不写死日期、版本和提交号，每次从 Git、容器和镜像中读取。
- 如果原部署使用多个 Compose 文件或额外 --env-file，所有检查、构建、上线与回退命令都必须沿用同一组参数，并备份对应文件。

## 1. 进入原部署目录并核对配置

~~~bash
cd /home/ubuntu/transitai-sub2api
pwd
git branch --show-current
git remote get-url origin
git status --short
~~~

确认目录正确，分支为 main，仓库所有者为 136465311。HTTPS 和 SSH 地址都可以。Git 状态中的 ?? 是未跟踪文件，服务器专用的生产配置可能就在其中，应保留。如果出现已跟踪文件修改、UU 冲突或正在合并，先处理；不要直接执行 git reset --hard 或 git clean。

检查运行中的 Compose 配置：

~~~bash
docker compose ls
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}'
docker inspect sub2api --format '{{index .Config.Labels "com.docker.compose.project"}}'
docker inspect sub2api --format '{{index .Config.Labels "com.docker.compose.project.config_files"}}'
docker inspect sub2api --format '{{index .Config.Labels "com.docker.compose.project.working_dir"}}'
docker inspect sub2api --format '{{.Config.Image}}'
~~~

应与上表中的项目、目录、生产文件对应。如果不一致，先按实际部署修正后续命令。

检查文件、配置和空间：

~~~bash
test -f docker-compose.prod.yml
test -f .env
docker compose -p transitai-sub2api -f docker-compose.prod.yml config --quiet
df -h . /root
docker system df
~~~

test 和 config --quiet 成功通常没有输出；报错应先处理。备份、依赖下载和镜像构建均需要剩余空间。不要直接删除数据卷或回退镜像来腾空间。

## 2. 备份配置、数据库、应用数据和旧镜像

创建本次独立的备份目录：

~~~bash
umask 077
UPDATE_STAMP=$(date +%Y%m%d-%H%M%S)
BACKUP_DIR=/root/transitai-backup-$UPDATE_STAMP
mkdir -m 700 "$BACKUP_DIR"
printf '%s\n' "$BACKUP_DIR"
~~~

记住输出的完整路径；时间戳采用服务器时间。

备份生产配置并记录更新前的版本：

~~~bash
cp -a .env docker-compose.prod.yml "$BACKUP_DIR/"
git rev-parse HEAD > "$BACKUP_DIR/source-before.txt"
docker exec sub2api /app/sub2api -version > "$BACKUP_DIR/runtime-before.txt" 2>&1
docker inspect sub2api --format '{{.Config.Image}}' > "$BACKUP_DIR/image-tag-before.txt"
docker inspect sub2api --format '{{.Image}}' > "$BACKUP_DIR/image-id-before.txt"
~~~

源码与运行版本分别记录，因为两者可能不同。额外挂载的配置、证书和额外环境文件也要备份。下面只查询挂载路径，不必输出 .env 内容：

~~~bash
docker inspect sub2api --format '{{json .Mounts}}'
~~~

备份数据库：

~~~bash
docker exec sub2api-postgres sh -ec 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$BACKUP_DIR/database.dump"
~~~

确认上一条成功退出，再检查备份非空且归档目录可读取：

~~~bash
test -s "$BACKUP_DIR/database.dump" && echo '数据库备份非空'
docker exec -i sub2api-postgres pg_restore --list < "$BACKUP_DIR/database.dump" > "$BACKUP_DIR/database.list"
ls -lh "$BACKUP_DIR/database.dump"
~~~

pg_restore --list 不会恢复或改写数据库，也不能替代完整恢复演练。备份失败时不要继续上线。

备份应用数据，保留旧镜像：

~~~bash
docker cp sub2api:/app/data "$BACKUP_DIR/app-data"
APP_IMAGE=$(cat "$BACKUP_DIR/image-tag-before.txt")
OLD_IMAGE_ID=$(cat "$BACKUP_DIR/image-id-before.txt")
docker tag "$OLD_IMAGE_ID" "transitai-sub2api-backup:$UPDATE_STAMP"
printf '%s\n' "transitai-sub2api-backup:$UPDATE_STAMP" > "$BACKUP_DIR/rollback-image.txt"
docker image inspect "transitai-sub2api-backup:$UPDATE_STAMP" --format '{{.Id}}'
ls -lh "$BACKUP_DIR"
~~~

这是在线备份。数据库归档与应用目录复制不是同一个事务快照，恢复时需要评估一致性。备份包含配置和业务数据，不要公开上传。

## 3. 拉取已推送到你仓库的代码

~~~bash
git pull --ff-only origin main
~~~

成功后记录本次目标提交：

~~~bash
TARGET_COMMIT=$(git rev-parse HEAD)
printf '%s\n' "$TARGET_COMMIT" > "$BACKUP_DIR/source-target.txt"
git log -1 --oneline
cat backend/cmd/server/VERSION
~~~

- Already up to date 表示服务器与远程 main 对齐；若没有预期更新，先确认开发端已完成推送。
- --ff-only 报错时停止，检查服务器本地提交或改动，不要强制覆盖。
- 拉取自己的 TransitAI 仓库；上游更新应先在开发端合并、验证，再推送到自己的仓库。
- 不要照抄历史的 c98e42a90 或旧版本号，TARGET_COMMIT 自动读取本次提交。

## 4. 按生产 Compose 配置构建新镜像

先检查更新后的配置：

~~~bash
docker compose -p transitai-sub2api -f docker-compose.prod.yml config --quiet
~~~

构建应用：

~~~bash
docker compose -p transitai-sub2api -f docker-compose.prod.yml build --build-arg COMMIT="$TARGET_COMMIT" sub2api
~~~

构建时原应用继续运行。等待命令成功退出后才进行第 5 步。首次构建或依赖更新可能耗时较长。

此命令沿用生产文件里的构建目录、Dockerfile 和构建参数。历史对话最后确认使用这套生产 Compose 构建方式，日常统一采用它。

构建后读取新镜像 ID，并核对镜像内程序版本：

~~~bash
NEW_IMAGE_ID=$(docker image inspect "$APP_IMAGE" --format '{{.Id}}')
printf '%s\n' "$NEW_IMAGE_ID" > "$BACKUP_DIR/image-id-target.txt"
docker run --rm --entrypoint /app/sub2api "$NEW_IMAGE_ID" -version
printf '本次目标提交：%s\n' "$TARGET_COMMIT"
~~~

-version 打印版本后退出，不启动业务服务或执行数据库迁移。输出中的 commit 应与 TARGET_COMMIT 对应；版本号可能相同，提交号仍可能不同。

如果 commit 不对，或原镜像标签仍指向旧内容，先核对生产配置的 build.context、build.dockerfile、image，以及 Dockerfile 是否接受 ARG COMMIT。不要在镜像内容未核实的情况下重建线上容器。

## 5. 重建应用容器

只有构建和镜像版本核对成功，才执行：

~~~bash
docker compose -p transitai-sub2api -f docker-compose.prod.yml up -d --no-deps --no-build --pull never --force-recreate sub2api
~~~

| 参数 | 作用 |
| --- | --- |
| -p transitai-sub2api | 沿用原项目名 |
| -f docker-compose.prod.yml | 沿用原生产配置和挂载 |
| --no-deps | 不连带重建 PostgreSQL、Redis 等依赖 |
| --no-build | 使用已经检查过的构建结果 |
| --pull never | 不拉取远程应用镜像 |
| --force-recreate | 明确替换应用容器 |

单应用容器替换可能造成短暂中断。新程序启动时可能执行数据库迁移，应检查启动日志。

日常更新不需要 docker compose down，尤其不要使用 down -v 删除数据卷；也不要手动删除 PostgreSQL、Redis 或它们的数据目录。

## 6. 验收运行提交、镜像与健康状态

核对运行提交：

~~~bash
docker exec sub2api /app/sub2api -version
printf '本次目标提交：%s\n' "$TARGET_COMMIT"
~~~

核对实际镜像 ID：

~~~bash
docker inspect sub2api --format '{{.Image}}'
printf '本次目标镜像：%s\n' "$NEW_IMAGE_ID"
~~~

镜像 ID 应完全一致，运行 commit 应对应目标提交。

检查状态和日志：

~~~bash
docker compose -p transitai-sub2api -f docker-compose.prod.yml ps
docker logs --since 10m --tail 120 sub2api
docker inspect sub2api --format '{{json .State.Health}}'
~~~

starting 时稍后再检查，直到 healthy。unhealthy、反复重启、数据库迁移错误或配置错误时，不算上线成功。

检查应用内部健康接口：

~~~bash
docker exec sub2api sh -c 'wget -q -T 5 -O- "http://127.0.0.1:${SERVER_PORT:-8080}/health"'
~~~

最后用浏览器检查实际站点：

1. 强制刷新页面（Windows Chrome 可按 Ctrl+F5）。
2. 确认首页、登录、AI 聊天和绘图页面。
3. 按需做小规模功能验证，核对本次实际修改的功能。

HTTP 200、容器 Up 或网站能打开，不能单独证明新代码生效。提交、镜像、健康和实际功能均确认后，才完成本次更新。

## 7. 常见问题

### 网站看起来没有更新

~~~bash
git log -1 --oneline
docker exec sub2api /app/sub2api -version
docker inspect sub2api --format '{{.Image}}'
~~~

- 源码未变：检查是否已推送、是否拉取正确仓库。
- 源码新而运行 commit 旧：按生产 Compose 重新构建，核对镜像版本后重建应用。
- 容器已新而页面旧：强制刷新；如有 CDN 或反向代理缓存，再检查缓存，并核对是否存在前端目录挂载覆盖镜像文件。

### 构建失败

保留现有运行容器，查看构建日志中第一处实际错误。网络下载中断可修复后重试原命令；类型检查或编译错误需要先修代码。不要跳过构建直接重建容器，也不要默认用 --no-cache 解决所有问题。

### 更新后启动失败

~~~bash
docker logs --tail 200 sub2api
docker inspect sub2api --format '{{json .State}}'
docker ps -a --filter name=sub2api
~~~

保留日志和备份，先确定是否已执行数据库迁移，再决定回退方式。

## 8. 断线后恢复本次操作

~~~bash
cd /home/ubuntu/transitai-sub2api
ls -dt /root/transitai-backup-*
read -r -p '输入本次备份目录完整路径：' BACKUP_DIR
test -d "$BACKUP_DIR"
~~~

确认输入的是本次更新对应的目录，再恢复变量：

~~~bash
APP_IMAGE=$(cat "$BACKUP_DIR/image-tag-before.txt")
OLD_IMAGE_ID=$(cat "$BACKUP_DIR/image-id-before.txt")
~~~

如果第 3、4 步已分别写入目标记录，再读取：

~~~bash
TARGET_COMMIT=$(cat "$BACKUP_DIR/source-target.txt")
NEW_IMAGE_ID=$(cat "$BACKUP_DIR/image-id-target.txt")
~~~

文件不存在，说明相应步骤未完成。不要把空变量带入上线命令。如果当前源码已不同于 TARGET_COMMIT，先查明原因，不要沿用旧记录构建。

## 9. 有条件回退应用镜像

仅在确认旧程序仍兼容当前数据库、应用数据，且原生产配置仍适用时使用。已经执行数据库迁移后，不能假定旧程序一定兼容。

新终端先按第 8 节选择正确的备份目录，再执行：

~~~bash
cd /home/ubuntu/transitai-sub2api
APP_IMAGE=$(cat "$BACKUP_DIR/image-tag-before.txt")
OLD_IMAGE_ID=$(cat "$BACKUP_DIR/image-id-before.txt")
docker image inspect "$OLD_IMAGE_ID" --format '{{.Id}}'
docker tag "$OLD_IMAGE_ID" "$APP_IMAGE"
docker compose -p transitai-sub2api -f docker-compose.prod.yml up -d --no-deps --no-build --pull never --force-recreate sub2api
docker exec sub2api /app/sub2api -version
docker logs --tail 80 sub2api
docker inspect sub2api --format '{{json .State.Health}}'
~~~

回退时不要重新构建：源码可能已经是新版，构建会再次生成新版镜像。镜像切换不会自动回退源码、生产配置或数据库。

必须恢复数据库时，先暂停业务写入，保留故障时数据与日志，评估备份后新增数据，再制定数据库和应用数据的一致性恢复方案。恢复可能覆盖备份后的数据，因此这里不提供直接覆盖生产数据库的一键命令。

## 10. 复用于其他服务器

逐一替换部署目录、Git 远程与分支、Compose 项目名、所有 Compose 文件及其顺序、环境文件、服务和容器名、二进制路径与健康接口。迁移后先重新执行第 1 节核对。

镜像标签从运行容器读取，版本和提交从 Git 与二进制读取，无需每次手工修改。备份和旧镜像应保留至验收完成，并超过你的回退保留期。
