# gotmux 开发方案（执行路线图）

本文档是 gotmux 后续开发的唯一路线图。任何模型/开发者在写代码前必须先读完本文档。
任务必须**严格按照 Phase 顺序执行**，不允许跳过，不允许自行发明新任务。

---

## 一、项目目标（背景，一段话）

gotmux 是用 Go 复刻 tmux：小客户端通过 Unix socket 连接常驻 server，
server 持有 session / window / pane / PTY。目标是**行为和真 tmux 一致**，
不添加 tmux 没有的功能。当前架构方向正确，测试全绿，问题在于：
屏幕模型缺颜色、历史记录模型错误、两个超大文件拖慢迭代。
本方案按"越晚改代价越大"的顺序修复这些问题，然后打穿日常可用性的核心链条。

---

## 二、铁律（每个任务都必须遵守）

1. **真 tmux 是唯一标准。** 任何行为不确定时，在本机运行真 tmux 观察输出，
   照抄它的行为。不要凭记忆猜测 tmux 的行为。
2. **测试先行。** 先在 `scripts/compat_probe.sh` 或 Go 测试里写好
   以真 tmux 输出为准的预期，再写实现，直到测试通过。
3. **不发明功能。** tmux 没有的功能一律不做。
4. **每个任务结束时必须全部通过：**
   ```sh
   go build ./...
   go test ./...
   scripts/compat_probe.sh
   ```
5. **每个任务完成后更新 `docs/COMPATIBILITY.md`**，如实描述实现范围。
6. **提交粒度：** 一个任务一个（或几个）commit，commit message 用英文祈使句，
   风格与 `git log` 现有历史一致（如 `Support pane SGR colors`）。
7. **禁止实现长尾命令。** 在 Phase 1–6 全部完成之前，禁止新增或扩展以下命令：
   `lock-server`、`lock-session`、`lock-client`、`server-access`、
   `customize-mode`、`choose-client`、`confirm-before`、`command-prompt`、
   `display-menu`、`display-popup`、`suspend-client`、prompt history 相关命令。
   这些命令已有的存根保持现状即可。

---

## 三、每个任务的标准工作流程

对下面每一个 Phase / 子任务，固定按这个循环执行：

1. 读本文档中该任务的"步骤"和"验收标准"。
2. 用真 tmux 手动复现目标行为，记录准确输出（必要时写进 probe 脚本）。
3. 在 Go 测试或 `scripts/compat_probe.sh` 中新增会失败的测试。
4. 写实现代码，让测试通过。
5. 运行第二节第 4 条的三个命令，全部通过。
6. 更新 `docs/COMPATIBILITY.md`。
7. 提交 commit。

---

## Phase 0：拆分超大文件（纯机械操作，不改任何行为）

**为什么：** `internal/model/model.go`（约 3900 行）和
`internal/server/commands.go`（约 3800 行）太大，每次编辑都慢且容易改错。

**步骤：**

1. 把 `internal/server/commands.go` 按命令族拆成多个文件（同一个 package，
   只移动代码，不改任何函数签名和逻辑）：
   - `cmd_session.go`：new-session、attach、detach、kill-session、rename-session、
     list-sessions、switch-client 等 session 级命令
   - `cmd_window.go`：new-window、select-window、kill-window、move-window、
     link-window、swap-window、list-windows 等 window 级命令
   - `cmd_pane.go`：split-window、select-pane、kill-pane、resize-pane、
     swap-pane、join-pane、break-pane、capture-pane、pipe-pane 等 pane 级命令
   - `cmd_option.go`：set-option、show-options、set-hook、show-hooks、
     set-environment、show-environment
   - `cmd_key.go`：bind-key、unbind-key、list-keys、send-keys、send-prefix
   - `cmd_buffer.go`：set-buffer、show-buffer、list-buffers、delete-buffer、
     paste-buffer、load-buffer、save-buffer
   - `cmd_misc.go`：其余命令
2. 把 `internal/model/model.go` 同样拆分：
   - `server.go`（Server 结构及方法）、`session.go`、`window.go`、`pane.go`、
     `options.go`、`layout.go`（几何/布局相关）
3. 拆分对应的测试文件，按同样的命名对应。

**验收标准：**
- `go build ./... && go test ./... && scripts/compat_probe.sh` 全部通过。
- `git diff` 除了代码位置移动，不包含任何逻辑改动。
- 拆分后单个 .go 文件不超过 1000 行。

**禁止：** 拆分过程中"顺手"重构、改名、优化任何逻辑。

---

## Phase 1：给屏幕单元格加颜色和属性（SGR）

**为什么：** 这是最紧迫的地基改动。`internal/terminal/screen.go` 中的
`cell` 结构目前只有 `r rune` 和 `used bool`，没有颜色。颜色是所有终端程序的
日常输出，而且 capture-pane、redraw、copy-mode 全部依赖 cell 结构，
越晚加成本越高。

**步骤：**

1. 修改 `internal/terminal/screen.go` 中的 `cell` 结构，增加样式字段：
   ```go
   type cell struct {
       r     rune
       used  bool
       style Style
   }

   type Style struct {
       Fg    Color  // 前景色
       Bg    Color  // 背景色
       Attrs uint16 // 位标志：bold/dim/italic/underline/blink/reverse/hidden/strikethrough
   }

   type Color struct {
       Mode  uint8 // 0=默认色 1=标准16色(值0-15) 2=256色(值0-255) 3=RGB
       Value uint8
       R, G, B uint8
   }
   ```
2. 在 Screen 上增加"当前画笔样式"字段（`curStyle Style`），
   `putRuneLocked` 写入字符时把 `curStyle` 存进 cell。
3. 在 `applyCSILocked` 中实现 `m`（SGR）序列，支持以下参数：
   - `0` 重置；`1` bold；`2` dim；`3` italic；`4` underline；`5` blink；
     `7` reverse；`8` hidden；`9` strikethrough
   - `21`–`29` 对应属性关闭
   - `30`–`37` 前景标准色；`39` 前景默认色；`90`–`97` 前景亮色
   - `40`–`47` 背景标准色；`49` 背景默认色；`100`–`107` 背景亮色
   - `38;5;n` 前景 256 色；`48;5;n` 背景 256 色
   - `38;2;r;g;b` 前景真彩色；`48;2;r;g;b` 背景真彩色
4. 清屏/清行/删除字符等操作填充的空白 cell 使用当前背景色
   （与真 tmux/终端的 BCE 行为一致）。
5. `capture-pane` 增加 `-e` 支持：输出时把 cell 样式还原成转义序列。
   相邻 cell 样式相同时不重复输出转义序列；行尾按 tmux 行为处理。
   用真 tmux 的 `capture-pane -e -p` 输出作为对照写 probe 测试。
6. 修改 `internal/server/render.go`：redraw 时输出带样式的行
   （在每行内样式变化处插入 SGR 序列，行首重置）。
   注意 `renderPaneCanvas` 目前用 `[][]rune` 画布，需要改成带样式的画布。

**验收标准：**
- Go 单测：向 Screen 写入含 SGR 的字节流，`CaptureRows` 能取回正确样式。
- probe 新增用例：在真 tmux 和 gotmux 中分别运行
  `printf '\033[31mred\033[0m plain \033[1;44mboldblue\033[0m'`
  之类的命令，比较两者 `capture-pane -e -p` 输出一致。
- 附着 gotmux 客户端后运行 `ls --color=force`，颜色正常显示（手动验证并
  在 COMPATIBILITY.md 记录）。

---

## Phase 2：把历史记录（scrollback）合并进 Screen 网格

**为什么：** 当前历史是 `internal/model/ring.go` 的**字节流环形缓冲**，
和 Screen 的网格是两套模型。真 tmux 的 scrollback 就是网格的一部分。
copy-mode 需要按行、带换行标记地回滚，字节流做不到，必须现在改。

**步骤：**

1. 在 `Screen` 结构中增加历史存储：
   ```go
   history      [][]cell // 从主屏顶部滚出的行，旧行在前
   historyWraps []bool   // 与 history 对应的 wrap 标记
   historyLimit int      // 对应 tmux 的 history-limit 选项，默认 2000
   ```
2. 修改 `lineFeedLocked` / `scrollUpLocked`：**只在主屏（非 alt screen）**
   把滚出顶部的行 append 进 history；超过 historyLimit 时丢弃最旧的行。
   alt screen 期间不写 history（与 tmux 一致）。
3. Screen 增加方法：`HistoryLen() int`、`HistoryRows(...)`（供 capture-pane 用）、
   `ClearHistory()`。
4. `capture-pane` 的 `-S`/`-E` 支持负数行号（负数表示历史中的行），
   `-S -` 表示从历史最顶端开始。行为以真 tmux 为准写 probe 用例。
5. `clear-history` 改为调用 `Screen.ClearHistory()`。
6. 把 `history-limit` 选项接到 `Screen.historyLimit`
   （新 pane 创建时读取该选项）。
7. 删除对 `model.Ring` 的 pane 历史使用；如果 `Ring` 不再被任何代码引用，
   删除 `ring.go` 和它的测试。

**验收标准：**
- Go 单测：写入超过屏幕高度的行后，`HistoryLen` 正确，
  `capture-pane -S -5` 能取到历史行。
- probe 用例：真 tmux 和 gotmux 中运行 `seq 1 100`，
  比较 `capture-pane -p -S -50` 输出一致。
- `clear-history` 后 `#{history_size}` 为 0（同时实现 `history_size`、
  `history_limit` 两个 format 变量）。

---

## Phase 3：宽字符支持（CJK）

**为什么：** 中文/日文/韩文字符占 2 个屏幕格子。现在按 1 格处理，
任何含中文的输出都会错位。

**步骤：**

1. 添加依赖：`go get github.com/mattn/go-runewidth`。
2. `cell` 结构增加宽度标记：宽字符主 cell 记 `width=2`，
   其后跟一个占位 cell（`width=0`，不单独渲染）。
3. `putRuneLocked`：用 `runewidth.RuneWidth(r)` 判断宽度。
   宽度 2 的字符：如果光标在行尾只剩 1 格，先自动换行再写入；
   写入后光标前进 2。宽度 0（组合字符）：附加到前一个 cell（简单处理：忽略也可，
   但要在 COMPATIBILITY.md 里写明）。
4. 覆盖规则：向宽字符的任一半写入新字符时，另一半清成空格。
5. `capture-pane`、`render.go` 输出时跳过占位 cell。
6. 光标移动、删除字符、插入字符对宽字符的处理以真 tmux 行为为准，
   先写 probe 用例再实现。

**验收标准：**
- probe 用例：真 tmux 和 gotmux 中 `printf '中文abc漢字\n'` 后
  `capture-pane -p` 输出一致。
- Go 单测：宽字符行尾换行、半覆盖清除。
- 附着客户端里 `ls` 含中文文件名的目录，列对齐正常（手动验证）。

---

## Phase 4：实现 tmux 布局字符串（window_layout）并接入 probe

**为什么：** `#{window_layout}` 是 tmux 对 window 内 pane 几何的精确编码
（例如 `b25d,80x24,0,0[80x12,0,0,0,80x11,0,13,1]`）。让 probe 直接比较
gotmux 和真 tmux 的 layout 字符串，等于强制两者的**分割/resize 几何算法
逐格一致**，比逐条测 resize 边界高效得多。

**步骤：**

1. 在 `internal/model`（layout 相关文件）实现 layout 字符串序列化：
   - 叶子：`WxH,X,Y,paneID`
   - 水平并排：`WxH,X,Y{child,child,...}`
   - 垂直堆叠：`WxH,X,Y[child,child,...]`
   - 前缀 checksum：tmux 的 16 位算法，参考 tmux 源码 `layout-custom.c`
     中 `layout_checksum`（对字符串逐字节：`csum = (csum >> 1) + ((csum & 1) << 15); csum += ch;`，
     以 4 位十六进制小写输出）。
2. 增加 format 变量 `window_layout`。
3. probe 中对每个几何场景（split -h、split -v、嵌套 split、resize-pane、
   select-layout 各内置布局）比较两边 `display-message -p '#{window_layout}'`。
4. 修正 probe 暴露出的所有几何差异（分割时的取整规则、边框占位等，
   一律以真 tmux 结果为准调整 gotmux 的计算）。

**验收标准：**
- probe 中所有布局场景的 `window_layout` 字符串（含 checksum）与真 tmux 完全一致。

---

## Phase 5：copy-mode（查看滚动 + 选择复制）

**为什么：** copy-mode 是 tmux 用户的核心日常操作，依赖 Phase 1（样式）
和 Phase 2（网格历史），所以排在它们之后。

**范围（第一版只做这些）：**

1. 进入/退出：`prefix [` 进入，`q` / `Escape` 退出；
   `copy-mode` 命令（含 `-u` 进入并上翻一页）。
2. 视图滚动：Up/Down/PgUp/PgDn/Home/End，滚动读取 Screen 的 history + 主屏。
3. 状态显示：右上角 `[当前位置/历史总行数]` 指示（照抄 tmux 样式）。
4. 选择与复制：`Space` 开始选择（emacs 表）/ `v`（vi 表可后置），
   移动扩展选区（反显样式渲染），`Enter` 复制选区到 paste buffer 并退出。
   选区取词/取行等高级功能不做。
5. format 变量：`pane_in_mode`、`scroll_position`。
6. `mode-keys` 选项第一版只支持 `emacs`（tmux 默认），`vi` 可以后置。

**验收标准：**
- 附着客户端手动验证：`seq 1 200` 后 `C-b [`，PgUp 能看到历史，
  选择三行按 Enter，`paste-buffer` 粘贴出这三行。
- Go 单测覆盖：滚动边界（顶部/底部）、选区跨行提取文本。
- probe：`copy-mode` 后 `#{pane_in_mode}` 为 1；退出后为 0。

---

## Phase 6：鼠标支持

**范围（第一版）：**

1. `set -g mouse on` 选项。
2. 客户端开启鼠标上报（SGR 1006 模式），把鼠标事件转发给 server。
3. 点击 pane 切换活动 pane；点击 status line 窗口名切换窗口。
4. 滚轮向上在普通模式自动进入 copy-mode 并滚动；copy-mode 中滚轮上下滚动。
5. 拖动 pane 边框调整大小。

**验收标准：** 以上 5 项手动验证 + 能写单测的部分（事件解析、命中判定）写单测。

---

## Phase 7（Phase 1–6 完成后再规划）

候选方向：状态行完整 format/style、`mode-keys vi`、搜索（copy-mode `/`）、
`respawn`/hook 深化、控制模式（`-C`）。到时候先更新本文档再动工。

---

## 附：常用验证命令

```sh
# 全量验证（每个任务结束必跑）
go build ./... && go test ./... && scripts/compat_probe.sh

# 观察真 tmux 行为（隔离 socket，不影响日常会话）
tmux -L probe-manual -f /dev/null new-session -d -s t
tmux -L probe-manual send-keys -t t 'printf "\033[31mred\033[0m\n"' Enter
tmux -L probe-manual capture-pane -e -p -t t
tmux -L probe-manual kill-server

# 观察 gotmux 行为
go run ./cmd/gotmux -L probe-manual new-session -d -s t
```
