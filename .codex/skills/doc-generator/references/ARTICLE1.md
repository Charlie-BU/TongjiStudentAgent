# HeartCompass 向 Immortality 的重构过程

先放 MVP 结算图：

![MVP](https://charlie-assets.oss-rg-china-mainland.aliyuncs.com/images/article/FINAL_3ab585d660.png)

2026 年 4 月 10 号，我做了一个重大的决定 ———— 从零开始重写这个工程。了解过本项目心路历程（详见 [HEARTCOMPASS.md](./HEARTCOMPASS.md)）的大家都知道，最一开始我是希望研发一个建立于爱情关系的 Agent，所以很多功能和模块都是基于这个场景而设计的。但随着研发逐渐深入，我渐渐认为这样的场景过于局限，远远达不到本系统的能力上限，所以我逐步把能力边界拓展到虚拟人（VirtualFigure）。这意味着很多最初场景下的设定，尽管不尽合理，但仍需要强行复用。特别是数据建模，这里的 pitfall 尤为巨大，很多实例名、类名、变量名和函数名完全脱离了其本身语义。直到从美国旅行回来后，经过了一些思想斗争，还是决定从 0 开始重写整个项目，并基于它的能力范围把它命名为 `Digital Immortality`（数字永生）。

原有架构的不合理之处，以及重构版本对其的改进 / 解决方案，主要有以下几部分：

1. 人物表与关系表拆分冗余

原先的人物表命名为 `Crush`，关系表为 `RelationChain`。但在本系统场景下，同一个用户和 ta 的一个 Crush 只能形成一个关系链 `RelationChain`，把这两个表拆分就十分冗余。

解决方案：
把两个表合为一个，命名为 `FigureAndRelaion`（即 `FR`）。表中涵盖了一个 Figure 的众多 “Intrinsic Fields 固有属性”，如 `figure_gender`、`figure_mbti`、`figure_birthday`、`figure_occupation`、`figure_education` 等，用来表达一个人物最基础的画像。

2. 画像信息来源不足，导致刻画能力弱

原先只通过用户和人物的共同经历 `Event`，以及双方聊天话题 `ChatTopic` 来补充关系与人物画像的细粒度信息。但客观地讲，仅靠固有字段和这两类信息，完全没办法刻画一段关系和一个人物画像，甚至可信度不足 60%。这不仅是数据建模问题，有效信息抽取也是大问题。

解决方案：
今年上半年，社区里一下子涌现了很多“蒸馏”人物的 Skill。相信关注的大家或许有所耳闻“同事.skill”“前任.skill”等诸多 Agent Skills，在 github 上迅速斩获几万 stars。得知这个消息时我正在加州和发小度假，一度十分失落，心想坏了，我正在做的产品已经有人提前落地了。但回国后仔细研究这些 Skill 的 repo，我又大松一口气：这些 Skills 全都是详细的 Markdown 文件，强依赖 OpenClaw 这类 Agent 消费，完全没有工程落地。唯一工程化的部分，是借助一系列 Python 脚本处理微信、飞书等导出的聊天记录和图片元信息。

于是我在自己的 OpenClaw 里引入了一个“蒸馏自己”的 skill，尝试喂入一些关于我的资料，然后查看这个 skill 对我的蒸馏结果。如下：
![Charlie-BU](https://charlie-assets.oss-rg-china-mainland.aliyuncs.com/images/article/image_78bd67aa72.png)
逐个文件看下来，我发现这个蒸馏（或者说抽取）策略的效果还是很不错的。基于我给出的信息，它可以比较科学客观地构建出我的人物画像。

但问题出在“消费”上。`SKILL.md` 中明确写了以下运行规则：

```markdown
## 运行规则

1. **先读 `personality.md`**：了解核心价值观与思维方式。
2. **再读 `interaction.md`**：掌握沟通风格与表达习惯。
3. **按需读 `procedure.md`**：了解技术栈和工作方法。
4. **按需读 `memory.md`**：了解人生经历和重要节点。
5. **遇到矛盾读 `conflicts.md`**。
```

这意味着每轮对话中，但凡需要用到我的人物画像，`personality.md` 和 `interaction.md` 都是必读的；如果需要 `procedure.md` 和 `memory.md`，也会被**全量加载**。随着一个人的画像越来越完善，这些文件体积会持续膨胀；每轮消费时全量加载，会让真正与本轮相关的有效信息被大量无效信息稀释，性能和效果都很差。这个策略的弊端其实很好想象，我就不再一一展开。

HeartCompass 虽然覆盖的属性不足以完整刻画一段关系与一个人物画像，但向量化召回策略却完美解决了这个问题。我借鉴了这些 skills 的部分抽取策略，通过提示词工程把这些 Markdown 兼容到本系统场景中。为此我新增了一个表 `FineGrainedFeed`，用于存储 FR 的细粒度信息。每条细粒度信息归类为以下四个维度（dimension）：

- PERSONALITY 性格与价值观
- INTERACTION_STYLE 互动风格
- PROCEDURAL_INFO 程序性知识
- MEMORY 人生记忆与故事

另外，我把每个 FR 分为以下几类角色 figure_role：

- SELF 自己
- FAMILY 家人
- FRIEND 朋友
- MENTOR 导师
- COLLEAGUE 同事
- PARTNER 伴侣
- PUBLIC_FIGURE 公众人物
- STRANGER 陌生人

值得注意的是，每个 dimension 和每个 `figure_role` 都有其特定的画像抽取策略与边界限定。在抽取过程中，我根据当前 FR 的 `figure_role` 以及提供文本涉及到的 dimension，组合两类 prompt 来抽取“特定 role × 特定 dimension”的信息。

`FRBuildingGraph` 即整个抽取链路。详细策略见：[FRBuildingGraph](../src/agents/graphs/FRBuildingGraph/README.md)。

这样一来，任何一个 FR 的画像可以被其固有属性 Intrinsic Fields 和细粒度信息 FineGrainedFeed 非常完整地描述。完美地解决了问题。

3. ConversationGraph 消费方式

工作流与短期记忆裁剪策略详见：[ConversationGraph](../src/agents/graphs/ConversationGraph/README.md)

4. 短期记忆 Checkpointer

基于 `PostgresSaver`，用数据库存储图执行快照 checkpoint，而不是存放在内存 `InMemorySaver` 中。在首次引入时，需要 setup 自动在对应 URI 位置创建数据库表。

`checkpointer.py` 中提供了同步 `getCheckpointer()` 和异步 `agetCheckpointer()` 两种方式。但由于图节点中存在异步调用，这意味着必须使用异步 `agetCheckpointer()`，也意味着图调用必须使用 `graph.ainvoke()`。

5. Server 与 Service 的解耦与 CLI 支持

原先架构中，我计划暴露一系列 OpenAPI 接口，开放给用户操作数据库、调用服务的能力。于是我基于 Robyn 做了 Server 层，而数据库相关操作均被封装在 **Server 层中** 的 Service 层（耦合严重），并给每个 Service 上层包装了一层路由，注册在 Robyn Server 中。这本是一套合理的后端架构 SOP，但从深圳出差后，我的一些认知发生了迭代。认为这个架构（在当前场景、当下时代）并不是最优。

深圳出差的一个重大需求，是支持字节云 CLI。在当下 Agent 大行其道，诸多平台、服务都逐渐开放了 CLI 能力，来支持 Agent 原生调用，例如飞书 CLI。回到本项目，在很久前我就打算不开发前端。既然没有前端，自然也不需要 OpenAPI 接口来提供服务能力。

想想 OpenClaw、Claude Code，这些项目的产出是什么？通过什么提供服务能力？没错，CLI。我也同时意识到，把 Service 层和 Server 层耦合，是愚蠢的。Service 层应当只关注数据库操作以及系统的其他能力，而 Server 本身仅仅是提供 HTTP 接口的。

于是，下一步，我果断把 Service 层和 Server 层解耦。完全砍掉了 Server 层，砍掉了 Robyn。我们根本不需要 OpenAPI 了，也就不需要 Web 框架了。

取而代之的是 CLI 层。我将本系统的所有能力（eg. 注册登录、增删 FR、查看 FR 画像、调用图服务等）分成两部分（部分有重叠）：一部分在 channel（当前场景下即飞书）暴露，一部分在 CLI 暴露。一些重要能力（例如查看 FR 画像）我会在两侧同时暴露。

CLI 的实现把 Service 能力原生组织为命令树。顶层命令拆成 `doctor / setup / auth / fr / lark-service` 等（后续可能继续扩展），并统一支持 `--json`，保证 Agent 调用和人类手工调用都能稳定消费同一套输出协议。

CLI 在本地 Home 目录创建了 `~/.immortality`，专门存放与本系统相关的文件（环境变量、session、logs 等）。这是我在此前研发桌面端应用 MineContext 时积累下来的经验：把系统用户相关信息放在 Home 目录的隐藏目录中。为什么不放在用户拉取的 Immortality 源码目录里？很简单，如果放在源码目录，用户每次更新、卸载、重装都可能覆盖这些用户信息。

我把 `.env.example` 和一些必要文件放在 `cli/assets` 中，并在 `pyproject.toml` 配置 `[tool.setuptools.package-data]`，把这个 `assets` 目录和源码一起打包构建，安装到用户本地环境。

我在 `setup` 中加入了 Docker 配置数据库能力。核心目标是尽可能降低用户手动配置 PostgreSQL 的复杂性，减少版本、权限、初始化脚本、连通性排查这类环境问题带来的心智负担，让用户把注意力集中在 CLI 能力本身，而不是被环境搭建拖慢。

## 补充

- 原本计划引入 Viking 记忆库来保存**用户和虚拟 FR 对话过程中不真实的记忆**。但在真实运行环境下，我认为现有短期记忆及其 trim 策略已经可以覆盖其能力。再者，每轮对话的上下文体积已经够大，再引入记忆库召回信息作为 SystemMessage 会让体积进一步膨胀、影响性能，遂决定暂不引入 Viking。

- 后来我还是加入了 Server 层，提供 OpenAPI 接口。但没有启用。详见：[BOTTLENECK.md](BOTTLENECK.md)

- 外部 Channel（飞书）的接入没有很大变化，这里不多赘述。详见之前 HeartCompass 的研发文档：[HEARTCOMPASS.md](HEARTCOMPASS.md)
