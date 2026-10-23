[![docker image build](https://github.com/Indexyz/als/actions/workflows/docker-image.yml/badge.svg)](https://github.com/Indexyz/als/actions/workflows/docker-image.yml)


语言: [English](README.md) | 简体中文

# ALS - 另一个 Looking-glass 服务器

## 快速开始 (Docker 环境)
```
docker run -d --name looking-glass --restart always --network host ghcr.io/indexyz/als:latest
```

镜像支持 `linux/amd64` 和 `linux/arm64`。`latest` 跟随默认分支 `master`，由该分支的推送、定时构建和手动运行更新。Git 标签构建会发布同名镜像标签，不更新 `latest`；需要固定部署版本时，请使用已发布的标签或 digest。Pull Request 只构建镜像，不推送。

镜像工作流使用具有 `packages: write` 权限的 `GITHUB_TOKEN`，自动推送到小写的 `ghcr.io/<仓库所有者>/<仓库名>`，无需配置 Docker Hub 凭据。首次成功发布后，请将 GHCR 软件包的可见性设为 **Public**，以允许匿名拉取。Fork 仓库请使用对应的镜像地址。

本地构建使用 Node.js 24.22.1 和 Go 1.27.3，前端步骤见 [UI 构建说明](ui/README.md)。两份 Dockerfile 均使用仓库提交的 npm 锁文件安装前端依赖。

[DEMO](http://lg.hk1-bgp.hkg.50network.com/)

如果不想使用 Docker , 您可以使用编译好的[服务器端](https://github.com/Indexyz/als/releases)

## 配置要求
 - 内存: 32MB 或更好

## 如何修改配置
```
# 你需要在 docker 命令中传递环境变量设置参数: -e KEY=VALUE
# 你可以在 环境变量表 中找到 KEY
# 例如，将监听端口改为 8080
docker run -d \
    --name looking-glass \
    -e HTTP_PORT=8080 \
    --restart always \
    --network host \
    ghcr.io/indexyz/als:latest
``` 

## 环境变量表
| Key                       | 示例                                                                | 默认                                                    | 描述                                                                             |
| ------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| LISTEN_IP                 | 127.0.0.1                                                              | (全部  IP)                                                   | 监听在哪一个 IP 上                                                     |
| HTTP_PORT                 | 80                                                                     | 80                                                         | 监听在哪一个端口上                                                              |
| SPEEDTEST_FILE_LIST       | 100MB 1GB                                                              | 1MB 10MB 100MB 1GB                                         | 静态文件大小列表, 使用空格隔开                                          |
| LOCATION                  | "this is location"                                                     | (请求 ipapi.co 获取) | 服务器位置的文本                                                                         |
| PUBLIC_IPV4               | 1.1.1.1                                                                | (从在线获取)                            | 服务器的 IPv4 地址                                                          |
| PUBLIC_IPV6               | fe80::1                                                                | (从在线获取)                            | 服务器的 IPv6 地址                                                          |
| DISPLAY_TRAFFIC           | true                                                                   | true                                                       | 实时流量开关                                                      |
| ENABLE_SPEEDTEST          | true                                                                   | true                                                       | 测速功能开关                                                            |
| UTILITIES_PING            | true                                                                   | true                                                       | Ping 功能开关                                                                 |
| UTILITIES_SPEEDTESTDOTNET | true                                                                   | true                                                       | Speedtest.net 功能开关                                                        |
| UTILITIES_TRACEROUTE       | true | true | Traceroute 功能开关 |
| UTILITIES_MTR              | true | true | MTR 功能开关 |
| UTILITIES_NEXTTRACE        | true | true | NextTrace 功能开关 |
| UTILITIES_IPERF3          | true                                                                   | true                                                       | iPerf3 服务器功能开关                                                               |
| UTILITIES_IPERF3_PORT_MIN | 30000                                                                  | 30000                                                      | iPerf3 服务器端口范围 - 开始                                                         |
| UTILITIES_IPERF3_PORT_MAX | 31000                                                                  | 31000                                                      | iPerf3 服务器端口范围 - 结束                                                           |
| SPONSOR_MESSAGE           | "Test message" or "/tmp/als_readme.md" or "http://some_host/114514.md" | ''                                                         | 显示节点赞助商信息 (支持 Markdown, 支持 URL/文字/文件 (文件需要映射到容器中, 使用映射后的路径) |


## 功能
- [x] HTML 5 速度测试
- [x] Ping - IPv4 / IPv6
- [x] iPerf3 服务器控制
- [x] 实时网卡流量显示
- [x] Speedtest.net 客户端
- [x] Ping / Traceroute / MTR / NextTrace 原生面板与受限 WebSocket API

Ping、Traceroute、MTR、NextTrace 均直接通过 Go 探测，诊断路径不再调用外部命令。网页终端、`/session/:session/shell`、本地 `als --shell` 和旧命令转发代码均已移除，`UTILITIES_FAKESHELL` 不再生效。前后端只接受合法 IPv4、IPv6 或 ASCII/Punycode 域名，探测参数由服务器固定。

Ping 使用 Go ICMP socket，校验回包来源、序号和随机负载；Traceroute/MTR/NextTrace 直接调用 `github.com/nxtrace/NTrace-core/trace` v1.7.5。MTR 每跳最多探测 10 次并实时刷新完整统计快照。路由测试共享一个并发槽，Ping 可并行。ALS 需要相应网络套接字权限；Linux 可静态构建，macOS 需要 CGO 和系统 libpcap，发布流程已改为在 macOS 原生构建，不再回退到命令。iPerf3 和 Speedtest.net 保持原有固定命令集成。接口及部署说明见 [WebSocket 协议文档](docs/diagnostics-websocket.md)。

## License

Code is licensed under MIT Public License.

* If you wish to support my efforts, keep the "Powered by WIKIHOST Opensource - ALS" link intact.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=Indexyz/als&type=Date)](https://star-history.com/#Indexyz/als&Date)
