//go:build cgo

package cli

// Blank-import the malgo subpackage so its init() registers the "malgo"
// backend with the parent audio package. Under !cgo this file is excluded
// and the malgo subpackage never compiles or registers, which is how the
// build communicates "this binary cannot do real audio" to NewBackend.
import _ "claudio.click/internal/audio/malgo"
