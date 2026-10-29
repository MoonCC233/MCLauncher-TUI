# MCLauncher TUI

> 一个 OpenCode 风格的 Minecraft 终端启动器。纯 Go 编写，单文件运行。

[![CI](https://github.com/MoonCC233/MCLauncher-TUI/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/MoonCC233/MCLauncher-TUI/actions/workflows/ci.yml)
[![Release](https://github.com/MoonCC233/MCLauncher-TUI/actions/workflows/release.yml/badge.svg)](https://github.com/MoonCC233/MCLauncher-TUI/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-9ece6a.svg)](LICENSE)

![mclauncher 首页](docs/home.png)

## 功能

- **终端原生 TUI**：Bubble Tea + Lip Gloss，键盘与鼠标操作，窄终端自动收起侧栏；首页使用五线像素字 “MCLAUNCHER” 标志，字距随窗口宽度自适应，窗口过窄时回退为渐变文字。
- **多语言界面**：14 种可选语言，自动跟随系统区域设置；中文、日文、韩文按显示宽度排版，阿拉伯语镜像侧栏、表头与底栏。
- **原版版本安装**：从 Mojang `piston-meta` 获取版本清单，安装 client、libraries、assets、natives 与 log4j 配置。
- **主题与强调色**：内置 OpenCode、Midnight、Forest、Rose、Mono 五种主题，支持自定义十六进制强调色并即时应用到整个界面。
- **共享依赖，独立实例**：libraries 和 assets 全局共享，每个实例拥有独立的游戏目录与存档。
- **离线与正版账户**：支持离线账户与 Microsoft 正版登录（device code 流程），可保存多个账户并快速切换；启动时自动刷新过期的 Microsoft 会话。
- **Java 运行时管理**：自动扫描常见安装位置，也可手动指定；按版本要求选择 Java，并在版本过旧时提示。
- **实例参数覆盖**：全局内存、实例内存、额外 JVM 参数；窗口尺寸提供从 854×480 到 4K 的常用预设，也可手动输入自定义分辨率。
- **可靠的下载**：多线程下载、SHA-1 校验、原子写入配置文件。
- **控制台**：实时显示游戏 stdout/stderr，支持滚动、清屏、停止游戏和打开日志目录。
- **后台任务与下载记录**：安装和游戏启动可发送到后台，在 `Downloads` 页面查看进度、重开进度弹窗或取消任务；最近 20 条任务会保留在历史记录中，可用 `↑` / `↓`、`pgup` / `pgdown` 翻看。
- **版本分类**：正式版、快照、远古版（Beta / Alpha）与愚人节版独立筛选；正式版和快照分类会自动排除愚人节版本；旧版 assets 的 virtual / map_to_resources 索引也有处理。
- **便携数据目录**：默认把游戏数据放在当前目录的 `.minecraft`，可在设置中改到任意路径。

## 界面预览

| 版本管理 | 实例管理 |
| --- | --- |
| ![版本](docs/versions.png) | ![实例](docs/instances.png) |

| 控制台 | 快捷键帮助 |
| --- | --- |
| ![控制台](docs/console.png) | ![帮助](docs/help.png) |

<details>
<summary>更多截图</summary>

| 设置 | 账户 |
| --- | --- |
| ![设置](docs/settings.png) | ![账户](docs/accounts.png) |

| 新建实例 | 安装进度 |
| --- | --- |
| ![表单](docs/form.png) | ![进度](docs/progress.png) |

| 账户类型 | 分辨率预设 |
| --- | --- |
| ![账户类型](docs/account-type.png) | ![分辨率](docs/resolution.png) |

</details>

## 安装

### 下载预编译版本

前往 [Releases](https://github.com/MoonCC233/MCLauncher-TUI/releases) 下载对应平台压缩包：

- Windows：`mclauncher_<version>_windows_amd64.zip`（ARM64 也有对应包）
- Linux：`mclauncher_<version>_linux_amd64.tar.gz`
- macOS：`mclauncher_<version>_darwin_amd64.tar.gz` / `arm64`

解压后直接运行：

```bash
# Linux / macOS
chmod +x mclauncher
./mclauncher
```

Windows 建议在 **Windows Terminal** 中运行 `mclauncher.exe`。传统 `cmd.exe` 对 ANSI 颜色和制表符的渲染可能不完整。

### 从源码构建

需要 Go **1.24.2** 或更高版本：

```bash
git clone https://github.com/MoonCC233/MCLauncher-TUI.git
cd MCLauncher-TUI
go build -o mclauncher ./cmd/mclauncher
./mclauncher
```

Windows 下输出文件为 `mclauncher.exe`。

### 前置条件

- **Java**：启动器不会自动下载 Java。请先安装对应版本所需的 JDK/JRE：
  - Minecraft 1.20.5+ / 1.21.x：通常需要 Java 21
  - Minecraft 1.18–1.20.4：通常需要 Java 17
  - 更旧的版本：通常需要 Java 8
- **网络**：首次加载版本清单和安装版本时需要访问 Mojang 的 `piston-meta`、`piston-data`、`resources`、`libraries` 服务。
- 现代终端：支持真彩色与完整 Unicode 制表符（Windows Terminal、iTerm2、Kitty、WezTerm 等）。

## 使用流程

1. **添加账户**  
   进入 `Accounts`，按 `a` 选择账户类型：  
   - **离线账户**：输入 3–16 位字母、数字或下划线用户名，按 Java 版离线 UUID 规则生成身份。  
   - **Microsoft 正版账户**：选择 `Sign in with Microsoft`。启动器会申请设备代码并自动打开 `https://www.microsoft.com/link`，在浏览器中输入弹窗里的代码并完成登录；也可以按 `c` 复制代码、按 `r` 重新申请。登录成功后会自动获取 Minecraft 档案、UUID 与 XUID。  
   多个账户可共存，按 `enter` 设为当前账户。

2. **安装版本**  
   进入 `Versions`，等待版本清单加载。使用 `/` 搜索，`t` 在“全部 / 正式版 / 快照 / 远古版 / 愚人节版”之间循环切换；愚人节版本会显示紫色 `April Fools` 标签。  
   - `enter`：安装（如果尚未安装）并打开“新建实例”表单。  
   - `i`：仅安装，不创建实例。  
   - `r`：重新拉取 Mojang 版本清单。

3. **创建并启动实例**  
   在表单中填写实例名、可选内存上限，按 `enter` 创建。回到 `Instances` 或 `Home`，按 `enter` 启动。

4. **查看日志 / 停止游戏**  
   进入 `Console` 查看实时输出。按 `x` 停止游戏进程，按 `o` 打开日志目录。

5. **后台任务与下载记录**  
   安装或启动时按 `b` 将任务留在后台，按 `d` 直接打开 `Downloads`。在 `Downloads` 中按 `enter` 重开进度弹窗，按 `x` 取消任务；完成、失败和取消的任务会保留在历史记录中。

6. **调整设置**  
   在 `Settings` 中调整全局内存、Java 运行时、JVM 参数、窗口尺寸、下载线程数、界面语言、主题、数据目录与 Microsoft 客户端 ID。进入某一行按 `enter` 编辑，内存和线程数也可用 `←` / `→` 或 `-` / `+` 调整。  
   - **窗口尺寸**：打开常用分辨率预设（854×480、1280×720、1366×768、1600×900、1920×1080、2560×1440、3840×2160），选择 `Custom…` 可手动输入任意 `宽x高`。  
   - **主题**：打开主题选择器，选择预设或 `Custom accent…` 输入十六进制颜色；`Reset accent` 恢复主题默认强调色。  
   - **数据目录**：留空使用默认 `.minecraft`；也可以输入绝对路径或 `~/...`。切换后会立即重新扫描已安装版本，并保留原有配置文件位置。  
   - **Microsoft 客户端 ID**：留空使用内置公开客户端；只有使用自己注册的 Azure 应用时才需要填写 GUID。自定义 Entra 应用需要获得 Mojang 批准，否则 Minecraft 服务可能返回 403。  
   - **语言**：打开语言选择器，第一项“跟随系统”会跟随操作系统区域设置。

## 快捷键

| 键 | 作用 |
| --- | --- |
| `tab` / `shift+tab` | 切换页面 |
| `1` … `7` | 直接跳转到指定页面 |
| `↑` `↓` / `k` `j` | 移动光标 |
| `pgup` / `pgdown` | 翻页 |
| `home` / `end` | 跳到首项 / 末项 |
| `enter` | 打开、启动或编辑 |
| `esc` | 返回 Home / 关闭弹窗 |
| `?` | 打开快捷键帮助 |
| `ctrl+c` | 退出启动器 |

页面专用按键：

| 页面 | 键与作用 |
| --- | --- |
| Instances | `n` 新建实例 · `e` 编辑 · `d` 删除（保留游戏文件）· `o` 打开实例目录 · `x` 停止游戏 |
| Versions | `/` 筛选 · `t` 切换类型（全部 / 正式版 / 快照 / 远古版 / 愚人节版）· `enter` 安装并创建实例 · `i` 仅安装 · `d` 卸载已安装版本 · `r` 刷新 · `o` 打开版本目录 |
| Accounts | `a` / `n` 添加账户（离线或 Microsoft）· `enter` 设为当前 · `d` 移除账户 |
| Settings | `enter` 编辑 · `s` / `r` 重新扫描 Java · `←` `→` / `-` `+` 调整数值 · `o` 打开数据目录 |
| Console | `x` 停止游戏 · `c` 清屏 · `o` 打开日志目录 · `G` / `end` 跟随最新日志 |
| Downloads | `enter` 重开当前进度 · `x` 取消当前任务 · `↑` `↓` / `pgup` `pgdown` 翻看历史 |

### 鼠标操作

鼠标行为参考 OpenCode TUI：

- **侧栏、筛选标签、底栏快捷键、弹窗按钮**：单击立即生效。
- **列表行**：第一次单击选中，450 ms 内再次单击才会打开 / 启动 / 编辑，避免误触。
- **滚轮**：滚动实例、版本、账户、设置、控制台、帮助和选择器列表。
- **弹窗**：单击弹窗外部关闭（进度弹窗除外）；表单字段可直接单击聚焦。

键盘操作始终可用，鼠标只是补充。

### 界面语言

支持以下 14 种界面语言：

| 语言 | 代码 | 语言 | 代码 |
| --- | --- | --- | --- |
| English | `en` | Português (Brasil) | `pt-BR` |
| Español | `es` | Português (Portugal) | `pt-PT` |
| Français | `fr` | Русский | `ru` |
| Deutsch | `de` | 한국어 | `ko` |
| 简体中文 | `zh-Hans` | Türkçe | `tr` |
| 繁體中文 | `zh-Hant` | Tiếng Việt | `vi` |
| 日本語 | `ja` | العربية | `ar` |

首次启动时自动检测系统语言；配置中的 `settings.language` 为空表示“跟随系统”。也可以在 `Settings → Language` 中切换，或用命令行参数临时指定：

```bash
./mclauncher --lang zh-Hans
```

`pt-PT` 缺少的条目会回退到 `pt-BR`，再回退到英文。阿拉伯语会镜像侧栏、页头、底栏和表格列；具体字形连写取决于终端字体与 BiDi 支持。技术日志（下载地址、Java 输出、错误详情）保持英文，便于排查问题。

## 数据目录

默认使用**便携模式**：所有游戏数据放在启动器当前工作目录的 `.minecraft` 中，启动器不会写入 `%APPDATA%` 或用户主目录。

```text
<当前目录>/.minecraft/
├── config.json                 # 账户、实例与设置（固定保存在启动根目录）
├── versions/<id>/              # 版本 JSON、client jar、natives、安装标记
├── libraries/                  # 全局共享依赖
├── assets/
│   ├── indexes/
│   ├── objects/
│   ├── virtual/                # 旧版本虚拟资源
│   └── log_configs/
├── instances/<id>/             # 每个实例独立的游戏目录（存档、模组、截图等）
├── logs/
└── cache/
```

可以在 `Settings → Data directory` 中把**游戏数据**改到其他位置，例如 `D:\MCLauncher\.minecraft` 或 `~/mc-data`。切换后启动器会立即使用新目录并在下次启动时继续生效；`config.json` 仍留在原来的启动根目录，避免改路径后丢失设置。清空该设置即可恢复默认 `.minecraft`。

也可以通过环境变量或命令行参数覆盖启动根目录：

```bash
MCLAUNCHER_DATA_DIR=D:\MCLauncher ./mclauncher
./mclauncher --data-dir ./portable-data
```

`--data-dir` 和 `MCLAUNCHER_DATA_DIR` 的优先级高于设置中的自定义路径；使用它们时，`config.json` 和游戏数据都会落在指定目录。

从旧版本升级时，`%APPDATA%\mclauncher`（或其他旧数据目录）不会自动迁移。用 `--data-dir` / `MCLAUNCHER_DATA_DIR` 指向旧目录即可直接沿用旧配置、版本和存档；也可以把旧 `config.json` 复制到当前的 `.minecraft`，再在设置中把游戏数据指向旧目录。

> Microsoft 登录令牌保存在 `config.json` 中（明文 JSON）。请勿把该文件提交到公开仓库或分享给他人；移除账户会同时删除对应令牌。

删除实例只会从启动器配置中移除条目，`instances/<id>/` 中的文件会保留。

## 命令行参数

```text
--data-dir <path>       覆盖数据目录
--lang <code>           指定界面语言（如 zh-Hans、pt-BR、ar）
--version               打印版本并退出
--preview               渲染一帧静态界面到 stdout
--preview-screen <name> home | instances | versions | accounts | settings | console | downloads | help | form | progress | theme | language | java | account-type | microsoft | microsoft-error | resolution
--width <n>             预览宽度（默认 120）
--height <n>            预览高度（默认 38）
```

`--preview` 不需要真实终端，适合在 CI 或文档构建中生成界面截图。

## 开发

```bash
gofmt -w .
go vet ./...
go test ./...
go build ./...
```

项目结构：

```text
cmd/mclauncher/         程序入口、参数解析、配置引导
internal/tui/           Bubble Tea 模型、页面、弹窗、鼠标命中区域与预览数据
internal/i18n/          界面语言目录、区域设置检测、复数与相对时间
internal/mojang/        版本清单、规则判断、参数替换、依赖解析、natives 解压
internal/install/       下载、校验、资源索引与安装标记
internal/launch/        Java 命令行构建、进程启动、输出流
internal/java/          Java 运行时扫描、版本解析与选择
internal/config/        配置持久化、账户与实例管理
internal/auth/          离线用户名校验、离线 UUID、Microsoft OAuth / Xbox / Minecraft 登录
internal/paths/         数据目录布局
internal/util/          路径安全、原子 JSON、参数解析、格式化
internal/version/       版本注入
scripts/ansi2html.go    预览 ANSI 转 HTML（生成文档截图用）
```

生成文档截图（Windows + Edge 示例）：

```powershell
go build -o mclauncher.exe ./cmd/mclauncher
mclauncher.exe --preview --preview-screen home --width 120 --height 38 > preview-home.ansi
# 生成中文界面预览
mclauncher.exe --preview --lang zh-Hans --preview-screen settings --width 120 --height 38 > preview-settings-zh.ansi
go run scripts/ansi2html.go preview-home.ansi preview-home.html mclauncher
# 再用浏览器无头模式将 preview-home.html 渲染为 PNG
```

## 发布

发布说明写在 `docs/releases/vX.Y.Z.md`，提交并推送 `main` 后再推送 `v*` 标签：

```bash
# 1. 编写并提交发布说明
git add docs/releases/v0.2.0.md
git commit -m "docs: add v0.2.0 release notes"
git push

# 2. 打标签并触发发布
git tag -a v0.2.0 -m "v0.2.0"
git push origin v0.2.0
```

Release 工作流会读取对应标签的 `docs/releases/<tag>.md`，通过 GoReleaser 的 `--release-notes` 写入 Release 页面；说明文件缺失时工作流会直接失败，确保每个版本都有更新详情。

发布流水线会构建以下目标，并生成校验和：

- `windows/amd64`、`windows/arm64`
- `linux/amd64`、`linux/arm64`
- `darwin/amd64`、`darwin/arm64`

压缩包内含 `mclauncher` 可执行文件、`README.md` 和 `LICENSE`。

## 已知限制

- **Microsoft 登录依赖网络与浏览器**：设备代码流程需要访问 Microsoft 登录页；部分账户首次使用前可能需要先在 `xbox.com` 创建 Xbox 档案。自定义 Entra 客户端需要 Mojang 审批，内置客户端开箱即用。
- **仅支持 Minecraft 原版**：尚未集成 Forge、Fabric、Quilt、OptiFine 等模组加载器。
- **不管理 Java 安装**：只检测和选择已安装的 Java；不会下载 JDK。
- **退出启动器不会终止游戏**：启动器退出后游戏继续运行，但后续 stdout/stderr 输出可能丢失。需要停止游戏时请先在 Console 中按 `x`。
- **无自动更新**：请关注 Releases 页面获取新版本。
- **下载历史只保留在当前会话**：`Downloads` 的最近 20 条任务记录保存在内存中，退出启动器后清空。
- **阿拉伯语显示取决于终端**：启动器会镜像布局，但阿拉伯文字形连写和 BiDi 排序由终端与字体决定；Windows Terminal 的显示效果通常最好。
- **预编译二进制未签名**：macOS 首次运行可能需要 `xattr -d com.apple.quarantine ./mclauncher`。

## 致谢

界面风格受 [OpenCode](https://github.com/sst/opencode) 启发；TUI 基于 [Bubble Tea](https://github.com/charmbracelet/bubbletea)、[Bubbles](https://github.com/charmbracelet/bubbles) 和 [Lip Gloss](https://github.com/charmbracelet/lipgloss)。

本项目与 Mojang Studios / Microsoft 无关联。使用 Minecraft 客户端及资源时请遵守 [Minecraft EULA](https://www.minecraft.net/eula)。

## License

[MIT](LICENSE) © 2026 MoonCC233
