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
│   ├── git-commit-start.wav     # Git commit operations starting
│   ├── git-start.wav            # Git operations starting
│   ├── npm-start.wav            # NPM operations starting
│   ├── bash-start.wav           # General bash operations
│   └── loading.wav              # Generic loading sound
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

## The Multi-Level Fallback System

Claudio uses different fallback chains depending on the event type. Each chain searches for the most specific sound available:

### PostToolUse Success Events (6-Level Fallback)

When Claude runs `git commit -m "fix bug"` and it succeeds:

1. **Level 1 - Exact Hint Match**
   ```
   success/git-commit-success.wav
   ```
   Most specific: exact tool + subcommand + result

2. **Level 2 - Command with Suffix**
   ```
   success/git-success.wav
   ```
   Tool-specific: git + success suffix

3. **Level 3 - Original Tool with Suffix**
   ```
   success/bash-success.wav
   ```
   Original tool: bash + success suffix

4. **Level 4 - Operation-Specific**
   ```
   success/tool-complete.wav
   ```
   Operation type: tool completion

5. **Level 5 - Category-Specific**
   ```
   success/success.wav
   ```
   Result category: any successful operation

6. **Level 6 - Default**
   ```
   default.wav
   ```
   Ultimate fallback: always present

### PreToolUse Loading Events (9-Level Enhanced Fallback)

When Claude is about to run `npm install express`:

1. **Level 1 - Exact Hint Match**
   ```
   loading/npm-install-start.wav
   ```
   Most specific: npm install starting

2. **Level 2 - Command-Subcommand**
   ```
   loading/npm-install.wav
   ```
   Tool + subcommand: npm install

3. **Level 3 - Command with Suffix**
   ```
   loading/npm-start.wav
   ```
   Tool + start suffix: npm starting

4. **Level 4 - Command-Only**
   ```
   loading/npm.wav
   ```
   Tool-specific: any npm operation

5. **Level 5 - Original Tool with Suffix**
   ```
   loading/bash-start.wav
   ```
   Original tool + suffix: bash starting

6. **Level 6 - Original Tool**
   ```
   loading/bash.wav
   ```
   Original tool: bash operations

7. **Level 7 - Operation-Specific**
   ```
   loading/tool-start.wav
   ```
   Operation type: tool starting

8. **Level 8 - Category-Specific**
   ```
   loading/loading.wav
   ```
   Event category: any operation starting

9. **Level 9 - Default**
   ```
   default.wav
   ```
   Ultimate fallback

### Simple Events (4-Level Fallback)

For UserPromptSubmit and other simple events:

1. **Level 1 - Specific Hint**
   ```
   interactive/message-sent.wav
   ```
   Event-specific sound

2. **Level 2 - Event-Specific**
   ```
   interactive/prompt-submit.wav
   ```
   Operation-based sound

3. **Level 3 - Category-Specific**
   ```
   interactive/interactive.wav
   ```
   Category fallback

4. **Level 4 - Default**
   ```
   default.wav
   ```
   Ultimate fallback

## Sound Categories

### loading/ - PreToolUse Events

Played when Claude Code is about to run a tool. These are "start" or "loading" sounds.

**Common Files:**
- `git-commit-start.wav` - Git commit operations starting
- `git-start.wav` - Git operations starting
- `npm-install-start.wav` - NPM install operations starting
- `npm-start.wav` - NPM operations starting  
- `docker-start.wav` - Docker operations starting
- `bash-start.wav` - General bash commands starting
- `tool-start.wav` - Generic tool starting
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