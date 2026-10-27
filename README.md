# feedsystem_video_go

基于 Go + Vue 3 的短视频 Feed 系统，含账号、视频、点赞、评论、关注、Feed 流、私信、通知，支持 Redis 缓存、RabbitMQ 异步 Worker、分片上传、SSE 实时推送、接口限流、Docker Compose 部署。



**注: 本项目基于[LeoninCS/feedsystem_video_go](https://github.com/LeoninCS/feedsystem_video_go)二次开发，感谢海豹学长的源码以及技术支持**

## 功能

| 模块 | 功能 |
|------|------|
| 账号 | 注册、登录、Refresh Token、改名、改密、登出、头像上传、个人简介、主页统计、用户查询 |
| 视频 | 普通上传、Chunk分片上传、断点续传、封面上传、发布、查询、作者作品、详情缓存、#话题标签 |
| 点赞 | 点赞、取消点赞、是否已赞、已赞列表、RabbitMQ 异步落库、热度更新、SSE 通知 |
| 评论 | 发布、删除、列表、@username 提及通知、RabbitMQ 异步落库、热度更新、SSE 通知 |
| 关注 | 关注、取关、粉丝列表、关注列表、粉丝/关注计数、SSE 通知 |
| Feed | 推荐流、关注流、点赞榜、热榜、话题流、冷热分离、游标分页、短视频沉浸播放 |
| 私信 | 发送私信、私信列表 |
| 通知 | 点赞/评论/关注事件通知、提及通知、SSE 实时推送、通知列表、未读计数、已读标记 |
| 工程 | Docker Compos、Worker 拆分运行、限流、pprof、健康检查 |

## Docker Compose 一键启动

```bash
docker compose up -d --build
```

访问：
- 前端：`http://localhost:5173`
- 后端 API：`http://localhost:8080`
- RabbitMQ 管理台：`http://localhost:15672`（`admin` / `123456`）

Docker Compose 会读取 `.env`

JWT_SECRET缺省时使用 `feedsystem-dev-secret-key`，生产环境请修改 `JWT_SECRET`。


## 本地开发

```bash
# 启动依赖
docker compose up -d mysql redis rabbitmq

# 后端
cd backend
go mod download
CONFIG_PATH=configs/config.compose-local.yaml go run ./cmd

# 前端
cd frontend
npm install && npm run dev
```

## 接口清单

### 账号 `/account`
| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/register` | 否 | 注册（限流 5次/时/IP） |
| POST | `/login` | 否 | 登录（返回双token） |
| POST | `/refresh` | 否 | 刷新 access_token（用 refresh_token） |
| POST | `/findByID` | 否 | 按 ID 查用户 |
| POST | `/findByUsername` | 否 | 按用户名精准查 |
| POST | `/findSimilarName` | 否 | 按用户名模糊查 |
| POST | `/getProfile` | 否 | 用户主页（视频数/获赞/粉丝数） |
| POST | `/logout` | JWT | 登出（同时双 token失效） |
| POST | `/rename` | JWT | 改名（同时旧双 token失效，返回新双token） |
| POST | `/uploadAvatar` | JWT | 上传头像（jpg/png/webp，≤10MB） |
| POST | `/updateProfile` | JWT | 更新简介/头像 |
| POST | `/changePassword` | JWT | 改密码（需旧密码，同时双token失效） |

### 视频 `/video`
| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/publish` | JWT | 发布视频（自动提取 #话题） |
| POST | `/uploadVideo` | JWT | 上传视频文件（mp4，≤200MB） |
| POST | `/uploadCover` | JWT | 上传封面（jpg/png/webp，≤10MB） |
| POST | `/chunk/init` | JWT | 初始化分片上传（文件 MD5、大小、分片数） |
| POST | `/chunk/upload` | JWT | 上传单个分片（multipart，含分片 MD5 校验） |
| POST | `/chunk/status` | JWT | 查询已上传分片 |
| POST | `/chunk/complete` | JWT | 合并分片 |
| POST | `/listByAuthorID` | 否 | 按作者查视频 |
| POST | `/getByID` | 否 | 按ID查视频(缓存优化) |
| POST | `/search` | 否 | 根据关键字查询相关视频(title+desc) |

### 点赞 `/like`
| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/like` | JWT | 点赞 |
| POST | `/unlike` | JWT | 取消点赞 |
| POST | `/isLiked` | JWT | 是否已赞 |
| POST | `/listMyLikedVideos` | JWT | 我赞过的视频 |

### 评论 `/comment`
| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/listAll` | 软鉴权 | 评论列表（未登录只能看前10条） |
| POST | `/publish` | JWT | 发布评论（支持 @username 提及） |
| POST | `/delete` | JWT | 删除评论 |

### 关注 `/social`
| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/follow` | JWT | 关注 |
| POST | `/unfollow` | JWT | 取关 |
| POST | `/isfollowed` | JWT | 是否关注 |
| POST | `/getAllFollowers` | JWT | 粉丝列表 |
| POST | `/getAllVloggers` | JWT | 关注列表 |
| POST | `/getCounts` | JWT | 粉丝/关注计数 |

### Feed `/feed`
| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/listLatest` | 软鉴权 | 最新视频（游标分页） |
| POST | `/listLikesCount` | 软鉴权 | 点赞排行（复合游标） |
| POST | `/listByPopularity` | 软鉴权 | 热度榜（快照分页） |
| POST | `/listByFollowing` | JWT | 关注流 |
| POST | `/listByTag` | 软鉴权 | 按 #话题 浏览 |

### 通知 `/notification`
| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| GET | `/stream?token=<access_token>` | 是 | SSE 实时推送，也支持 `Authorization: Bearer <token>` |
| POST | `/list` | 是 | 最近通知 |
| POST | `/markRead` | 是 | 标记已读；传 `id` 标记单条，省略 `id` 标记全部 |
| POST | `/unreadCount` | 是 | 未读计数 |

### 私信 `/message`
| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/send` | JWT | 发送私信 |
| POST | `/list` | JWT | 对话列表 |

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `JWT_SECRET` | `feedsystem-dev-secret-key` | JWT 签名密钥，生产须改 |
| `SERVER_PORT` | `8080` | 后端监听端口 |
| `MYSQL_HOST` / `MYSQL_PORT` | `localhost/3306` | MySQL 地址 |
| `MYSQL_USER` / `MYSQL_PASSWORD` | `root/123456` | MySQL 账号密码 |
| `MYSQL_ROOT_PASSWORD` | `123456` | MySQL root 密码 |
| `MYSQL_DATABASE` | `feedsystem_video` | MySQL 数据库名 |
| `REDIS_HOST` / `REDIS_PORT` | `localhost/6379` | Redis 地址 |
| `REDIS_PASSWORD` | `123456` | Redis 密码 |
| `REDIS_DB` | `0` | Redis DB |
| `RABBITMQ_HOST` / `RABBITMQ_PORT` | `localhost/5672` | RabbitMQ 地址 |
| `RABBITMQ_USER` / `RABBITMQ_PASS` | `admin` / `123456` | RabbitMQ 账号 |

如果要更改请自行添加 `.env`。

## 运维与可观测性

- `GET /test` 返回后端健康状态。
- 本地配置默认开启 pprof：`localhost:6060`。
- 上传文件写入 `backend/.run/uploads`；Docker 环境挂载到 `backend_uploads` volume。
- Redis 用于 Token 缓存、视频实体缓存、Feed 时间线、热榜窗口、分布式锁、分片上传会话。
- RabbitMQ Topic Exchange 覆盖点赞、评论、关注、热度、视频时间线事件。
