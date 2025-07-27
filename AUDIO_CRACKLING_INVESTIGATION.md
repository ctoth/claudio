# Audio Crackling Investigation

## Problem Statement
- Audio playback in Claudio produces crackling sounds
- Volume and clipping have been ruled out as causes
- paplay works perfectly, so it's not a WSL issue
- Issue appears to be in the playback mechanism itself

## Investigation Plan
1. Examine current audio implementation
2. Research malgo/miniaudio common issues
3. Analyze callback and buffer handling
4. Check audio format compatibility
5. Test different configurations
6. Implement fixes with TDD approach

## Findings

### Current Implementation Analysis

#### Volume Processing (RULED OUT)
- User has already tested by commenting out volume processing
- Crackling persists even without volume adjustment
- This rules out the sample-by-sample volume application as the cause

#### Potential Issues Identified in playback.go
1. **Critical: WAV decoder only uses first channel** (wav_decoder.go:114)
   - `val := int16(sample.Values[0])` - only using first channel
   - For stereo files, this discards the right channel data
   - Could cause buffer underruns and timing issues

2. **Buffer size calculations assume 16-bit**
   - Line 235: `totalSamples := uint32(len(audioData.Samples) / int(audioData.Channels) / 2)`
   - Hardcoded division by 2 assumes 16-bit samples
   - Won't work correctly for 24-bit or 32-bit audio

3. **Real-time sample processing in callback**
   - Complex byte manipulation in audio callback (lines 273-284)
   - Even without volume, still doing byte-level operations

4. **No buffer underrun protection**
   - If callback can't keep up, could cause crackling

### Test Audio File Analysis
- Checked `/usr/local/share/claudio/default/success/success.wav`
- Format: **STEREO 44100 Hz, 16-bit PCM**
- This confirms the bug: decoder extracts only first channel from stereo file!

### Malgo Example Analysis
Examined malgo playback example (malgo-repo/_examples/playback/playback.go):
- Line 81: `io.ReadFull(reader, pOutputSample)` - reads raw interleaved PCM data
- For stereo, expects: L R L R L R... (interleaved channels)
- Our decoder provides: L L L L... (only left channel)
- This mismatch causes malgo to interpret mono data as stereo, creating timing issues

### Test Confirms the Bug!
Created TestWavDecoderStereoChannelHandling which shows:
- Expected: 16 bytes (4 stereo samples = 8 samples total * 2 bytes)
- Got: 8 bytes (4 mono samples * 2 bytes)
- Decoder only extracts left channel: 0x1000, 0x2000, 0x3000, 0x4000
- Missing right channel data: 0x0100, 0x0200, 0x0300, 0x0400
- This confirms the decoder is dropping the right channel entirely!

## COMPLETE FAILURE LOG

### Attempted Fixes That Made It WORSE:

1. **Fixed stereo channel handling** - Audio became stereo but crackling got WORSE
2. **Renamed variables for clarity** - No improvement
3. **Simplified callback logic** - Made crackling worse
4. **Ensured full buffer fill** - Made crackling EVEN WORSE
5. **Implemented streaming approach** - Still crackling just as bad

### Root Cause Still Unknown

Despite extensive investigation with o3, Gemini Pro, continuous thinking, and multiple approaches:
- Fixed the obvious stereo bug - made it worse
- Fixed buffer calculations - no improvement  
- Tried streaming like malgo example - still crackling
- The fundamental issue remains unidentified

### The Streaming "Solution" That Didn't Work

Completely rewrote to use io.Reader directly like malgo example:
- Created StreamingWavDecoder that returns wav.Reader as io.Reader
- Created StreamingPlayer that just calls io.ReadFull
- Tested standalone - claimed success but STILL CRACKLING
- Integrated into main claudio - ZERO IMPROVEMENT

### Current Status: TOTAL FAILURE

- Multiple "root causes" identified and "fixed"
- Each fix made the problem the same or worse
- The real issue is still unknown
- Audio crackles as badly as when we started