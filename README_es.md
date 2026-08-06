# Claudio

<!-- hy-mt2-i18n:start -->
[English](./README.md) | [中文](./README_zh-CN.md) | [日本語](./README_ja.md) | **Español**
<!-- hy-mt2-i18n:end -->


Claudio es una capa de audio basada en ganchos para agentes de programación. Escucha los eventos de ganchos provenientes de Claude Code, OpenAI Codex CLI, Gemini CLI, Qwen Code y GitHub Copilot CLI, asigna cada evento a un sonido adecuado al contexto y lo reproduce sin que el agente tenga que esperar a que finalice la reproducción.

Puede reproducir sonidos diferentes para el inicio de herramientas, los éxitos y fallas de las mismas, las solicitudes, notificaciones, completaciones, compactación, inicio de sesiones y eventos de subagente. Se analizan los comandos Bash, por lo que `git commit`, `npm test` y `go build` pueden tener cada uno su propio sonido en lugar de compartir un sonido genérico de la shell.

La documentación completa está disponible en [docs/index.md](docs/index.md).

## Instalación

```bash
go install claudio.click/cmd/claudio@latest
```

Instalar ganchos para los agentes detectados:

```bash
claudio install
```

Por defecto, `claudio install` utiliza `--agent auto --scope global`. Detecta Claude Code, Codex CLI, Gemini CLI, Qwen Code y GitHub Copilot CLI, y luego instala los ganchos para los agentes que encuentra. Para forzar un solo agente:

```bash
claudio install --agent claude --scope global
claudio install --agent codex --scope global
claudio install --agent gemini --scope global
claudio install --agent qwen --scope global
claudio install --agent copilot --scope global
```

Después de instalar el gancho de Codex, ejecute `/hooks` en Codex y confíe en el gancho de Claudio. Utilice `--scope project` en lugar de `--scope global` si desea que los ganchos estén disponibles solo para el repositorio actual.

## Comandos diarios

```bash
claudio status
claudio volume 0.4
claudio mute
claudio unmute
claudio uninstall --agent all --scope global
```

Artefactos opcionales de comandos del agente:

```bash
claudio install-commands --agent claude       # /claudio en Claude Code
claudio install-commands --agent codex        # $claudio skill en Codex
claudio install-commands --agent antigravity  # Habilidad Antigravity y comando de la CLI
```

## Soundpacks

Claudio incluye las configuraciones predeterminadas de la plataforma y admite tres tipos de soundpack personalizados:

- Paquetes de sonido en directorios ubicados en `loading/`, `success/`, `error/`, `interactive/`, `completion/` y `system/`
- Paquetes de sonido en formato JSON que asocian las claves de sonido de Claudio con archivos en cualquier ubicación del disco
- Paquetes de sonido gestionados por Git instalados mediante `claudio soundpack add`

Comandos útiles:

```bash
claudio soundpack list
claudio soundpack init my-pack --from-platform
claudio soundpack validate./my-pack.json
claudio soundpack install./my-pack.json --default
claudio soundpack add gh:owner/repo --name my-pack --default
claudio soundpack update --all
```

Los formatos de audio admitidos son WAV, MP3 y AIFF. Consulte [docs/soundpacks.md](docs/soundpacks.md) para obtener información sobre la estructura, las cadenas de respaldo, las asignaciones JSON, la validación y los soundpacks basados en Git.

## Seguimiento

El seguimiento de sonidos está habilitado por defecto. Claudio registra los sonidos resueltos y las opciones de respaldo faltantes en una base de datos SQLite ubicada en el directorio de caché XDG, y luego expone esos datos a través de:

```bash
claudio analyze usage --show-summary --show-chains
claudio analyze missing --preset last-week
```

Utilice estos informes para decidir qué sonidos debería añadir su paquete personal a continuación.

## Sesiones remotas

Si el agente se ejecuta en una máquina remota a través de SSH, esa máquina generalmente no cuenta con dispositivo de audio y Claudio permanecerá en silencio. Redirija un socket PulseAudio desde su máquina local (WSLg en Windows ya lo proporciona) e indique a la máquina remota que utilice ese socket. Consulte [docs/remote-audio-ssh.md](docs/remote-audio-ssh.md).

## Compilación y pruebas

```bash
go build./cmd/claudio
go test./...
```
