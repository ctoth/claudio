# Claudio

<!-- hy-mt2-i18n:start -->
[English](./README.md) | **中文** | [日本語](./README_ja.md) | [Español](./README_es.md)
<!-- hy-mt2-i18n:end -->


Claudio 是一个基于钩子机制的编程智能体音频层。它会监听来自 Claude Code、OpenAI Codex CLI、Gemini CLI、Qwen Code 以及 GitHub Copilot CLI 的钩子事件，将相应事件映射为特定的背景音，然后立即播放该声音，而不会让智能体等待音频播放完成。

它可以为工具启动、工具成功、工具失败、提示、通知、补全内容、数据压缩、会话开始以及子代理事件播放不同的声音。由于会解析 Bash 命令，因此 `git commit`、`npm test` 和 `go build` 每个命令都能拥有专属的声音，而无需使用通用的shell声音。

完整文档可从 [docs/index.md](docs/index.md) 查看。

## 安装

```bash
go install claudio.click/cmd/claudio@latest
```

为检测到的智能体安装钩子：

```bash
claudio install
```

`claudio install` 默认使用 `--agent auto --scope global` 参数。它会自动检测 Claude Code、Codex CLI、Gemini CLI、Qwen Code 以及 GitHub Copilot CLI，然后为检测到的工具安装钩子。若要指定特定工具，则需手动指定：

```bash
claudio install --agent claude --scope global
claudio install --agent codex --scope global
claudio install --agent gemini --scope global
claudio install --agent qwen --scope global
claudio install --agent copilot --scope global
```

安装完 Codex 钩子后，在 Codex 中运行 `/hooks` 并信任 Claudio 钩子。如果只需为当前仓库启用钩子，请使用 `--scope project` 而非 `--scope global`。

## 日常命令

```bash
claudio status
claudio volume 0.4
claudio mute
claudio unmute
claudio uninstall --agent all --scope global
```

可选的智能体命令相关资源：

```bash
claudio install-commands --agent claude       # Claude Code 中的 /claudio 命令
claudio install-commands --agent codex        # Codex 中的 $claudio skill 命令
claudio install-commands --agent antigravity  # Antigravity 的技能及 CLI 命令
```

## 音效包

Claudio 提供了平台默认配置，并支持三种自定义音效包格式：

- 位于 `loading/`、`success/`、`error/`、`interactive/`、`completion/` 和 `system/` 下的目录型音效包  
- 将 Claudio 音效键映射到磁盘上任意位置的文件的 JSON 格式音效包  
- 通过 `claudio soundpack add` 安装的托管型 git 音效包

常用命令：

```bash
claudio soundpack list
claudio soundpack init my-pack --from-platform
claudio soundpack validate./my-pack.json
claudio soundpack install./my-pack.json --default
claudio soundpack add gh:owner/repo --name my-pack --default
claudio soundpack update --all
```

支持的音频格式为 WAV、MP3 和 AIFF。有关布局、备用音效链、JSON 映射、验证以及基于 Git 的音效包的相关内容，请参阅 [docs/soundpacks.md](docs/soundpacks.md)。

## 声音追踪

声音追踪功能默认处于开启状态。Claudio 会将已确定的音效以及缺失的备用音效记录到 XDG 缓存目录下的 SQLite 数据库中，随后通过以下方式提供这些数据：

```bash
claudio analyze usage --show-summary --show-chains
claudio analyze missing --preset last-week
```

利用这些报告来决定您的自定义音包接下来应添加哪些声音。

## 远程会话

如果该代理通过 SSH 在远程机器上运行，那台机器通常没有音频设备，因此 Claudio 会保持静音状态。请将本地机器的 PulseAudio 套接字转发过去（Windows 上的 WSLg 已经提供了该功能），并让远程机器指向该套接字。详情请参阅 [docs/remote-audio-ssh.md](docs/remote-audio-ssh.md)。

## 构建与测试

```bash
go build./cmd/claudio
go test./...
```
