# 贡献指南

感谢你愿意改进 MCLauncher TUI —— 一个用 Go 写的 Minecraft 终端启动器。缺陷报告、翻译修正、文档补充和代码同样重要。文档以中文为主，英文 issue 和 PR 同样欢迎。

- [报告问题](#报告问题)
- [功能建议](#功能建议)
- [开发环境](#开发环境)
- [动手之前](#动手之前)
- [项目结构](#项目结构)
- [代码约定](#代码约定)
- [界面与翻译](#界面与翻译)
- [测试](#测试)
- [提交信息与 PR](#提交信息与-pr)
- [发布流程](#发布流程)
- [行为准则](#行为准则)
- [许可](#许可)

## 报告问题

1. 用最新的 [Release](https://github.com/MoonCC233/MCLauncher-TUI/releases) 复现一次，并先搜索现有 [Issues](https://github.com/MoonCC233/MCLauncher-TUI/issues)。
2. 使用 issue 模板提交，不要开空白 issue。模板会自动带上所需字段。
3. 缺陷报告请尽量包含：
   - `mclauncher --version` 的输出（Windows 上是 `mclauncher.exe --version`）；
   - 操作系统、终端与终端字体、界面语言；
   - 涉及的 Minecraft 版本或加载器；
   - 最小复现步骤、期望结果与实际结果；
   - 日志：`<数据目录>/logs/launcher.log`、`<数据目录>/instances/<实例>/logs/latest.log`，或者 Console 页的输出。
4. **不要粘贴 `config.json`**。Microsoft 登录令牌以明文保存在里面，贴设置时只保留相关字段并清掉 `accounts`。
5. 如果问题涉及令牌泄露、任意文件写入、zip 路径穿越等安全问题，请不要开公开 issue。优先使用仓库 Security 页面的「Report a vulnerability」私下报告；该入口不可用时再联系维护者。

## 功能建议

- 先看 README 的[功能列表](https://github.com/MoonCC233/MCLauncher-TUI#功能)与按键说明，有些能力是有意不做的（例如自动写 `%APPDATA%`、托管正版令牌）。
- 描述具体场景，而不是只给结论：「我在 1.19.4 的 Fabric 实例装模组时需要……」，比「加一个模组管理」更容易讨论。
- 涉及按键或界面层级时，写出大致流程和替代方案，并说明现在的绕过办法。

## 开发环境

需要 Go 1.24 或更新版本（以 `go.mod` 为准）。项目不依赖 CGO，也不需要在本地安装 Minecraft。

```bash
git clone https://github.com/MoonCC233/MCLauncher-TUI.git
cd MCLauncher-TUI

# 构建
go build -o mclauncher ./cmd/mclauncher        # Windows: go build -o mclauncher.exe ./cmd/mclauncher

# 运行（默认使用当前目录的 .minecraft 作为数据目录）
./mclauncher

# 用独立的数据目录调试，避免和日常实例混在一起
./mclauncher --data-dir .mclauncher

# 临时切换界面语言
./mclauncher --lang zh-Hans
```

默认的 `.minecraft/` 与 `.mclauncher/` 已在 `.gitignore` 中，不会污染工作区。也可以用环境变量 `MCLAUNCHER_DATA_DIR` 覆盖数据目录，优先级高于设置里的自定义路径。

提交前请本地跑一遍 CI 会执行的检查：

```bash
gofmt -l .        # 必须无输出；CI 在 Linux 任务里强制执行
go vet ./...
go test ./...
go build ./...
```

CI 会在 Ubuntu、Windows 和 macOS 上分别运行 vet / test / build，格式检查只在 Linux 上执行。

## 动手之前

- 小修复直接提 PR 即可；新功能、新的加载器支持、改变配置结构或数据目录布局的改动，建议先开 issue 讨论，避免白做。
- 一个 PR 只做一件事。顺手重构、格式化、重命名请拆成单独的 PR。
- 不要提交构建产物、`.minecraft/`、`config.json`、日志或截图临时文件；`preview-*.ansi`、`preview-*.html` 也在忽略列表中。

## 项目结构

```text
cmd/mclauncher/      程序入口、参数解析、配置引导
internal/tui/        Bubble Tea 模型、页面、弹窗、鼠标命中区域与预览数据
internal/i18n/       界面语言目录、区域设置检测、复数与相对时间
internal/mojang/     版本清单、规则判断、参数替换、依赖解析、版本继承合并、natives 解压
internal/install/    下载、校验、资源索引与安装标记
internal/loader/     Fabric / Quilt / Forge / NeoForge / OptiFine 安装与加载器元数据
internal/mods/       模组扫描、元数据解析、启用/禁用与删除
internal/packs/      资源包、光影与模组的安装、启停和目录扫描
internal/catalog/    Modrinth 搜索与下载元数据
internal/remote/     GitHub Releases、贡献者、issue 与更新公告读取
internal/update/     GitHub Release 检查、镜像下载与启动器替换
internal/launch/     Java 命令行构建、进程启动、输出流
internal/java/       Java 运行时扫描、版本解析、选择与 JDK 下载
internal/gametitle/  Windows 游戏窗口标题修改
internal/config/     配置持久化、账户与实例管理
internal/auth/       离线用户名校验、离线 UUID、Microsoft OAuth / Xbox / Minecraft 登录
internal/paths/      数据目录布局
internal/util/       路径安全、原子 JSON、参数解析、格式化
internal/version/    版本注入
scripts/ansi2html.go 预览 ANSI 转 HTML（生成文档截图用）
```

## 代码约定

- **界面文案必须走 i18n。** 使用 `m.t("...")` / `m.tf("...", args...)`，并在 `internal/i18n/catalog_*.go` 的 14 个目录里同时补齐（en、es、fr、de、zh-Hans、zh-Hant、ja、ar、pt-BR、pt-PT、ru、ko、tr、vi）。`go test ./internal/i18n/` 会检查缺漏，缺一条就会失败。技术日志（下载 URL、Java 输出、错误详情）保持英文，便于排查问题。
- **列宽按显示宽度计算。** 终端里 CJK 占两列，`len()` 会算错。使用 `ansi.StringWidth`、`fitLine`、`padLabel`、`wrapText`；阿拉伯语由布局镜像处理，不要手工反转字符串。
- **配置写入必须原子。** 用 `internal/util` 的 JSON 辅助函数（临时文件 + 重命名重试），不要直接 `os.WriteFile`。新增字段要保证零值可用，旧 `config.json` 不迁移也能读取；只有语义必须改变时才提升 `config.DataVersion` 并写迁移。
- **外部请求要可控。** 带 `context` 超时、使用可替换的 `*http.Client`（测试用 `httptest`）、结果缓存到数据目录，离线时给出可读错误而不是 panic。GitHub 访问要保留镜像回退路径（见 `internal/update`、`internal/remote`）。
- **不要信任下载内容。** 路径拼接用 `internal/util` 的校验函数，解压前检查条目路径，下载校验 SHA-1 / SHA-512。
- **保持跨平台。** 发布矩阵是 windows / linux / darwin × amd64 / arm64，`CGO_ENABLED=0`。平台相关的代码用 build tag 或运行时判断隔离，不要让其他平台编译失败。
- **错误要能被看到。** 用户操作失败时给出 toast 或弹窗，同时 `addLog("launcher", ...)` 记录细节；不要把错误只写进 stderr 或直接吞掉。
- **新增逻辑要有测试。** 非 UI 逻辑直接写表驱动测试；UI 交互尽量通过模型 update 测试覆盖。测试不应访问真实网络或真实数据目录。

## 界面与翻译

新增或修改界面时，除了页面本身，还要同步这些地方：

- `internal/tui/model.go` 的页面常量与按键路由；需要鼠标操作时补 `addHit` 命中区域。
- 帮助弹窗（`?`）与 README 的按键表。
- `internal/tui/preview.go` 加一个 `--preview-screen` 分支，方便截图和人工检查；README 的命令行参数列表也要补上。
- 预览文案跟随 `--lang`：中文用中文演示数据，其他语言用英文，避免截图里混排导致等宽网格错位。

重新生成文档截图（Windows + Edge / Chrome 示例）：

```powershell
go build -o mclauncher.exe ./cmd/mclauncher

# PowerShell 的 > 默认写 UTF-16，用 cmd /c 生成 UTF-8 的 ANSI 预览
cmd /c "mclauncher.exe --preview --lang en --preview-screen settings --width 120 --height 38 > preview-settings.ansi"
go run scripts/ansi2html.go preview-settings.ansi preview-settings.html mclauncher

# 再用浏览器无头模式渲染为 1680×1180 的 PNG，放到 docs/
& "$env:ProgramFiles\Google\Chrome\Application\chrome.exe" --headless=new --disable-gpu `
  --hide-scrollbars --force-device-scale-factor=1 --window-size=1680,1180 `
  --screenshot="$PWD\docs\settings.png" ("file:///" + ((Resolve-Path preview-settings.html).Path -replace '\\','/'))
```

界面有变化时请在 PR 中更新受影响的 `docs/*.png` 和 README 的截图表。

## 测试

- `go test ./...` 是基线；只改某个包时可以先跑 `go test ./internal/tui/` 加快迭代。
- 网络相关测试用 `httptest.Server` 或注入的 `http.Client`，不要请求真实的 GitHub / Mojang / Modrinth。
- 涉及文件系统的测试用 `t.TempDir()`，不要写进仓库目录。
- 修 bug 的 PR 最好带上能复现该 bug 的测试；如果确实无法测试（例如依赖真实终端），在 PR 里写清手动验证步骤。

## 提交信息与 PR

提交信息采用 `type: summary` 的形式，summary 用英文、小写、祈使句，正文说明行为与影响：

```text
fix: stop losing settings when the config write races a file lock

Saving goes through a temporary file and a rename. On Windows that rename fails
whenever something else has the config open for a moment, and the setting was
dropped with only a toast to show for it.

- retry the rename a few times, then write in place as a last resort
- name the file in the error so the toast says which path failed
- cover the overwrite and cleanup paths in tests
```

常用类型：`feat`、`fix`、`docs`、`test`、`chore`、`refactor`、`perf`、`ci`。

PR 流程：

1. Fork 仓库，从 `main` 拉分支，分支名建议 `feat/about-screen`、`fix/config-lock` 这样。
2. 按 `.github/pull_request_template.md` 填写概述、主要改动和测试结果。
3. 涉及界面时附上截图，或贴 `--preview-screen` 的输出；涉及按键时说明新旧操作路径。
4. 确保 `gofmt -l .` 无输出，CI 全绿。评审期间可以继续加提交，不必反复 force-push。
5. 如果使用了 AI 助手生成代码，请在 PR 里说明，并确保自己理解改动、跑过测试。

## 发布流程

发布由维护者执行：先在 `docs/releases/vX.Y.Z.md` 写好更新说明，再推 `v*` 标签，`Release` 工作流会用它作为 Release 正文。完整清单见 [AGENTS.md](AGENTS.md)。没有更新说明的 Release 不会被发布。

## 行为准则

- 对事不对人，讨论代码与行为，不攻击贡献者。
- 不发布他人的令牌、存档、日志中的个人信息或私聊内容。
- 不接受骚扰、歧视或与项目无关的推销内容；维护者可以关闭或删除这类内容。

## 许可

项目使用 [MIT 许可证](LICENSE)。提交 PR 即表示你同意以相同许可证发布你的贡献。
