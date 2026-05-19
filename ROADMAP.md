# 项目 Roadmap — Betrayal at House on the Hill 数字化

我已系统通读项目代码与素材，下面给出一个分阶段、每阶段都可独立验收的 roadmap。

**设计原则**

- 每个阶段结束都能跑出"更完整一点的可玩版本"，避免长时间无产出。
- 每阶段都有**明确的验收准则**（manual + 自动化），不打勾不进入下一阶段。
- 充分利用 `assets/raw/`（规则书、奸徒剧本、求生指南、卡牌图、人物图、表格）做数据化驱动，避免手写大量 Go 字面量。
- 数据 → 玩法 → UI 三层解耦，每层都可独立测试。
- **不强行重构**：在已实现的菜单/场景/相机/板面/抽牌之上增量演进。

**目录**

- §0 当前基线（已完成的事 — 详见 §A 附录）
- §1 总览：分阶段目标与每阶段产出
- §2 阶段 1 — 素材结构化（数据底座）
- §3 阶段 2 — 多人 + 回合 + 属性面板
- §4 阶段 3 — 骰子检定 + 速度移动
- §5 阶段 4 — 三类卡牌（事件 / 物品 / 预兆）+ Haunt Roll
- §6 阶段 5 — Haunt 进入：第一个完整剧本闭环
- §7 阶段 6 — 怪物 + 战斗
- §8 阶段 7 — 剧本批量数据化（全 50/100 剧本）
- §9 阶段 8 — UI/UX 美化与中文化
- §10 阶段 9 — 存档 / 读档 / 重玩
- §11 阶段 10 — 联机（可选）
- §A 附录：当前已落地的图像识别管线
- §B 附录：`assets/raw/` 资产盘点
- §C 附录：跨阶段约定（命名 / 包结构 / 测试 / 命令）

---

## 0. 当前基线（一句话）

菜单 → 游戏场景已通；起始三房与基础+扩展共 66 张地块已切好；点击相邻格可移动 / 抽牌 / 旋转 / 放置，门连通校验生效。`tileMeta` 已经带 `Doors + Floors`，但**业务侧只用了 Doors，Floors 还没消费**。所有桌游核心机制（玩家属性、骰子、卡牌、Haunt、剧本、战斗）尚未开始。

详细实现细节（图像识别算法、阈值、校准过程、改动汇总、抽样校验）见 §A 附录。

---

## 1. 总览

| 阶段 | 主题 | 一句话产出 | 体量(d) |
| --- | --- | --- | --- |
| 0 | 基线 | 地图切牌 + 楼层识别（已完成） | — |
| 1 | 素材结构化 | doc/xls/jpg → 项目内 yaml/json | 3–5 |
| 2 | 多人 + 回合 | 1–6 名探险者轮转，属性面板可见 | 2–3 |
| 3 | 骰子 + 速度 | 1d6/2d6 + speed 决定每回合步数 | 2 |
| 4 | 三堆卡牌 | 进入新房抽对应卡，omen 触发 Haunt Roll | 3–4 |
| 5 | 第一个剧本闭环 | 选 1 号剧本端到端跑通 | 4–5 |
| 6 | 怪物 + 战斗 | 攻防 might 对抗，伤害结算 | 2 |
| 7 | 剧本批量化 | 50（+50 扩展）剧本数据驱动 | 5–8 |
| 8 | UI 美化 | ebitenui + 中文字体 + 卡面渲染 | 3–4 |
| 9 | 存读档 | json 落盘 + 重玩同种子 | 1–2 |
| 10 | 联机 | LAN 房主 + 客户端镜像（可选） | 5–8 |

每阶段开头都列：**目标 / 输入 / 涉及文件 / 验收准则**。不强求顺序绝对线性，阶段 1 是后续所有阶段的前置；2/3 可并行；4 依赖 2；5 依赖 4；6/7 互相独立；8 何时做都可以。

---

## 2. 阶段 1 — 素材结构化（数据底座）

> 这是最先做、最关键的一步。所有后续阶段都要"读结构化数据"，而不是再写一遍中文字面量。

### 2.0 当前进度

**已落地（本次提交）**：

- `cmd/gendoors` 现在同时副产 `assets/datafs/tile_meta.yaml`（65 条），含 `id/source/row/col/doors/floors` 等几何字段 + `name_cn/name_en` 占位文本字段。**重新生成时几何字段被覆盖，但人类填的文本字段会保留**（小型自实现 yaml 反读保边逻辑）。
- 新增 `cmd/genmeta`：从 `assets/raw/Workshop/831685264.json`（29283 行）解析所有 `ObjectStates` + `ContainedObjects` + `States` 树，输出：
    - `assets/datafs/monsters.yaml`：7 色怪物 pawn 池（红 24 + 橙 24 + 绿 16 + 蓝 9 + 紫 6 + 玫红 6 + 黄 6）含每只 GUID。
    - `assets/datafs/tts_index.yaml`：277 条命名实体（去重后）含 nickname/kind/guid/description，作为后续阶段 5/6/7 的查询字典。
- 新增 `cmd/genxls`：纯 Go 解析 .xls（`github.com/extrame/xls`，无需 LibreOffice / 外部工具）：
    - `小黑屋/真相表.xls` → `assets/datafs/omens.yaml`：13 预兆 × 13 事件房间的 Haunt-roll 查询矩阵（matrix[ri][oi] = scenario_id）。预兆中文名来自表头，英文名在 `cmd/genxls` 的硬编码表中。
    - `小黑屋/奸徒身份表.xls` → `assets/datafs/scenarios_index.yaml`：50 个剧本的奏徒目标描述（`{id, traitor}`，完全连续 1..50）。
- 新增 `cmd/genrooms`：纯 Go 解析中文规则 PDF（`github.com/ledongthuc/pdf`，无需 LibreOffice）：
    - `小黑屋/Betrayal at House on the Hill_cn.pdf` → `assets/datafs/rooms.yaml`：45 个有特别规则的房间，每条 `{name_cn, name_en, floors[4]bool, rule_text}`。
    - 房间识别策略：以孤立 `(` 行为锚点，前一非空行必须为纯 ASCII（英文房名）以过滤规则正文中的括号；`(...)` 行内提取 `(basement|ground|upper|roof)` 关键字解析 floors。
    - 文本截断锚点：`以上房间皆无特别规则` 是规则书在最后一个房间（酒窖 / Wine Cellar）后面的总结句，用作附录起点防止 rule_text 吞掉事件牌附录。
- 新增 `cmd/gendoc`：纯 Go 解析 OLE2 .doc（无需任何第三方 OLE 库）——直接将全文件字节对重解释为 UTF-16LE code units，过一道 `[\p{Han}].{1,40}` 正则扫出汉字主导的连续文本 run：
    - `小黑屋/房间表.doc` → `assets/datafs/rooms_doc.yaml`：43 个带中文名 + 类型标签的房间（`{name_cn, kind_cn, kind}`，17 event + 13 omen + 5 item + 8 base，0 unknown）。
    - 为后续手填 `tile_meta.yaml` 提供完整中文名清单 + 类型参考，并补上 `rooms.yaml` 设计上不收入的“仅出现于房间表但规则书中未列出”的房间。
- 新增 `assets/datafs/`（独立子包，不依赖 ebiten）`//go:embed *.yaml`，避免 `pkg/data` 测试时拉入 X11 依赖。
- 新增 `pkg/data/`：`LoadTiles / LoadMonsters / LoadTTSIndex / LoadOmens / LoadScenarios / LoadRooms / LoadRoomsDoc`，单测覆盖锚点地块楼层 / 怪物分组 count / 关键 Haunt NPC 名存在性 / 预兆 × 房间金丝雀单元格 / 50 剧本连续性 / 房间名楼层金丝雀（陵墓·裂缝·塔楼·酒窖·阁楼）+ 酒窖 rule_text 长度回归校验 / .doc 房间名·类型金丝雀 + “unknown 零条”覆盖性校验 / **starter 三块 `start_cell` 金丝雀（他 tile 必为 nil）**。`go test ./pkg/data` 7 个测试全过。
- **运行时接入**：`assets/image.go` 在 `LoadStarterTiles / loadDeckTiles` 中通过 `sync.Once` 懒加载 `tile_meta.yaml`，给每个 `RoomTile` 填 `NameCN/NameEN`；HUD 悬停时显示 `中文 / English [#id]`（CJK 字体接入在阶段 8，stub 先走 ASCII）。
- **starter 几何配置驱动**：`tile_meta.yaml` 的 starter 三块新增 `start_cell: [x, y]` 字段（`pkg/data.TileMeta.StartCell *[2]int`），`cmd/gendoors` 重跑时与 `name_cn/name_en` 同等越过几何覆写保边。`assets.LoadStarterTiles` 返回 `[]StarterPlacement`（tile + cell），`pkg/scene/game.go` 不再硬编码 starter 列。玩家初始位置为 id=1002（入口大厅）的 cell。
- **一键抽牌**：`pkg/scene/game.go` HUD 右上角新增 `Draw [D] (n)` 按钮 + `D` 键快捷键，选中当前房间第一个“有门且邻格为空”的黄色高亮格抽一张牌进入 `stateDrawing`；按钮在牌堆空 / 无候选格 / 抽牌中自动置灰。`GOOS=js GOARCH=wasm go build` 结果 17.8MB。
- **数据源统一**：删除 `assets/doors_gen.go`（原生成的 `var tileMeta = map[int]TileMeta{...}`），`assets/image.go` 新开 `tileCacheEntry { Meta + NameCN/EN + StartCell }` 单一 `sync.Once` 懒加载，`tileMetaOf(id)` 提供 `Doors+Floors` 查询；`cmd/gendoors` 删除 stdout `var tileMeta` 写出代码路径及头部文档中 `> assets/doors_gen.go` 重定向。`tile_meta.yaml` 现为全部几何元数据唯一源，双写不一致风险彻底消除；vet/test/wasm 全绿。
- `gopkg.in/yaml.v3` 已通过 `go mod tidy` 提升为 direct require；`github.com/extrame/xls`、`github.com/ledongthuc/pdf` 作为新依赖加入。

**未做（doc 与图片限制）**：

- `房间表.doc` 已通过 `cmd/gendoc` 的纯 Go UTF-16LE 扫描提取完成（不需要 LibreOffice / unioffice）。其他 doc【`标记牌表.doc / pw牌表.doc / 奸徒剧本.doc / 求生剧本.doc`】同样可用同一思路提取，留作阶段 4/5 按需动工。
- `cmd/gencards / cmd/genchars`：当前环境 `file` / ImageMagick 无尺寸输出，需要先在能跑的环境下用 Go 写一段图片尺寸检测，再决定网格切分参数。**已留作下一子任务**。

### 2.1 原始计划（保留供后续推进）

**目标**：把 `assets/raw/` 里 doc/xls/jpg/json 中的规则数据，提取为项目可直接 `embed` + 反序列化使用的结构化文件。

**输入**

- `小黑屋/规则WORD版本.doc`、`小黑屋/Betrayal at House on the Hill_cn.pdf`
- `小黑屋/房间表.doc`、`小黑屋/真相表.xls`、`小黑屋/奸徒身份表.xls`、`小黑屋/标记牌表.doc`、`小黑屋/pw牌表.doc`
- `小黑屋/奸徒剧本.doc`、`小黑屋/求生剧本.doc`
- `剧本-网络收集/小黑屋-原版-奸徒密谋.docx`、`小黑屋-原版-求生指南.doc`
- `剧本-网络收集/小黑屋-扩展-奸徒密谋.pdf`、`小黑屋-扩展-求生指南.pdf`
- `Workshop/831685264.json`（TTS 模组）

**子任务**

1. **新增 `cmd/genmeta/`**
    - 读 `Workshop/831685264.json`，提取 `Nickname / GUID / Description / States` 等字段。
    - 与 `assets/datafs/tile_meta.yaml` 的 `tiles[]`（按 ID 索引）按几何位置或 GUID 对齐，给每张地块补 `Name / Outdoor / SpecialKind`。
    - 输出 `assets/tile_meta.yaml`（YAML 比 Go 字面量好 review）。
2. **doc/xls → yaml**
    - `房间表.doc` → `assets/data/rooms.yaml`：每条 `{id, name_cn, name_en, floors, event_text, item_text}`。
    - `真相表.xls` → `assets/data/omens.yaml`：预兆触发的 Haunt 真相矩阵（剧本 # × 触发 omen 名）→ scenario_id。
    - `奸徒身份表.xls` → `assets/data/scenarios_index.yaml`：每个剧本编号 → 名字 / 阵营 / 章节锚点。
    - `标记牌表.doc` → `assets/data/tokens.yaml`：标记图标 → 规则文本。
    - `pw牌表.doc` → `assets/data/cards.yaml`：分 `event / item / omen` 三类。
3. **卡牌图按编号切分**
    - `小黑屋/卡片A.jpg ~ 卡片H.jpg` 是多卡拼图；写一个 `cmd/gencards/`，按已知行列切到 `assets/cards/{event,item,omen}/{id}.png`。
    - 每类有专属背面：`卡片A背面.jpg` 等，复用即可，不重复切。
4. **新增解析包 `pkg/data/`**
    - `LoadRooms() ([]Room, error)` / `LoadCards() (CardDB, error)` / `LoadScenarios() ([]Scenario, error)`
    - 用 `//go:embed assets/data/*.yaml`，`gopkg.in/yaml.v3` 反序列化。
    - 启动时一次性加载，缓存为只读单例。
5. **不动正在用的 `tileMeta`**
    - `assets/image.go` 已经把 `Doors / Floors / NameCN / NameEN / StartCell` 全部从 `tile_meta.yaml` 懒加载（`assets/doors_gen.go` 已删除，`cmd/gendoors` 只写 yaml）；阶段 1 末尾让该 cache 同时 attach 来自扩展 yaml 的 `Outdoor / SpecialKind / EventTrigger` 等字段到 `RoomTile`。

**涉及文件**

```
cmd/genmeta/main.go        新增
cmd/gencards/main.go       新增
assets/data/*.yaml         新增（4–5 份）
assets/cards/**/*.png      新增（切图产物）
pkg/data/loader.go         新增
assets/image.go            扩展：合并 tile_meta.yaml 字段
go.mod                     新增 yaml.v3
```

**验收准则**

- [ ] `go run ./cmd/genmeta` / `./cmd/gencards` 一键生成所有数据文件，不需要人工编辑。
- [ ] `go test ./pkg/data/` 加载全部 yaml 不报错；房间表 ≥ 60 条，剧本 ≥ 50 条。
- [ ] 启动游戏，HUD 上 hover 房间能显示中文房间名（来自 `tile_meta.yaml`）。
- [ ] 数据文件全部 < 200KB，git diff 友好。

---

## 3. 阶段 2 — 多人 + 回合 + 属性面板

**目标**：从"单 pawn 探索"扩展到桌游真正的人物 + 回合制。

**输入**

- `小黑屋/人物A.jpg ~ 人物D.jpg`（每张是 3 人 × 2 性别，共 12 名探险者，含初始属性数）
- 或 `Workshop/*.json` 中的角色定义

**子任务**

1. **新增 `cmd/genchars/`**：从人物图按格子切出 12 张头像 → `assets/chars/{id}.png`，并把 `Name / Sanity / Knowledge / Might / Speed` 四项基础属性写到 `assets/data/characters.yaml`。每项属性是数组 `[s0, s1, s2, ...]`（被攻击/事件会上下移动指针）。
2. **`pkg/player/player.go` 升级**
    ```go
    type Stat struct{ Track []int; Idx int }
    type Player struct {
        ID, Name string
        Pos      board.Cell
        Floor    tile.Floor
        Sanity, Knowledge, Might, Speed Stat
        Items    []CardID
        Dead     bool
    }
    func (p *Player) StatValue(k StatKind) int { ... }
    func (p *Player) Shift(k StatKind, delta int) { ... } // 撞 0 = Dead
    ```
3. **新增 `pkg/turn/`**
    - `Order` 维护 1–6 名 player 顺序。
    - `Phase`: `start → move → action → draw → end`。
    - `Engine` 暴露 `Current()`, `EndTurn()`, `OnEvent(...)`。
4. **场景层接入**
    - `GameScene` 不再持有单 `*Player`，而是 `players []*Player + turn *turn.Engine`。
    - 当前操作者高亮显示；其他 pawn 灰显在各自 cell。
    - 新增菜单：开局选择 1–6 名玩家、各自挑角色。
5. **HUD 升级**
    - 左下角属性面板：4 项 stat track（横条上一个滑块），鼠标 hover 高亮当前值。
    - 顶栏增加 "Round X • Turn: 张三"。

**涉及文件**

```
cmd/genchars/main.go            新增
assets/chars/*.png              新增
assets/data/characters.yaml     新增
pkg/player/player.go            重写
pkg/turn/turn.go                新增
pkg/scene/setup.go              新增（角色选择子场景）
pkg/scene/game.go               接入 turn.Engine
```

**验收准则**

- [ ] 菜单 → 角色选择 → 进游戏，3 名玩家正确轮转。
- [ ] 当前 pawn 视觉高亮，"End Turn" 按钮把控制权交给下一位。
- [ ] 属性面板四项可见；按测试快捷键 `1/2/3/4` + `+/-` 可推动指针，撞 0 后该玩家被标记 Dead 并跳过回合。
- [ ] 单元测试：`turn_test.go` 覆盖正反向轮转 + 跳过死亡玩家 + 全员死亡时返回 `GameOver`。

---

## 4. 阶段 3 — 骰子检定 + 速度移动

**目标**：把"点击就走"换成桌游真实的"speed = 本回合可移动步数 + 经过门口可能要 might 检定"。

**子任务**

1. **`pkg/dice/`**
    - `Roll(n int) int`：`n` 颗骰子，每颗等概率掷 0/0/1/1/2/2 之一，返回总和。
    - 单元测试断言均值与方差在置信区间内。
2. **回合开始计 stepsLeft = speed**
    - `move` 阶段每移一格扣 1；为 0 时强制进入 `action` 阶段。
3. **检定 UI**
    - 卡牌 / 剧本要求"X 检定 ≥ Y"时弹一个掷骰小窗：动画掷 X 骰 → 显示数字 → 自动比对 Y → 给出成功/失败。
    - 暴露 `RollAgainst(stat StatKind, target int) Outcome`。
4. **移动规则细化**
    - 经过"楼梯""窗户""陷门"等特殊房间用 `tile.SpecialKind` 触发对应规则（先桩，留接口）。

**涉及文件**

```
pkg/dice/dice.go            新增 + 测试
pkg/turn/turn.go            扩展 stepsLeft
pkg/scene/dialog/dice.go    新增掷骰窗口（先用矩形+数字渲染）
pkg/tile/tile.go            新增 SpecialKind 枚举
```

**验收准则**

- [ ] 一回合内移动格数 = speed，超出格数报错且不动。
- [ ] 通过 `T` 测试键打开掷骰窗，能掷 1–8 颗骰子并展示结果。
- [ ] `dice_test.go`：100k 次 4 颗骰子均值在 `4 ± 0.05` 区间。

---

## 5. 阶段 4 — 三堆卡牌 + Haunt Roll

**目标**：进入未抽过的房间会按图标触发抽对应卡（事件/物品/预兆）。每抽一张预兆做 Haunt Roll：掷 6 颗骰 < 累计预兆数 → 进入 Haunt 阶段。

**前置**：阶段 1（卡牌已切好+yaml 已有）、阶段 3（骰子）。

**子任务**

1. **`pkg/cards/`**
    - 三堆 `EventDeck / ItemDeck / OmenDeck`，每张牌 `{ID, Name, Kind, Text, Effects []Effect}`。
    - `Effect` 先做枚举：`stat_shift / draw_card / move / require_roll / spawn_token`。所有"复杂"事件先标 `Effect{Kind:"manual"}`，runtime 弹文本提示由玩家自助裁决（非常重要：不阻塞流程）。
2. **房间触发**
    - `tile_meta.yaml` 增加 `event_trigger / item_trigger / omen_trigger bool` 字段（部分房间名右下角有图标，由 `cmd/gendoors` 识别或手工 yaml 覆盖），运行时由 `assets.tileMetaOf` 暴露三个 bool。
    - 进入新房 + Revealed=false → 翻面 + 触发对应抽牌。
3. **HauntTrack**
    - `pkg/turn/haunt.go`：`OmenCount` 累加；每抽 omen 立即 Haunt Roll。
    - 失败 → `OnHauntTriggered(scenarioID, hauntPlayer)`，调用阶段 5。
4. **卡牌弹窗**
    - 抽牌时全屏半透明遮罩 + 卡面图（来自 `assets/cards/`）+ 中文文本 + "Apply" / "Manual" 两个按钮。

**涉及文件**

```
pkg/cards/cards.go              新增
pkg/cards/effects.go            新增（基础 effect 解释器）
pkg/turn/haunt.go               新增
pkg/scene/dialog/card.go        新增
assets/data/cards.yaml          来自阶段 1
assets/doors_gen.go             ❌ 已删（geometry 全部走 tile_meta.yaml）
```

**验收准则**

- [ ] 抽到事件牌弹卡面 + 中文文本，点击 Apply 后属性正确变化（用 `stat_shift` 能演示的牌做用例）。
- [ ] 抽预兆牌 N 次后做 Haunt Roll，掷骰 < N 时确实跳转剧本选择（先用桩函数 `printf`）。
- [ ] `cards_test.go`：构造 5 名玩家 + 模拟抽 10 张预兆，统计触发回合分布与桌游手册一致。

---

## 6. 阶段 5 — Haunt 进入：第一个完整剧本闭环

**目标**：选取**一个**剧本（推荐 1 号 / 难度低 / 无需特殊计算）端到端跑通：奸徒判定 → 求生 / 奸徒规则书分发 → 胜负判定。**这是首次让游戏"能玩到结束"**。

**输入**：`小黑屋/奸徒剧本.doc` + `求生剧本.doc` 中的 1 号剧本文本。

**子任务**

1. **奸徒判定矩阵**
    - 进 Haunt 时根据 `OmenCount × 触发房间名` → 查 `assets/data/omens.yaml`（阶段 1）→ scenarioID。
    - `assets/data/scenarios_index.yaml` 给出 `traitor_rule`（"持有最多预兆者""触发者左侧玩家"等）。
2. **`pkg/scenario/`**
    - 每个剧本一个文件：`scenario_001.go` 实现 `Scenario` 接口：
        ```go
        type Scenario interface {
            ID() int
            Name() string
            HeroBriefing() string   // 求生指南文本（中文 wrapped）
            TraitorBriefing() string // 奸徒密谋
            OnHauntStart(g *Game)
            OnTurnEnd(g *Game)
            CheckWin(g *Game) (heroes, traitor bool)
        }
        ```
    - 阶段 5 只实现 `scenario_001`。
3. **剧本 UI**
    - Haunt 触发后，弹"分发简报"对话框：奸徒看奸徒页，其他人看求生页（同一台机器先用顺序展示+遮罩"现在传给 X"）。
    - 屏幕顶栏切换为 `HAUNT • Scenario #1: 〈名字〉`。
4. **胜负判定**
    - 每回合末跑 `CheckWin`，任一阵营胜利 → 进入 `GameOverScene` 显示赢家、回合数、关键事件 log。
5. **事件日志**
    - 新增 `pkg/log/event.go`：所有重要事件 append（移动、抽牌、检定、Haunt、伤害）。GameOver 显示最后 30 条。

**涉及文件**

```
pkg/scenario/scenario.go         接口
pkg/scenario/scenario_001.go     实现
pkg/scene/briefing.go            分发简报弹窗
pkg/scene/gameover.go            结算页
pkg/log/event.go                 事件日志
```

**验收准则**

- [ ] 走脚本：3 玩家 → 抽够预兆 → 触发 Haunt → 简报弹窗 → 双方按各自规则操作 → CheckWin 判赢 → GameOver 页。
- [ ] 整局可在 < 30 分钟内打完，无崩溃。
- [ ] 所有规则文本来自 yaml/doc 抽取，**没有中文硬编码在 .go 里**。
- [ ] 关键事件日志可滚动查阅。

---

## 7. 阶段 6 — 怪物 + 战斗

**目标**：阶段 5 跑通后，多数剧本会涉及怪物。引入战斗 = 两玩家或一玩家+怪物互相 might 掷骰，差额 = 伤害。

**输入**：`小黑屋/怪兽.jpg / 怪兽A~C.jpg`（含 might / speed 等数值）。

**子任务**

1. **`cmd/genmonsters/`**：切怪物头像 + 提取数值到 `assets/data/monsters.yaml`。
2. **`pkg/monster/`**：`Monster` 与 `Player` 共享 `Combatant` 接口。
3. **战斗流程**：进入同格 / 相邻格 + 触发条件 → `combat.Resolve(attacker, defender)`：
    - 双方各掷 `Might` 颗骰；高者胜，差额作为伤害（按防方选择 `Sanity` 或 `Might` 扣）。
4. **怪物 AI**：先实现"贪心移动到最近的英雄"。每剧本可在 `Scenario.OnTurnEnd` 里覆盖。
5. **板面渲染**：怪物用方形带图标 + 红描边，区分玩家圆形 pawn。

**验收准则**

- [ ] 测试场景：固定 1 玩家 + 1 怪物，互相站邻格，10 回合内必分胜负。
- [ ] `combat_test.go`：5000 局蒙特卡洛，胜率分布与解析期望一致。
- [ ] 怪物可同时出 ≥ 3 只无明显卡顿。

---

## 8. 阶段 7 — 剧本批量数据化

**目标**：把"一个剧本一个 .go 文件"改造为"剧本是 yaml 数据"，让 50 + 50 个剧本都能跑（或至少能在不写代码的情况下加载）。

**子任务**

1. **DSL 设计**：剧本 yaml 描述：
    ```yaml
    id: 5
    name: "蛛网"
    haunt: traitor       # 或 hero / both
    triggers:
      - on: haunt_start
        do: spawn_monster
        kind: spider
        count: 3
        room: kitchen
    win_conditions:
      heroes: [all_traitor_players_dead]
      traitor: [any_hero_at_zero_sanity]
    ```
2. **`pkg/scenario/dsl.go`**：解释器，覆盖阶段 5/6 的硬编码 hooks。
3. **批量录入**：写一个 `cmd/scrapescenarios/`，从 `小黑屋-原版-奸徒密谋.docx` / `求生指南.doc` 提取每剧本三段文本（标题/简介/胜负）+ 人工补上 DSL 字段。**不强求 50 个全跑通**：阶段 7 验收只要求 ≥ 5 个剧本可玩通关。
4. **剧本浏览器**：菜单加 "Scenario Library"，可按编号查阅简报全文（即使尚不可玩）。

**验收准则**

- [ ] 5 个新剧本（ID 1/3/5/10/20）端到端可通关。
- [ ] 剩余剧本至少能加载文本不报错；不可玩的标 `playable: false`。
- [ ] DSL 字段文档化在 `pkg/scenario/dsl.md`（**唯一允许的 .md 设计文档**）。

---

## 9. 阶段 8 — UI/UX 美化与中文化

**目标**：把 `ebitenutil.DebugPrintAt` 全部替换为正式 UI；中文字体支持；卡面/属性面板美化。

**子任务**

1. **接入 `ebitenui v0.6`**（go.mod 已有）。
2. **中文字体**：内嵌 `Noto Sans CJK SC` 或 `思源黑体`，`pkg/ui/font.go` 提供 `Face(size int)`。
3. **统一对话框框架** `pkg/ui/dialog.go`：覆盖卡牌窗 / 掷骰窗 / 简报窗。
4. **侧栏**：左侧玩家列表（头像+四项 stat 条），右侧事件日志（实时滚动）。
5. **底栏**：当前阶段 + 剩余步数 + 操作提示。
6. **设置页**：分辨率 / 音量 / 字号 / 中英切换。

**验收准则**

- [ ] 全屏 1080p / 1440p / 2160p 自适应不裂。
- [ ] 中文文本无乱码，字号 ≥ 14px 易读。
- [ ] 所有交互入口都是按钮 + 快捷键双通道。

---

## 10. 阶段 9 — 存档 / 读档

**子任务**

1. `pkg/save/save.go`：序列化 `{seed, board, players, decks, turn, haunt, log}` → `~/.house/save_*.json`。
2. 菜单加 "Continue"，自动列最近 5 个存档。
3. 每回合结束自动 `autosave.json`。

**验收准则**

- [ ] 任意阶段保存 → 退出 → 重启 → 加载，状态完全一致（包括牌堆顺序）。
- [ ] 存档跨版本兼容用 `version` 字段；不兼容时优雅降级提示。

---

## 11. 阶段 10 — 联机（可选）

**子任务**

1. `pkg/net/`：房主 = 状态权威，客户端只发 `Intent`（move / draw / roll）；服务端广播 `Event`。
2. 协议：JSON over TCP，先不做加密 / 抗作弊。
3. 同步性测试：两个 client 各自 `replayLog == authority.replayLog`。

**验收准则**

- [ ] 局域网 2 人能完整跑通阶段 5 的剧本 #1。
- [ ] 任一方掉线 30s 内能重连恢复。

---

## A. 附录 — 当前已落地的图像识别管线（基线）

> 下面是阶段 0 已经完成的事，**后续阶段不要轻易改这套数字**。

### A.1 资源命名英文化

`assets/image/` 下中文文件名已英文化：

| 旧名 | 新名 |
| --- | --- |
| `主地图.jpg` | `main_map.jpg` |
| `主地图-背面.jpg` | `main_map_back.jpg` |
| `入口大厅.png` | `entry_hall.png` |
| `扩展地图.jpg` | `extension_map.jpg` |
| `扩展地图-背面.jpg` | `extension_map_back.jpg` |

同步：`assets/image.go` 五个常量、`cmd/gendoors/main.go` 三处路径。`raw/Workshop/831685264.json` 的中文 `Nickname` 不动（属于上游模组数据）。

### A.2 楼层属性识别算法

每张地块背面一个房屋剪影叠四种 label，亮黄表示该楼层可放：

| y 中心（占 cell 高） | 含义 |
| --- | --- |
| 0.25 | `FloorRoof`（仅扩展） |
| 0.40 | `FloorUpper` |
| 0.55 | `FloorGround` |
| 0.70 | `FloorBasement` |

像素判定（与门检测共用 `isDoorYellow`）：

```go
r8 > 200 && g8 > 150 && b8 < 80 && r8 - b8 > 100
```

阈值（`cmd/gendoors/main.go`）：

```go
floorBandHRatio  = 0.06   // 采样窄带高度
floorYellowRatio = 0.10   // 黄色像素占比阈值；命中区域典型 0.30~0.40
```

### A.3 校准记录

1. 第一版 y 中心拍脑袋 `{0.18, 0.42, 0.62, 0.82}` → 大量 Floors 全 false。
2. 对 7 个代表性 cell 1/20 步长扫描得真实峰值 `{0.25, 0.40, 0.55, 0.70}`。
3. ID 0 / 1（"上层""地下室"锚点）背面留白 → `force` 回调手动指定 Upper-only / Basement-only。

### A.4 `processSheet` 签名

```go
type forceFn func(idx int, doors *[4]bool, floors *[4]bool, note *string)

func processSheet(
    frontPath, backPath string,
    cols, rows, idBase int,
    kind string,
    out *[]entry,
    defaultFloors [4]bool,
    force forceFn,
)
```

`backPath == ""` 跳过楼层识别，使用 `defaultFloors`（如入口大厅 = Ground）。

### A.5 生成结果

`assets/datafs/tile_meta.yaml`（已删除的 `assets/doors_gen.go` 旧格式仅作历史参考）：

```yaml
tiles:
  - id: 0
    source: base
    row: 0
    col: 0
    doors:  [true, true, true, true]
    floors: [false, true, false, false]
    name_cn: ""
    name_en: ""
    note: "base r0 c0 (manual: upper-floor anchor)"
  ...
```

总计：base 44 + extension 19 + starter 3 = **66 个有效地块**。抽样校验：

| ID 范围 | 示例 | Floors | 解读 |
| --- | --- | --- | --- |
| 0 | base r0 c0 | `{F,T,F,F}` | 手动，Upper 锚点 |
| 1 | base r0 c1 | `{F,F,F,T}` | 手动，Basement 锚点 |
| 2~8 | base r0 c2~c8 | `{F,T,T,T}` | Upper+Ground+Basement |
| 35~43 | base r3~r4 | `{F,F,F,T}` | 仅 Basement |
| 501 | extension r0 c1 | `{T,T,T,T}` | 全四层 |
| 510 | extension r1 c0 | `{T,F,F,F}` | 仅顶层 |
| 1000~1002 | starter | `{F,F,T,F}` | 强制 Ground |

### A.6 是否切 JSON 存储

**保留生成 Go 文件**。理由：体量 < 70、纯几何产物、编译期类型校验、无需热加载。  
触发迁移条件（任一）：① 字段超 5–6 个且 ≥ 2 个需手编；② 多语言；③ 第三方编辑器；④ 玩家自定义牌组。  
迁移路径：`cmd/gendoors` 改输出 `tile_meta.json` + `embed` + 启动 `Unmarshal`，调用方零改动。

### A.7 已知边界

- `cmd/gendoors` 仅依赖 `image / image/jpeg / image/png`，CI 无 X11 也能跑。
- 主程序 `go build ./...` 仍需 `X11/Xlib.h`（`ebiten/internal/glfw`），与本次改动无关。
- 正反图分辨率必须严格相等（已验证 `4500×2250` / `4500×900` / `1418×450`）。

### A.8 校准采样原始数据

```text
== r0 c2 (Upper+Ground+Basement) ==        == r0 c9 (Upper only) ==
y=0.40: 0.393  ###############             y=0.40: 0.396  ###############
y=0.55: 0.353  ##############              y=0.50~0.75: 0.000
y=0.70: 0.361  ##############

== r3 c0 (Ground only) ==                  == r3 c5 (Basement only) ==
y=0.55: 0.357  ##############              y=0.70: 0.357  ##############

== extension_map_back r0 c1 (full 4) ==
y=0.25: 0.446  #################
y=0.40: 0.396  ###############
y=0.55: 0.359  ##############
y=0.70: 0.353  ##############
```

由此选定中心 `{0.25, 0.40, 0.55, 0.70}`、阈值 `0.10`、窄带高 `0.06`。

---

## B. 附录 — `assets/raw/` 资产盘点

| 路径 | 体量 | 用途 | 阶段 |
| --- | --- | --- | --- |
| `小黑屋/规则WORD版本.doc` `Betrayal..._cn.pdf` | 中文规则全本 | 抽阶段术语 + 检定算法 | 1 |
| `小黑屋/房间表.doc` | 房间×事件文本 | 阶段 1 → `rooms.yaml` | 1, 4 |
| `小黑屋/真相表.xls` | omen × 房间 → 剧本号 | Haunt Roll 矩阵 | 4, 5 |
| `小黑屋/奸徒身份表.xls` | 剧本号 → 奸徒规则 | 阶段 5 入口 | 5 |
| `小黑屋/标记牌表.doc` | 标记图标含义 | tokens.yaml | 4, 7 |
| `小黑屋/pw牌表.doc` | 事件/物品/预兆所有卡 | cards.yaml | 4 |
| `小黑屋/奸徒剧本.doc` `求生剧本.doc` | 50 剧本中文文本 | 阶段 5/7 简报 | 5, 7 |
| `剧本-网络收集/*` | 原版+扩展剧本完整版 | 补充 50 + 50 | 5, 7 |
| `小黑屋/卡片A~H.jpg` `卡片*背面.jpg` | 拼图卡牌正反面 | 切图 → `assets/cards/` | 1 |
| `小黑屋/人物A~D.jpg` | 12 名探险者 | `assets/chars/` + `characters.yaml` | 2 |
| `小黑屋/怪兽*.jpg` `标记*.jpg` | 怪物/标记图 | 阶段 6 + 阶段 4 | 4, 6 |
| `Workshop/831685264.json` | TTS 模组数据 | 房间名补全 + 备查 | 1 |
| `Models/*.obj` | 3D 模型 | 不用（保留） | — |
| `Images/*.jpg` | 截图素材 | 临时参考 | — |
| `PDF/*.PDF` | 5 份扫描 | 备查 | — |

---

## C. 附录 — 跨阶段约定

### C.1 包结构（最终目标）

```
cmd/
  gendoors/      ✅ 已有（图像几何识别）
  genmeta/       阶段 1（合并 TTS + 表格）
  gencards/      阶段 1（卡牌切图）
  genchars/      阶段 2（人物切图）
  genmonsters/   阶段 6
  scrapescenarios/  阶段 7
  proxy/         ✅ 已有
  main.go        ✅ 已有
pkg/
  board/         ✅
  camera/        ✅
  component/     ✅
  deck/          ✅（阶段 4 可拆 deck/event deck/item deck/omen）
  player/        阶段 2 重写
  scene/         ✅（持续扩展子场景）
  tile/          ✅
  utils/         ✅
  data/          阶段 1 新增
  turn/          阶段 2 新增
  dice/          阶段 3 新增
  cards/         阶段 4 新增
  monster/       阶段 6 新增
  scenario/      阶段 5 新增
  log/           阶段 5 新增
  ui/            阶段 8 新增
  save/          阶段 9 新增
  net/           阶段 10 新增
assets/
  image/         ✅
  data/          阶段 1
  cards/         阶段 1
  chars/         阶段 2
  monsters/      阶段 6
  fonts/         阶段 8
  datafs/        ✅ (tile_meta.yaml + 6 份 yaml)
  image.go       ✅
```

### C.2 测试 / 命令

```bash
# 重新生成地块元数据（写 assets/datafs/tile_meta.yaml；保留 name_cn/name_en/start_cell）
go run ./cmd/gendoors

# 阶段 1 起每个 cmd 都遵循"幂等 + 不依赖运行时 ebiten"
go run ./cmd/genmeta
go run ./cmd/gencards
go run ./cmd/genchars

# 单独校验生成器（CI 无 X11 也能跑）
go vet ./cmd/...

# 主程序构建（需要 X11）
go build ./...
go test ./pkg/...
```

### C.3 命名 / 风格

- 楼层索引 `[0..3] = {Roof, Upper, Ground, Basement}` 是生成器、运行时、测试三方共识，**永远不要改顺序**。
- 数据 yaml 用英文 key，文本字段同时保留 `name_cn / name_en` 双语。
- 中文字符串**只允许出现在** `assets/data/*.yaml` 与 `assets/raw/`，源码 `*.go` 内禁止硬编码中文（注释除外）。
- 每个 `cmd/gen*` 必须满足：① 幂等 ② 输出可 diff 可读 ③ 不依赖 ebiten 运行时。

### C.4 任务清单（按阶段聚合）

- 阶段 1：[x] gendoors 副产 tile_meta.yaml [x] genmeta（monsters / tts_index）[x] genxls（omens / scenarios_index）[x] genrooms（rooms.yaml，45 条带规则房间）[x] gendoc（rooms_doc.yaml，43 名·类型）[x] pkg/data + 7 测试 [x] yaml.v3 / extrame/xls / ledongthuc/pdf 提为 direct [x] assets/image.go 接入 yaml + HUD hover 显示房名（CN 需阶段 8 接入 CJK 字体后可见）  
  下一子任务：[ ] 手填 tile_meta.yaml 剩余 ~62 条 name_cn/name_en（可从 rooms.yaml 拼出 45 条）[ ] gencards（待图片尺寸探测方案）[ ] genchars [ ] doc → cards.yaml / tokens.yaml（LibreOffice headless 转 docx 后用 unioffice）
- 阶段 2：[ ] genchars [ ] characters.yaml [ ] player 升级 [ ] turn engine [ ] setup scene [ ] HUD 属性面板
- 阶段 3：[ ] dice [ ] dialog/dice.go [ ] stepsLeft 接入
- 阶段 4：[ ] cards 包 [ ] effects 解释器 [ ] haunt.go [ ] dialog/card.go [ ] tile triggers
- 阶段 5：[ ] scenario 接口 [ ] scenario_001 [ ] briefing [ ] gameover [ ] event log
- 阶段 6：[ ] genmonsters [ ] monster 包 [ ] combat.Resolve [ ] 怪物贪心 AI
- 阶段 7：[ ] scenario DSL [ ] scrapescenarios [ ] 5 个剧本可通关 [ ] dsl.md
- 阶段 8：[ ] ebitenui 接入 [ ] CJK 字体 [ ] 对话框框架 [ ] 设置页
- 阶段 9：[ ] save 序列化 [ ] autosave [ ] continue 菜单
- 阶段 10：[ ] net 协议 [ ] 状态权威 [ ] 重连
