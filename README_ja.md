# Claudio

<!-- hy-mt2-i18n:start -->
[English](./README.md) | [中文](./README_zh-CN.md) | **日本語** | [Español](./README_es.md)
<!-- hy-mt2-i18n:end -->


Claudioは、コーディングエージェント向けのフック駆動型オーディオレイヤーです。Claude Code、OpenAI Codex CLI、Gemini CLI、Qwen Code、GitHub Copilot CLIからのフックイベントを受信し、そのイベントに応じた状況に合った音声にマッピングして再生します。そのため、エージェントは音声の再生を待つことなく即座に処理を続行できます。

ツールの起動、成功、失敗、プロンプト、通知、補完結果、データ圧縮、セッション開始、およびサブエージェントイベントに応じて異なる音声を再生できます。Bashコマンドも解析されるため、`git commit`、`npm test`、`go build` それぞれが共通のシェル音声を共有するのではなく、独自の音声を持つことが可能です。

全てのドキュメントは [docs/index.md](docs/index.md) からご覧いただけます。

## インストール

```bash
go install claudio.click/cmd/claudio@latest
```

検出されたエージェント用のフックをインストールする：

```bash
claudio install
```

`claudio install`はデフォルトで`--agent auto --scope global`を使用します。これによりClaude Code、Codex CLI、Gemini CLI、Qwen Code、GitHub Copilot CLIを検出し、見つかったエージェント用のフックをインストールします。特定のエージェントのみに強制したい場合は：

```bash
claudio install --agent claude --scope global
claudio install --agent codex --scope global
claudio install --agent gemini --scope global
claudio install --agent qwen --scope global
claudio install --agent copilot --scope global
```

Codexのフックがインストールされたら、Codex内で `/hooks` を実行し、Claudioのフックを信頼してください。現在のリポジトリ内でのみフックを利用したい場合は、`--scope global` の代わりに `--scope project` を使用してください。

## 日常使用コマンド

```bash
claudio status
claudio volume 0.4
claudio mute
claudio unmute
claudio uninstall --agent all --scope global
```

オプションのエージェントコマンドアーティファクト：

```bash
claudio install-commands --agent claude       # Claude Code 内の /claudio
claudio install-commands --agent codex        # Codex 内の $claudio skill
claudio install-commands --agent antigravity  # Antigravity スキルおよび CLI コマンド
```

## サウンドパック

Claudioはプラットフォーム固有のデフォルト設定を提供するほか、3種類のカスタムサウンドパック形式もサポートしています：

- `loading/`、`success/`、`error/`、`interactive/`、`completion/`、`system/` 下のディレクトリ形式のサウンドパック  
- Claudioのサウンドキーをディスク上の任意のファイルにマッピングするJSON形式のサウンドパック  
- `claudio soundpack add` を使用してインストールされる管理型gitサウンドパック

便利なコマンド：

```bash
claudio soundpack list
claudio soundpack init my-pack --from-platform
claudio soundpack validate./my-pack.json
claudio soundpack install./my-pack.json --default
claudio soundpack add gh:owner/repo --name my-pack --default
claudio soundpack update --all
```

サポートされているオーディオ形式はWAV、MP3、AIFFです。レイアウト、フォールバックチェーン、JSONマッピング、検証、およびgitで管理されるサウンドパックについては、
[docs/soundpacks.md](docs/soundpacks.md)をご覧ください。

## トラッキング

サウンドのトラッキングはデフォルトで有効になっています。Claudioは解決されたサウンドや不足しているフォールバック候補をXDGキャッシュディレクトリ内のSQLiteデータベースに記録し、そのデータを以下の手段で公開します：

```bash
claudio analyze usage --show-summary --show-chains
claudio analyze missing --preset last-week
```

これらのレポートを利用して、カスタムパックに次に追加すべきサウンドを決定しましょう。

## リモートセッション

エージェントがSSHを経由してリモートマシン上で実行されている場合、そのマシンには通常オーディオデバイスがなく、Claudioは音声を出しません。ローカルマシンからPulseAudioソケットを転送し（WindowsのWSLgでは既に提供されています）、リモートマシンがそのソケットを参照するように設定してください。詳細は[docs/remote-audio-ssh.md](docs/remote-audio-ssh.md)をご覧ください。

## ビルドとテスト

```bash
go build./cmd/claudio
go test./...
```
