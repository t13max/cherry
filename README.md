# 🍒 欢迎使用 cherry！

![cherry logo](https://img.shields.io/badge/cherry--game-cherry-red)
![cherry license](https://img.shields.io/github/license/cherry-game/cherry)
![go version](https://img.shields.io/github/go-mod/go-version/cherry-game/cherry)
![cherry tag](https://img.shields.io/github/v/tag/cherry-game/cherry)

- **高性能分布式的 Golang 游戏服务器框架**
- 采用 Golang + Actor Model 构建，具备高性能、可伸缩等特性
- 简单易学，让开发者更专注于游戏业务开发

## 📢 重要更新

- **新增 Actor model 实现**
- **新增 simple 网络数据包结构**（id(4bytes) + dataLen(4bytes) + data(n bytes)）
- **示例代码迁移**：[examples](https://github.com/cherry-game/examples)
- **组件库迁移**：[components](https://github.com/cherry-game/components)
- **文档地址**：[点击查看](https://cherry-game.github.io/)

## 💬 讨论与交流

- 加入 QQ 群：[191651647](https://jq.qq.com/?_wv=1027&k=vdIddlK0)

## 📖 示例

### 单节点精简版聊天室

适合新手熟悉项目，具备以下特性：

- 基于网页客户端，构建 HTTP 服务器
- 采用 WebSocket 作为连接器
- 使用 JSON 作为通信格式
- 实现创建房间、发送消息、广播消息等功能

准备步骤：

* [环境安装与配置](https://cherry-game.github.io/guides/install-go.html)
* 源码位置：[examples/demo_chat](https://github.com/cherry-game/examples/tree/master/demo_chat)

### 多节点分布式游戏示例

适合作为基础框架构建游戏服务端，特性如下：

- 基于 H5 构建客户端
- 搭建 Web 服、网关服、中心服、游戏服等节点
- 实现区服列表、多 SDK 帐号体系、帐号注册、登录、创建角色等功能

准备步骤：

* [环境安装与配置](https://cherry-game.github.io/guides/install-go.html)
* 源码位置：[examples/demo_cluster](https://github.com/cherry-game/examples/tree/master/demo_cluster)

## 🌟 核心功能

### 组件管理

- 以组件方式组合功能，便于统一管理生命周期
- 支持自定义组件注册，灵活扩展
- 可配置集群模式和单机模式

### 环境配置

- 支持多环境参数配置切换
- 基于 profile 文件配置系统和组件参数
- 可自由拆分或组装 profile 子文件，精简配置

### Actor 模型

- 个 Actor 独立运行于一个 goroutine，逻辑串行处理
- 接收本地、远程、事件三种消息，各自有独立队列按 FIFO 原则消费
- 可创建子 Actor，消息由父 Actor 路由转发
- 支持跨节点 Actor 通信

### 集群 & 注册发现

- 提供三种发现服务实现方式
- 基于 nats.io 实现 RPC 调用，提供同步 / 异步方式

### 连接器

- 支持 tcp、websocket、http server、http client 等
- kcp 组件计划后续集成

### 消息 & 路由

- 实现多种网络数据包结构及编解码
- 支持消息路由、序列化（json/protobuf）、事件处理

### 日志

- 基于 uber zap 封装，性能优良
- 支持多文件输出、日志切割等功能

## 🧰 扩展组件

### 已开放组件

* **data-config 组件** ：策划配表读取管理，支持多种加载方式及数据查询
* **etcd 组件** ：基于 etcd 封装，用于节点集群和注册发现
* **gin 组件** ：集成 gin 实现 http server 功能，增加管理周期和中间件组件
* **gorm 组件** ：集成 gorm 实现 mysql 数据库访问，支持多数据库配置
* **mongo 组件** ：集成 mongo-driver，支持多 mongodb 数据库配置
* **cron 组件** ：基于 robfig/cron 封装，性能良好

### 待开放组件

- db 队列、gopher-lua 脚本、限流组件等

## 🎮 游戏客户端 SDK

### 通信协议格式

* [协议结构图](_docs/pomelo-protocol.jpg)
* [pomelo wiki 协议格式](https://github.com/NetEase/pomelo/wiki/%E5%8D%8F%E8%AE%AE%E6%A0%BC%E5%BC%8F)

### 各平台客户端

* **unity3d** ：[YMoonRiver/Pomelo_UnityWebSocket](https://github.com/YMoonRiver/Pomelo_UnityWebSocket-2.7.0)、[NetEase/pomelo-unityclient](https://github.com/NetEase/pomelo-unityclient) 等
* **cocos2dx** ：[NetEase/pomelo-cocos2dchat](https://github.com/NetEase/pomelo-cocos2dchat)
* **Javascript** ：[pomelonode/pomelo-jsclient-websocket](https://github.com/pomelonode/pomelo-jsclient-websocket) 等
* **C** ：[topfreegames/libpitaya](https://github.com/topfreegames/libpitaya)、[NetEase/libpomelo](https://github.com/NetEase/libpomelo/) 等
* **iOS** ：[NetEase/pomelo-iosclient](https://github.com/NetEase/pomelo-iosclient) 等
* **Android & Java** ：[NetEase/pomelo-androidclient](https://github.com/NetEase/pomelo-androidclient) 等
* **微信** ：[wangsijie/pomelo-weixin-client](https://github.com/wangsijie/pomelo-weixin-client)

## 🗺️ 游戏服务端架构示例

![game-server-architecture](_docs/game-server-architecture.jpg)

## 🙏 致谢

- [pomelo](https://github.com/NetEase/pomelo)
- [pitaya](https://github.com/topfreegames/pitaya)

## Study

