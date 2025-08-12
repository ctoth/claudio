---
layout: default
title: "Troubleshooting"
---

# Troubleshooting

Common issues and solutions for Claudio installation and usage.

## Installation Issues

### "claudio: command not found"

**Problem:** The claudio binary isn't found in your PATH.

**Solutions:**

1. **Check Go installation:**
   ```bash
   go version
   ```
   If Go isn't installed, install it from [golang.org](https://golang.org).

2. **Check GOPATH/GOBIN:**
   ```bash
   go env GOPATH
   go env GOBIN
   ```
   The binary is installed to `$GOPATH/bin` (usually `~/go/bin`).

3. **Add Go bin to PATH:**
   ```bash
   # Add to ~/.bashrc or ~/.zshrc
   export PATH=$PATH:$(go env GOPATH)/bin
   
   # Reload shell
   source ~/.bashrc
   ```

4. **Verify installation:**
   ```bash
   ls -la $(go env GOPATH)/bin/claudio
   claudio --help
   ```

### "No Claude Code settings found"

**Problem:** Claudio can't locate Claude Code settings files.

**Solutions:**

1. **Verify Claude Code is installed:**
   - Make sure Claude Code has been run at least once
   - Check that settings directory exists

2. **Check settings locations:**
   ```bash
   # User settings (most common)
   ls -la ~/.config/claude-code/settings.json        # Linux
   ls -la ~/Library/Application\ Support/claude-code/settings.json  # macOS
   
   # Project settings
   ls -la .claude-code/settings.json
   ```

3. **Try different scope:**
   ```bash
   # If user scope fails, try project scope
   claudio install --scope project
   
   # If project scope fails, try user scope
   claudio install --scope user
   ```

4. **Create settings manually:**
   ```bash
   # Create settings directory
   mkdir -p ~/.config/claude-code
   
   # Create minimal settings file
   echo '{}' > ~/.config/claude-code/settings.json
   
   # Try installation again
   claudio install
   ```

### Installation Permission Errors

**Problem:** Permission denied when installing hooks.

**Solutions:**

1. **Check file permissions:**
   ```bash
   ls -la ~/.config/claude-code/settings.json
   ```

2. **Fix permissions:**
   ```bash
   chmod 644 ~/.config/claude-code/settings.json
   chmod 755 ~/.config/claude-code
   ```

3. **Run with correct user:**
   ```bash
   # Don't use sudo for user-scope installation
   claudio install
   ```

## Audio Issues

### No Sound Output

**Problem:** Claudio runs but produces no audio.

**Solutions:**

1. **Check system audio:**
   ```bash
   # Test system audio works
   speaker-test -t sine -f 1000 -l 1  # Linux
   afplay /System/Library/Sounds/Ping.aiff  # macOS
   ```

2. **Check Claudio configuration:**
   ```bash
   # Verify Claudio is enabled
   export CLAUDIO_LOG_LEVEL=debug
   echo '{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"test"}}' | claudio
   ```

3. **Check volume settings:**
   ```bash
   # Test with higher volume
   echo '...' | claudio --volume 1.0
   
   # Check configuration volume
   export CLAUDIO_VOLUME=0.8
   ```

4. **Verify soundpack exists:**
   ```bash
   # Check default soundpack
   ls -la /usr/local/share/claudio/default/
   ls -la ~/.local/share/claudio/default/
   
   # Test with explicit soundpack
   CLAUDIO_SOUNDPACK=default echo '...' | claudio
   ```

### Audio Crackling or Distortion

**Problem:** Sound plays but has crackling or distortion.

**Solutions:**

1. **Lower volume:**
   ```bash
   # Try lower volume levels
   echo '...' | claudio --volume 0.3
   echo '...' | claudio --volume 0.5
   ```

2. **Check sound file quality:**
   ```bash
   # Verify sound files aren't corrupted
   file /usr/local/share/claudio/default/default.wav
   
   # Play sound file directly
   aplay /usr/local/share/claudio/default/default.wav  # Linux
   afplay /usr/local/share/claudio/default/default.wav  # macOS
   ```

3. **Update audio configuration:**
   ```json
   {
     "volume": 0.3,
     "log_level": "debug"
   }
   ```

### "No audio device available"

**Problem:** Claudio reports no audio devices found.

**Solutions:**

1. **Check audio system:**
   ```bash
   # Linux: Check ALSA/PulseAudio
   aplay -l
   pulseaudio --check
   
   # macOS: Check system preferences
   system_profiler SPAudioDataType
   ```

2. **Try different audio backends:**
   ```bash
   # Run with debug to see audio system info
   export CLAUDIO_LOG_LEVEL=debug
   echo '...' | claudio
   ```

3. **Restart audio services:**
   ```bash
   # Linux
   sudo systemctl restart pulseaudio
   sudo systemctl restart alsa-state
   
   # macOS
   sudo killall coreaudiod
   ```

## Claude Code Integration Issues

### Hooks Not Triggering

**Problem:** Claudio installed but no sounds during Claude Code usage.

**Solutions:**

1. **Verify hook installation:**
   ```bash
   # Check what's installed
   claudio install --dry-run
   
   # Print current configuration
   claudio install --print
   ```

2. **Check Claude Code settings:**
   ```bash
   # Examine settings file directly
   cat ~/.config/claude-code/settings.json
   
   # Look for hooks section:
   # "hooks": {
   #   "PreToolUse": "claudio",
   #   "PostToolUse": "claudio",
   #   "UserPromptSubmit": "claudio"
   # }
   ```

3. **Reinstall hooks:**
   ```bash
   # Reinstall hooks (overwrites existing Claudio hooks)
   claudio install
   ```

4. **Test hook execution manually:**
   ```bash
   # Test PostToolUse hook
   echo '{"session_id":"test","transcript_path":"/test","cwd":"/test","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"test","stderr":"","interrupted":false}}' | claudio
   ```

### Wrong Sounds Playing

**Problem:** Sounds play but aren't appropriate for the tool being used.

**Solutions:**

1. **Enable debug logging:**
   ```bash
   export CLAUDIO_LOG_LEVEL=debug
   # Use Claude Code normally to see sound selection process
   ```

2. **Check soundpack contents:**
   ```bash
   # List available sounds
   find /usr/local/share/claudio/default -name "*.wav" | sort
   
   # Verify tool-specific sounds exist
   ls -la /usr/local/share/claudio/default/success/git-*
   ls -la /usr/local/share/claudio/default/success/npm-*
   ```

3. **Test fallback chain:**
   ```bash
   # Create test hook JSON with specific tool
   echo '{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"git status"},"tool_response":{"stdout":"clean","stderr":""}}' | claudio
   ```

## Configuration Issues

### Configuration Not Loading

**Problem:** Configuration changes don't take effect.

**Solutions:**

1. **Check configuration file location:**
   ```bash
   # Find where Claudio looks for config
   export CLAUDIO_LOG_LEVEL=debug
   echo '...' | claudio 2>&1 | grep -i config
   ```

2. **Verify JSON syntax:**
   ```bash
   # Validate configuration file
   cat /etc/xdg/claudio/config.json | jq .
   cat ~/.config/claudio/config.json | jq .
   ```

3. **Check file permissions:**
   ```bash
   ls -la /etc/xdg/claudio/config.json
   ls -la ~/.config/claudio/config.json
   ```

4. **Test with environment variables:**
   ```bash
   # Override configuration temporarily
   CLAUDIO_VOLUME=0.1 CLAUDIO_LOG_LEVEL=debug echo '...' | claudio
   ```

### Environment Variables Not Working

**Problem:** Environment variables don't override configuration.

**Solutions:**

1. **Check variable names:**
   ```bash
   # Correct names (case-sensitive)
   export CLAUDIO_VOLUME=0.5
   export CLAUDIO_ENABLED=true
   export CLAUDIO_SOUNDPACK=default
   export CLAUDIO_LOG_LEVEL=debug
   ```

2. **Verify variables are set:**
   ```bash
   env | grep CLAUDIO
   ```

3. **Test individual variables:**
   ```bash
   # Test each variable separately
   CLAUDIO_VOLUME=0.1 echo '...' | claudio
   CLAUDIO_ENABLED=false echo '...' | claudio
   ```

## Soundpack Issues

### Custom Soundpack Not Found

**Problem:** Custom soundpack can't be loaded.

**Solutions:**

1. **Check soundpack structure:**
   ```bash
   # Verify directory structure
   find ~/.local/share/claudio/my-pack -type f
   
   # Must contain at least:
   # default.wav
   # success/success.wav
   # error/error.wav
   # loading/loading.wav
   ```

2. **Check search paths:**
   ```bash
   # Debug soundpack discovery
   export CLAUDIO_LOG_LEVEL=debug
   CLAUDIO_SOUNDPACK=my-pack echo '...' | claudio
   ```

3. **Test with absolute path:**
   ```json
   {
     "default_soundpack": "my-pack",
     "soundpack_paths": ["/full/path/to/soundpack/directory"]
   }
   ```

### Sound Files Not Playing

**Problem:** Soundpack found but individual sounds don't play.

**Solutions:**

1. **Check audio format:**
   ```bash
   # Verify file format
   file ~/.local/share/claudio/my-pack/default.wav
   
   # Should be: WAVE audio or MP3
   ```

2. **Test files directly:**
   ```bash
   # Play sound file with system player
   aplay ~/.local/share/claudio/my-pack/default.wav  # Linux
   afplay ~/.local/share/claudio/my-pack/default.wav  # macOS
   ```

3. **Check file permissions:**
   ```bash
   ls -la ~/.local/share/claudio/my-pack/*.wav
   # Should be readable (644 or similar)
   ```

## Debug Information Collection

### Enabling Debug Mode

For comprehensive troubleshooting, enable debug logging:

```bash
export CLAUDIO_LOG_LEVEL=debug
```

### Collecting System Information

```bash
# System information
uname -a
go version

# Audio system information
# Linux
aplay -l
pulseaudio --dump-conf

# macOS
system_profiler SPAudioDataType

# Claudio configuration
claudio install --print
find /usr/local/share/claudio -name "*.wav" | head -10
env | grep CLAUDIO
```

### Testing Hook Execution

```bash
# Test complete hook pipeline
export CLAUDIO_LOG_LEVEL=debug
echo '{"session_id":"debug","transcript_path":"/tmp/test","cwd":"/tmp","hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"echo test"},"tool_response":{"stdout":"test\n","stderr":"","interrupted":false}}' | claudio
```

## Getting Help

If these solutions don't resolve your issue:

1. **Check existing issues:** [GitHub Issues](https://github.com/ctoth/claudio/issues)
2. **Create new issue:** Include debug output and system information
3. **Include in issue report:**
   - Operating system and version
   - Go version
   - Output of debug mode
   - Claude Code version
   - Steps to reproduce

## See Also

- **[Installation Guide](/installation)** - Proper setup procedures
- **[Configuration](/configuration)** - Configuration file format and options
- **[CLI Reference](/cli-reference)** - Command-line usage details