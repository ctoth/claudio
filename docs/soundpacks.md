---
layout: default
title: "Soundpacks"
---

# Soundpacks

Soundpacks are Claudio's system for organizing and playing contextual audio. Understanding the soundpack structure and fallback system is key to customizing your audio experience.

## Soundpack Directory Structure

A soundpack is a directory containing organized sound files. Here's the standard structure:

```
default/                    # Soundpack directory
├── loading/               # PreToolUse event sounds
│   ├── git-thinking.wav      # Git operations starting
│   ├── npm-thinking.wav      # NPM operations starting
│   ├── bash-thinking.wav     # General bash operations
│   └── loading.wav           # Generic loading sound
├── success/               # PostToolUse success sounds
│   ├── git-commit-success.wav  # Specific: git commit succeeded
│   ├── git-success.wav         # Tool: any git operation succeeded
│   ├── bash-success.wav        # Operation: any bash command succeeded
│   └── success.wav             # Category: any successful operation
├── error/                 # PostToolUse error sounds
│   ├── git-error.wav          # Git operations failed
│   ├── npm-error.wav          # NPM operations failed
│   ├── bash-error.wav         # General bash failures
│   └── error.wav              # Generic error sound
├── interactive/           # UserPromptSubmit sounds
│   ├── message-sent.wav       # User sent a message
│   └── interactive.wav        # Generic interaction
└── default.wav            # Ultimate fallback sound
```

## The 5-Level Fallback System

When Claude Code triggers an event, Claudio searches for the most specific sound available using this hierarchy:

### Example: Git Commit Success

When Claude runs `git commit -m "fix bug"` and it succeeds:

1. **Level 1 - Exact Hint Match**
   ```
   success/git-commit-success.wav
   ```
   Most specific: exact tool + subcommand + result

2. **Level 2 - Tool-Specific**
   ```
   success/git-success.wav
   ```
   Tool-specific: any git operation that succeeded

3. **Level 3 - Operation-Specific**
   ```
   success/bash-success.wav
   ```
   Operation type: any bash command that succeeded

4. **Level 4 - Category-Specific**
   ```
   success/success.wav
   ```
   Result category: any operation that succeeded

5. **Level 5 - Default**
   ```
   default.wav
   ```
   Ultimate fallback: always present

### Example: NPM Install Thinking

When Claude is about to run `npm install express`:

1. **Level 1 - Exact Hint Match**
   ```
   loading/npm-install-thinking.wav
   ```
   Most specific: npm install starting

2. **Level 2 - Tool-Specific**
   ```
   loading/npm-thinking.wav
   ```
   Tool-specific: any npm operation starting

3. **Level 3 - Operation-Specific**
   ```
   loading/bash-thinking.wav
   ```
   Operation type: any bash command starting

4. **Level 4 - Category-Specific**
   ```
   loading/loading.wav
   ```
   Event category: any operation starting

5. **Level 5 - Default**
   ```
   default.wav
   ```
   Ultimate fallback

## Sound Categories

### loading/ - PreToolUse Events

Played when Claude Code is about to run a tool. These are "thinking" or "working" sounds.

**Common Files:**
- `git-thinking.wav` - Git operations starting
- `npm-thinking.wav` - NPM operations starting  
- `docker-thinking.wav` - Docker operations starting
- `bash-thinking.wav` - General bash commands starting
- `loading.wav` - Generic loading sound

### success/ - PostToolUse Success

Played when a tool completes successfully (no stderr, zero exit code).

**Common Files:**
- `git-commit-success.wav` - Git commits succeeded
- `git-push-success.wav` - Git pushes succeeded
- `npm-install-success.wav` - NPM installs succeeded
- `test-success.wav` - Test suites passed
- `build-success.wav` - Build processes succeeded
- `success.wav` - Generic success sound

### error/ - PostToolUse Failures

Played when a tool fails (stderr present, non-zero exit code).

**Common Files:**
- `git-error.wav` - Git operations failed
- `npm-error.wav` - NPM operations failed
- `test-error.wav` - Test suites failed
- `build-error.wav` - Build processes failed
- `error.wav` - Generic error sound

### interactive/ - UserPromptSubmit

Played when you send a message to Claude Code.

**Common Files:**
- `message-sent.wav` - User sent a message
- `interactive.wav` - Generic interaction sound

## Creating Custom Soundpacks

### Step 1: Create Directory Structure

```bash
mkdir -p ~/.local/share/claudio/my-pack/{loading,success,error,interactive}
```

### Step 2: Add Sound Files

Add `.wav` or `.mp3` files to appropriate directories. Start with essentials:

```bash
# Essential files for a functional soundpack
touch ~/.local/share/claudio/my-pack/loading/loading.wav
touch ~/.local/share/claudio/my-pack/success/success.wav
touch ~/.local/share/claudio/my-pack/error/error.wav
touch ~/.local/share/claudio/my-pack/interactive/interactive.wav
touch ~/.local/share/claudio/my-pack/default.wav
```

### Step 3: Configure Claudio

Update your configuration to use the new soundpack:

```json
{
  "default_soundpack": "my-pack",
  "soundpack_paths": [
    "/home/user/.local/share/claudio",
    "/usr/local/share/claudio"
  ]
}
```

### Step 4: Test Your Soundpack

```bash
# Test with your new soundpack
CLAUDIO_SOUNDPACK=my-pack echo '{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"success"}}' | claudio
```

## Advanced Soundpack Techniques

### Tool-Specific Customization

Create highly specific sounds for tools you use frequently:

```
success/
├── git-commit-success.wav     # Git commits
├── git-push-success.wav       # Git pushes
├── git-pull-success.wav       # Git pulls
├── npm-install-success.wav    # NPM installs
├── npm-test-success.wav       # NPM tests
├── docker-build-success.wav   # Docker builds
└── pytest-success.wav        # Python tests
```

### Contextual Error Sounds

Different sounds for different types of failures:

```
error/
├── git-merge-error.wav        # Merge conflicts
├── npm-install-error.wav      # Dependency issues
├── test-error.wav             # Test failures
├── build-error.wav            # Compilation errors
└── network-error.wav          # Network timeouts
```

### Minimal Soundpacks

For distraction-free environments, create minimal soundpacks:

```
minimal/
├── success/
│   └── success.wav           # Subtle success chime
├── error/
│   └── error.wav             # Gentle error tone
└── default.wav               # Quiet notification
```

## Soundpack Discovery

Claudio searches for soundpacks in these locations:

1. **Custom paths** (from configuration `soundpack_paths`)
2. **User directory:** `~/.local/share/claudio/`
3. **System directory:** `/usr/local/share/claudio/`
4. **System fallback:** `/usr/share/claudio/`

### Listing Available Soundpacks

```bash
# Check what soundpacks are available
ls ~/.local/share/claudio/
ls /usr/local/share/claudio/
```

### Testing Soundpack Availability

```bash
# Test if a soundpack exists and works
CLAUDIO_SOUNDPACK=test-pack echo '...' | claudio
```

## Audio Format Requirements

**Supported Formats:**
- WAV (recommended)
- MP3

**Recommendations:**
- **Sample Rate:** 44.1kHz or 48kHz
- **Bit Depth:** 16-bit or 24-bit
- **Length:** 0.5-3 seconds for UI sounds
- **Volume:** Normalized to prevent clipping

**Avoid:**
- Very long sounds (>5 seconds) for frequent events
- Sounds with long fade-ins for immediate feedback
- Extremely quiet or loud sounds (use consistent levels)

## Built-in Tool Detection

Claudio recognizes these tools and can provide specific sounds:

**Version Control:**
- git, svn, hg

**Package Managers:**
- npm, yarn, pip, cargo, composer

**Build Tools:**
- make, cmake, gradle, maven

**Containers:**
- docker, podman, kubectl

**Languages/Runtime:**
- node, python, go, rust, java

**Testing:**
- pytest, jest, mocha, cargo test

## Soundpack Best Practices

1. **Start Simple:** Begin with category-level sounds (success.wav, error.wav)
2. **Add Gradually:** Add tool-specific sounds for frequently used tools
3. **Stay Consistent:** Use similar audio characteristics across your soundpack
4. **Test Thoroughly:** Verify sounds work for common development workflows
5. **Consider Context:** Match sound mood to your development environment

## Troubleshooting Soundpacks

**Soundpack not found:**
- Check soundpack name matches directory name exactly
- Verify soundpack directory is in search paths
- Check directory permissions are readable

**No sounds playing:**
- Ensure `default.wav` exists (required fallback)
- Check audio file formats are supported (WAV/MP3)
- Verify files aren't corrupted or empty

**Wrong sounds playing:**
- Enable debug logging: `CLAUDIO_LOG_LEVEL=debug`
- Check fallback chain in debug output
- Verify file naming matches expected patterns

## See Also

- **[Configuration](/configuration)** - Setting up soundpack paths
- **[Examples](/examples)** - Real-world soundpack usage scenarios
- **[CLI Reference](/cli-reference)** - Command-line soundpack options