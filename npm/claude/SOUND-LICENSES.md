# Sound Assets License and Provenance (Claude Code Plugin)

## Provenance
All starter sound effects included in this directory (`sounds/`) are original, 100% software-generated audio assets synthesized specifically for Agent SFX. They do not contain any third-party audio recordings, commercial clips, or copied audio from other projects.

The mathematical synthesis definitions for these exact sound files are maintained in the Agent SFX core repository (`cmd/gen-sounds/main.go`).

### Specifications:
- **Format**: RIFF / WAVE (uncompressed 16-bit signed PCM)
- **Channels**: 1 (mono)
- **Sample Rate**: 44,100 Hz
- **Bit Depth**: 16-bit little-endian signed integers

### Synthesis Details:
1. `sounds/permission_requested/permission_requested.wav`:
   - Dual-tone melodic bell chime (A4 440.0 Hz, 100ms followed by C#5 554.37 Hz, 180ms) with exponential decay envelope and subtle 2nd harmonic overtone.
2. `sounds/task_started/task_started.wav`:
   - Ascending frequency chirp (smooth continuous sweep from 330.0 Hz to 660.0 Hz over 180ms) with a bell-shaped amplitude window.
3. `sounds/task_finished/task_finished.wav`:
   - Bright major triad chord progression (C5 523.25 Hz for 70ms, E5 659.25 Hz for 70ms, G5 783.99 Hz for 160ms) with natural acoustic decay.
4. `sounds/waiting_for_user/waiting_for_user.wav`:
   - Gentle double pulse (50ms ping at 698.46 Hz, 40ms silence, 120ms ping at 698.46 Hz) with smooth envelope.
5. `sounds/tests_passed/tests_passed.wav`:
   - Triumphant 4-note ascending fanfare (G4 392.00 Hz, C5 523.25 Hz, E5 659.25 Hz, G5 783.99 Hz) with brisk phrasing and ringing final note.
6. `sounds/usage_exhausted/usage_exhausted.wav`:
   - Descending 2-note minor chime (E5 659.25 Hz for 90ms down to C5 523.25 Hz for 180ms) with soft harmonics.
7. `sounds/error/error.wav`:
   - Low caution interval (simultaneous dual-tone mix of 220.0 Hz and 233.08 Hz for 220ms) with exponential decay.

## Dedication (CC0 1.0 Universal)
To the extent possible under law, the authors have dedicated all copyright and related and neighboring rights to these sound files to the public domain worldwide under the Creative Commons Zero (CC0 1.0 Universal) Public Domain Dedication.

You can copy, modify, distribute, and perform the work, even for commercial purposes, all without asking permission.
Full license text: https://creativecommons.org/publicdomain/zero/1.0/
