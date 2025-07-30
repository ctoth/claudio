---
layout: default
title: "Examples"
---

# Examples & Usage Scenarios

See how Claudio integrates into real development workflows with contextual audio feedback.

## Git Workflow Scenarios

### Scenario 1: Successful Commit Flow

**What Claude Does:**
```bash
git add src/main.go
git commit -m "Fix authentication bug"
git push origin main
```

**What You Hear:**
1. `loading/git-thinking.wav` - Git add starting
2. `success/git-success.wav` - Git add succeeded
3. `loading/git-commit-thinking.wav` - Git commit starting  
4. `success/git-commit-success.wav` - Git commit succeeded
5. `loading/git-push-thinking.wav` - Git push starting
6. `success/git-push-success.wav` - Git push succeeded

### Scenario 2: Merge Conflict Resolution

**What Claude Does:**
```bash
git pull origin main
# (merge conflict occurs)
# Claude edits files to resolve conflicts
git add .
git commit -m "Resolve merge conflicts"
```

**What You Hear:**
1. `loading/git-thinking.wav` - Git pull starting
2. `error/git-error.wav` - Git pull failed (merge conflicts)
3. `loading/git-thinking.wav` - Git add starting (after conflict resolution)
4. `success/git-success.wav` - Git add succeeded
5. `loading/git-commit-thinking.wav` - Git commit starting
6. `success/git-commit-success.wav` - Merge conflicts resolved

## Development Workflow Scenarios

### Scenario 3: NPM Package Development

**What Claude Does:**
```bash
npm install express
npm run test
npm run build
npm publish
```

**What You Hear:**
1. `loading/npm-install-thinking.wav` - NPM install starting
2. `success/npm-install-success.wav` - Dependencies installed
3. `loading/npm-test-thinking.wav` - Test suite starting
4. `success/npm-test-success.wav` - All tests passed
5. `loading/npm-build-thinking.wav` - Build process starting
6. `success/npm-build-success.wav` - Build completed
7. `loading/npm-thinking.wav` - NPM publish starting
8. `success/npm-success.wav` - Package published

### Scenario 4: Docker Development

**What Claude Does:**
```bash
docker build -t myapp .
docker run -p 3000:3000 myapp
docker ps
```

**What You Hear:**
1. `loading/docker-build-thinking.wav` - Docker build starting
2. `success/docker-build-success.wav` - Image built successfully
3. `loading/docker-thinking.wav` - Docker run starting
4. `success/docker-success.wav` - Container started
5. `loading/docker-thinking.wav` - Docker ps starting
6. `success/docker-success.wav` - Container status displayed

## Error Handling Scenarios

### Scenario 5: Test Failure Investigation

**What Claude Does:**
```bash
npm test
# (tests fail)
# Claude investigates the failing test file
cat tests/user.test.js
# Claude fixes the test
npm test
```

**What You Hear:**
1. `loading/npm-test-thinking.wav` - Test suite starting
2. `error/npm-test-error.wav` - Tests failed
3. `loading/bash-thinking.wav` - File read starting
4. `success/bash-success.wav` - File read completed
5. `loading/npm-test-thinking.wav` - Test suite starting again
6. `success/npm-test-success.wav` - Tests now pass

### Scenario 6: Build System Errors

**What Claude Does:**
```bash
make build
# (compilation errors)
# Claude examines error logs
make clean
make build
```

**What You Hear:**
1. `loading/bash-thinking.wav` - Make build starting
2. `error/build-error.wav` - Compilation failed
3. `loading/bash-thinking.wav` - Make clean starting
4. `success/bash-success.wav` - Clean completed
5. `loading/bash-thinking.wav` - Make build starting again
6. `success/build-success.wav` - Build succeeded

## Interactive Development Scenarios

### Scenario 7: Code Review Session

**User Actions and Sounds:**

**You:** "Review this function for potential bugs"
- **Sound:** `interactive/message-sent.wav`

**Claude analyzes code, runs tests:**
```bash
go test ./internal/auth
go vet ./internal/auth
```
- **Sounds:** `loading/go-test-thinking.wav` → `success/go-test-success.wav`

**You:** "Fix the race condition you found"
- **Sound:** `interactive/message-sent.wav`

**Claude implements fix, verifies:**
```bash
go test -race ./internal/auth
```
- **Sounds:** `loading/go-test-thinking.wav` → `success/go-test-success.wav`

### Scenario 8: Feature Development Session

**You:** "Add user authentication to the API"
- **Sound:** `interactive/message-sent.wav`

**Claude creates files, installs dependencies:**
```bash
touch internal/auth/middleware.go
go mod tidy
go test ./internal/auth
```
- **Sounds:** Multiple loading/success cycles as Claude builds the feature

**You:** "Test it with a real request"
- **Sound:** `interactive/message-sent.wav`

**Claude tests the implementation:**
```bash
curl -X POST localhost:8080/auth/login
```
- **Sounds:** `loading/bash-thinking.wav` → `success/bash-success.wav`

## Multitasking Benefits

### Background Development Awareness

While Claude works on complex tasks, audio feedback lets you:

**Stay Informed Without Watching:**
- Continue other work while Claude builds/tests
- Know immediately when operations complete
- Hear when errors occur that need attention

**Example: Long Build Process**
```bash
# Claude starts a complex build
npm run build:production
```
- You hear `loading/npm-build-thinking.wav` and can switch to other tasks
- 5 minutes later, `success/npm-build-success.wav` tells you it's done
- No need to check terminal constantly

### Multi-Step Operation Tracking

**Example: Database Migration**
```bash
# Claude performs multi-step database update
npm run migrate:backup
npm run migrate:run
npm run migrate:verify
npm run test:integration
```

**Audio Timeline:**
1. `loading/npm-thinking.wav` → `success/npm-success.wav` (backup)
2. `loading/npm-thinking.wav` → `success/npm-success.wav` (migrate) 
3. `loading/npm-thinking.wav` → `success/npm-success.wav` (verify)
4. `loading/npm-test-thinking.wav` → `success/npm-test-success.wav` (tests)

You know each step completed successfully without monitoring the screen.

## Common Development Patterns

### Pattern 1: Code-Test-Fix Cycle

**Typical Flow:**
```bash
# Claude writes code
# Tests the code
go test ./...
# Test fails, Claude investigates
go test -v ./pkg/utils
# Claude fixes issue
# Tests again
go test ./...
```

**Audio Pattern:**
- Loading sound → Error sound (test fails)
- Loading sound → Success sound (investigation complete)  
- Loading sound → Success sound (tests now pass)

### Pattern 2: Dependency Management

**Typical Flow:**
```bash
# Add new dependency
npm install lodash
# Update lockfile
npm audit fix
# Verify everything works
npm test
```

**Audio Pattern:**
- Loading sound → Success sound (install)
- Loading sound → Success sound (audit)
- Loading sound → Success sound (tests)

### Pattern 3: Deployment Pipeline

**Typical Flow:**
```bash
# Build application
npm run build
# Run security scan
npm audit
# Deploy to staging
npm run deploy:staging
# Run smoke tests
npm run test:smoke
```

**Audio Pattern:**
- Series of loading → success sounds indicating pipeline progress
- Any error sound alerts you to investigate specific stage

## Customization for Your Workflow

### Tool-Specific Soundpacks

Create sounds that match your most common tools:

**Python Development:**
```
loading/
├── pytest-thinking.wav
├── pip-install-thinking.wav
└── python-thinking.wav

success/
├── pytest-success.wav
├── pip-install-success.wav
└── python-success.wav
```

**Frontend Development:**
```
loading/
├── webpack-thinking.wav
├── jest-thinking.wav
└── yarn-thinking.wav

success/
├── webpack-success.wav
├── jest-success.wav
└── yarn-success.wav
```

### Environment-Specific Configurations

**Development Environment (Detailed Feedback):**
```json
{
  "volume": 0.8,
  "default_soundpack": "detailed",
  "log_level": "debug"
}
```

**Production Environment (Minimal Sounds):**
```json
{
  "volume": 0.3,
  "default_soundpack": "minimal",
  "log_level": "error"
}
```

## Productivity Benefits

### Immediate Error Awareness
- No need to constantly check terminal output
- Audio alerts draw attention to failures immediately
- Faster response to issues that need investigation

### Multi-Step Process Confidence
- Know when long operations complete
- Confidence that each step in complex workflows succeeded
- Early warning when processes stall or fail

### Reduced Context Switching
- Stay focused on other tasks while Claude works
- Audio feedback reduces need to monitor progress visually
- Smoother development flow with less interruption

## See Also

- **[Soundpacks](/soundpacks)** - Create custom sounds for your workflow
- **[Configuration](/configuration)** - Adjust volume and behavior for your environment
- **[Installation](/installation)** - Get started with Claudio