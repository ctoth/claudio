package cli

// Register the native Oto backend in every build, including CGO_ENABLED=0.
import _ "claudio.click/internal/audio/native"
