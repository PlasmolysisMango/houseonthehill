# 项目 Roadmap

记录从图像识别管线到地块元数据存储的演进规划。当前已落地内容写在 "已完成" 区，
"短中期" 是直接可接的下一步，"长期" 是需要更多业务输入才动手的方向。

---

## 1. 已完成（基线）

### 1.1 资源命名英文化
- `assets/image/` 下的中文文件名全部改为英文：
  - `主地图.jpg` → `main_map.jpg`
  - `主地图-背面.jpg` → `main_map_back.jpg`
  - `入口大厅.png` → `entry_hall.png`
  - `扩展地图.jpg` → `extension_map.jpg`
  - `扩展地图-背面.jpg` → `extension_map_back.jpg`
- 同步更新 `assets/image.go` 常量与 `cmd/gendoors/main.go` 路径。

### 1.2 楼层属性识别
- `cmd/gendoors/main.go` 新增 `detectFloors`，在背面图每个 cell 中央列采样
  四个 y 中心 `{0.25, 0.40, 0.55, 0.70}`，对应 顶层 / 上层 / 地面 / 地下室。
- 复用 `isDoorYellow` 颜色判定 + 阈值 `floorYellowRatio = 0.10` + 窄带高度
  `floorBandHRatio = 0.06`，对图像有 10× 安全边际。
- `processSheet` 接受 `frontPath / backPath / defaultFloors / force`：
  - 入口大厅无背面图，使用默认 `Floors{Ground}`。
  - 锚点地块 "上层"(ID 0) 和 "地下室"(ID 1) 背面留白，由 `force` 回调写入。
- 生成结果 `assets/doors_gen.go` 的导出数据从 `tileDoors map[int][4]bool`
  升级为 `tileMeta map[int]TileMeta`（含 `Doors / Floors`）。

### 1.3 运行时结构
- `pkg/tile/tile.go`：
  - 新增常量 `FloorRoof / FloorUpper / FloorGround / FloorBasement`。
  - `RoomTile` 新增 `Floors [4]bool` 字段（不参与旋转）。
  - 新增方法 `(*RoomTile).AllowsFloor(floor int) bool`。
- `assets/image.go`：
  - 新增 `TileMeta` 公共类型。
  - `LoadStarterTiles` / `loadDeckTiles` 改用 `tileMeta` 同时填充 `Doors/Floors`。

### 1.4 已知边界
- 生成器无 ebiten 依赖，CI 无需 X11 即可跑：
  `go run ./cmd/gendoors > assets/doors_gen.go`
- 主程序构建仍依赖 X11（`X11/Xlib.h`），与本次改动无关。

---

## 2. 短期（直接可做）

### 2.1 业务侧消费 Floors
- 在 `pkg/board/` 放置 / 抽牌逻辑里加入"楼层校验"：
  当玩家位于某层时，从牌堆抽到的 `RoomTile` 必须 `AllowsFloor(curFloor) == true`，
  否则按规则压回 / 抽下一张。
- 在 `pkg/scene/game.go` 提示 UI 里展示当前可放置楼层。

### 2.2 楼层模型抽象
- 当前楼层用 `int` + 常量。建议：
  ```go
  type Floor int
  const (
      FloorRoof Floor = iota
      FloorUpper
      FloorGround
      FloorBasement
  )
  func (f Floor) String() string { ... }
  ```
- 增加 `Floor.Mask() [4]bool` 与 `MatchAny(mask [4]bool, floors ...Floor) bool`
  辅助函数，避免业务层直接索引下标。

### 2.3 生成器自检
- `cmd/gendoors/main.go` 在 `main()` 末尾增加一致性断言：
  - 任意 `Doors` 全 false → 警告（理论已被 `anyDoor` 过滤）。
  - 任意 `Floors` 全 false → 警告（除 starter 外不允许）。
  - 输出统计：每楼层覆盖牌数；门朝向分布。
- 用环境变量 `GENDOORS_VERBOSE=1` 控制是否打印到 stderr。

### 2.4 视觉 / 单测
- 在 `cmd/gendoors/` 加一个 `-debug=path/to/out.png` 模式，把识别带框住的可视化
  导出，便于调阈值。
- 新增 `assets/tilemeta_test.go`：
  - 验证 `tileMeta[0].Floors[FloorUpper]` 之类核心断言；
  - 与硬编码的若干样本对照，防止生成器误改回归。

---

## 3. 中期（需要更多业务字段时）

### 3.1 扩展 TileMeta
拟增字段（仅在确有规则消费时再加，避免过度设计）：

| 字段 | 含义 | 来源 |
| --- | --- | --- |
| `Name` | 房间名（中/英） | 手工或从 raw JSON 提取 |
| `Event` | 进入触发事件 / 物品 / 怪物 | 规则书 |
| `Outdoor` | 是否户外 | 背面图左上 "户外" 角标识别 |
| `Special` | 楼梯 / 电梯 / 隐藏通道等枚举 | 手工标注 |

### 3.2 户外 / 楼梯角标自动识别
- 主地图正面在某些 tile 左上角有 "户外" 文字，可加一个 `detectCorner` 类似
  `detectFloors` 的小函数。
- 楼梯类房间（地下室楼梯、神秘电梯等）在艺术图中有显著竖向通道，可用
  HSV/亮度阈值 + 形状粗判，必要时仍以白名单兜底。

### 3.3 数据来源整合
- `assets/raw/Workshop/831685264.json` 等是 TTS 模组数据，里面已经有
  `Nickname`、自定义状态等字段；可写一个 `cmd/genmeta` 把它和 gendoors 的
  几何识别结果合并，输出更完整的 `tileMeta`。

---

## 4. 是否切换到 JSON 存储

### 4.1 现状结论
**保留生成 Go 文件**。理由：
- 数据完全由图像自动产出，无需人工手维护。
- 编译期类型校验，运行期零解析、零失败路径。
- 体量小（< 70 条），git diff 友好。
- 二进制已 `embed` 图像，再多一份 JSON 没有部署收益。

### 4.2 切换的触发条件
出现以下任一条件再考虑迁移：
- TileMeta 字段超过 5~6 个，且其中 ≥ 2 个需要非程序员手动编辑。
- 需要做多语言（房间名、事件文案）。
- 第三方工具（地图编辑器 / mod 工具）也要消费同一份数据。

### 4.3 平滑迁移路径
现有 `TileMeta` 已是唯一聚合点，未来切 JSON 只需：
1. `cmd/gendoors` 输出从 `.go` 改为 `assets/tile_meta.json`（结构一致）。
2. `assets/image.go` 增加：
   ```go
   //go:embed tile_meta.json
   var tileMetaRaw []byte

   var tileMeta = func() map[int]TileMeta {
       m := map[int]TileMeta{}
       _ = json.Unmarshal(tileMetaRaw, &m)
       return m
   }()
   ```
3. 删除 `doors_gen.go`。
- 调用方 `loadDeckTiles` / `LoadStarterTiles` 不动。

---

## 5. 任务清单（可逐项打勾）

- [ ] 业务层接入 `Floors` 校验（板面放置 / 抽牌）
- [ ] `pkg/tile` 引入 `Floor` 强类型 + 工具方法
- [ ] gendoors 增加自检 + verbose 输出
- [ ] gendoors `-debug` 可视化模式
- [ ] `tilemeta_test.go` 关键断言
- [ ] 户外 / 楼梯角标自动识别
- [ ] TileMeta 字段扩展（按需）
- [ ] 评估 JSON 迁移触发条件，必要时按 §4.3 执行
