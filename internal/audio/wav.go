package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"time"
)

const (
	MaxWAVSizeBytes = 25 * 1024 * 1024 // 25 MiB limit for up to 60s stereo 16-bit PCM at 48kHz
	ProcessHeadroom = 5 * time.Second  // 5 seconds bounded overhead for OS audio server setup & teardown
)

// WAVInfo stores the validated format metadata of an uncompressed 16-bit PCM WAV.
type WAVInfo struct {
	NumChannels   uint16
	SampleRate    uint32
	BitsPerSample uint16
	BlockAlign    uint16
	ByteRate      uint32
	DataOffset    int64
	DataSize      uint32
	DurationMS    int64
}

// ValidateClipDuration checks that the audio clip's PCM duration does not exceed maxAllowedMS.
// It uses precise byte-level and ceiling-millisecond comparison so longer clips are never rounded down.
func (w *WAVInfo) ValidateClipDuration(maxAllowedMS int) error {
	bytesPerSecond := int64(w.SampleRate) * int64(w.BlockAlign)
	if bytesPerSecond > 0 {
		maxBytes := (int64(maxAllowedMS) * bytesPerSecond) / 1000
		if int64(w.DataSize) > maxBytes {
			return fmt.Errorf("audio clip duration %dms exceeds maximum allowed duration %dms", w.DurationMS, maxAllowedMS)
		}
	}
	if w.DurationMS > int64(maxAllowedMS) {
		return fmt.Errorf("audio clip duration %dms exceeds maximum allowed duration %dms", w.DurationMS, maxAllowedMS)
	}
	return nil
}

// ComputePlayerTimeout returns the bounded process execution timeout for an audio clip:
// actual clip duration + 5 seconds of process overhead.
// The overhead accommodates CoreAudio / ALSA startup and buffer teardown latency and does not
// permit longer audio.
func ComputePlayerTimeout(clipDurationMS int64) time.Duration {
	return time.Duration(clipDurationMS)*time.Millisecond + ProcessHeadroom
}

// ValidateAndParseWAV checks that the given data is a valid uncompressed 16-bit PCM WAV file.
// It enforces bounds on file size and audio parameters.
func ValidateAndParseWAV(r io.ReaderAt, size int64) (*WAVInfo, error) {
	if size < 44 {
		return nil, errors.New("wav file is too small to contain a valid header")
	}
	if size > MaxWAVSizeBytes {
		return nil, fmt.Errorf("wav file exceeds max allowed size of %d bytes (got %d)", MaxWAVSizeBytes, size)
	}

	header := make([]byte, 12)
	if _, err := r.ReadAt(header, 0); err != nil {
		return nil, fmt.Errorf("failed to read RIFF header: %w", err)
	}

	if string(header[0:4]) != "RIFF" {
		return nil, fmt.Errorf("invalid magic: expected 'RIFF', got %q", string(header[0:4]))
	}

	riffLength := binary.LittleEndian.Uint32(header[4:8])
	if int64(riffLength)+8 > size+1 { // allow slight padding discrepancies
		return nil, fmt.Errorf("corrupt RIFF chunk size: header reports %d, file size is %d", riffLength+8, size)
	}

	if string(header[8:12]) != "WAVE" {
		return nil, fmt.Errorf("invalid format: expected 'WAVE', got %q", string(header[8:12]))
	}

	offset := int64(12)
	var foundFmt, foundData bool
	var info WAVInfo

	for offset+8 <= size {
		chunkHeader := make([]byte, 8)
		if _, err := r.ReadAt(chunkHeader, offset); err != nil {
			return nil, fmt.Errorf("failed to read chunk header at offset %d: %w", offset, err)
		}

		chunkID := string(chunkHeader[0:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])
		chunkDataOffset := offset + 8

		if offset+8+int64(chunkSize) > size && chunkID == "data" {
			// Some encoders don't update data chunk size if file was truncated
			return nil, fmt.Errorf("chunk %q size %d extends beyond file size %d", chunkID, chunkSize, size)
		}

		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return nil, fmt.Errorf("fmt chunk size %d is too small for PCM", chunkSize)
			}
			fmtData := make([]byte, 16)
			if _, err := r.ReadAt(fmtData, chunkDataOffset); err != nil {
				return nil, fmt.Errorf("failed to read fmt chunk: %w", err)
			}

			audioFormat := binary.LittleEndian.Uint16(fmtData[0:2])
			if audioFormat != 1 {
				return nil, fmt.Errorf("unsupported audio format %d: only uncompressed PCM (format 1) is supported", audioFormat)
			}

			info.NumChannels = binary.LittleEndian.Uint16(fmtData[2:4])
			if info.NumChannels != 1 && info.NumChannels != 2 {
				return nil, fmt.Errorf("unsupported channel count %d: must be 1 (mono) or 2 (stereo)", info.NumChannels)
			}

			info.SampleRate = binary.LittleEndian.Uint32(fmtData[4:8])
			if info.SampleRate < 8000 || info.SampleRate > 192000 {
				return nil, fmt.Errorf("unsupported sample rate %d: must be between 8000 and 192000 Hz", info.SampleRate)
			}

			info.ByteRate = binary.LittleEndian.Uint32(fmtData[8:12])
			info.BlockAlign = binary.LittleEndian.Uint16(fmtData[12:14])
			info.BitsPerSample = binary.LittleEndian.Uint16(fmtData[14:16])

			if info.BitsPerSample != 16 {
				return nil, fmt.Errorf("unsupported bits per sample %d: only 16-bit PCM is supported", info.BitsPerSample)
			}

			expectedBlockAlign := info.NumChannels * 2
			if info.BlockAlign != expectedBlockAlign {
				return nil, fmt.Errorf("invalid block align %d: expected %d", info.BlockAlign, expectedBlockAlign)
			}

			foundFmt = true

		case "data":
			if !foundFmt {
				return nil, errors.New("data chunk encountered before fmt chunk")
			}
			info.DataOffset = chunkDataOffset
			info.DataSize = chunkSize
			foundData = true

			// Calculate duration accurately from sample count and format using ceiling math
			bytesPerSecond := int64(info.SampleRate) * int64(info.BlockAlign)
			if bytesPerSecond > 0 {
				info.DurationMS = int64(math.Ceil(float64(chunkSize) * 1000.0 / float64(bytesPerSecond)))
			}
		}

		if foundData {
			break
		}

		// RIFF chunks are padded to word boundaries (2 bytes)
		paddedSize := int64(chunkSize)
		if paddedSize%2 != 0 {
			paddedSize++
		}
		offset = chunkDataOffset + paddedSize
	}

	if !foundFmt {
		return nil, errors.New("missing 'fmt ' chunk in WAV file")
	}
	if !foundData {
		return nil, errors.New("missing 'data' chunk in WAV file")
	}
	if info.DataSize%uint32(info.BlockAlign) != 0 {
		return nil, errors.New("data chunk size is not aligned to sample frame size")
	}

	return &info, nil
}

// ValidateWAVFile reads and validates a WAV file on disk.
func ValidateWAVFile(filePath string) (*WAVInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAV file: %w", err)
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat WAV file: %w", err)
	}

	return ValidateAndParseWAV(file, fi.Size())
}

// ScaleWAVVolume reads an input 16-bit PCM WAV, scales all audio samples by volume (0.0..1.0),
// and returns the scaled WAV as bytes.
func ScaleWAVVolume(r io.ReaderAt, size int64, volume float64) ([]byte, error) {
	info, err := ValidateAndParseWAV(r, size)
	if err != nil {
		return nil, err
	}

	if volume < 0.0 || volume > 1.0 {
		return nil, fmt.Errorf("invalid volume %v: must be between 0.0 and 1.0", volume)
	}

	rawData := make([]byte, info.DataSize)
	if _, err := r.ReadAt(rawData, info.DataOffset); err != nil {
		return nil, fmt.Errorf("failed to read audio samples: %w", err)
	}

	numSamples := len(rawData) / 2
	scaledData := make([]byte, len(rawData))

	for i := 0; i < numSamples; i++ {
		idx := i * 2
		sample := int16(binary.LittleEndian.Uint16(rawData[idx : idx+2]))

		var scaledSample int16
		if volume == 0.0 {
			scaledSample = 0
		} else if volume == 1.0 {
			scaledSample = sample
		} else {
			scaledFloat := float64(sample) * volume
			if scaledFloat > math.MaxInt16 {
				scaledSample = math.MaxInt16
			} else if scaledFloat < math.MinInt16 {
				scaledSample = math.MinInt16
			} else {
				scaledSample = int16(math.Round(scaledFloat))
			}
		}

		binary.LittleEndian.PutUint16(scaledData[idx:idx+2], uint16(scaledSample))
	}

	// Construct complete new WAV
	var buf bytes.Buffer
	totalSize := uint32(36 + len(scaledData)) // 4 (WAVE) + 24 (fmt) + 8 (data header) + len(scaledData)
	buf.WriteString("RIFF")
	_ = binary.Write(&buf, binary.LittleEndian, totalSize)
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(16)) // subchunk size
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))  // PCM
	_ = binary.Write(&buf, binary.LittleEndian, info.NumChannels)
	_ = binary.Write(&buf, binary.LittleEndian, info.SampleRate)
	_ = binary.Write(&buf, binary.LittleEndian, info.ByteRate)
	_ = binary.Write(&buf, binary.LittleEndian, info.BlockAlign)
	_ = binary.Write(&buf, binary.LittleEndian, info.BitsPerSample)

	// data chunk
	buf.WriteString("data")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(scaledData)))
	buf.Write(scaledData)

	return buf.Bytes(), nil
}

// EncodePCM16WAV generates a standard 16-bit PCM WAV byte slice from raw samples.
func EncodePCM16WAV(numChannels uint16, sampleRate uint32, samples []int16) []byte {
	dataSize := uint32(len(samples) * 2)
	blockAlign := numChannels * 2
	byteRate := sampleRate * uint32(blockAlign)
	totalChunkSize := uint32(36 + dataSize)

	var buf bytes.Buffer
	buf.WriteString("RIFF")
	_ = binary.Write(&buf, binary.LittleEndian, totalChunkSize)
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, numChannels)
	_ = binary.Write(&buf, binary.LittleEndian, sampleRate)
	_ = binary.Write(&buf, binary.LittleEndian, byteRate)
	_ = binary.Write(&buf, binary.LittleEndian, blockAlign)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(16))

	// data chunk
	buf.WriteString("data")
	_ = binary.Write(&buf, binary.LittleEndian, dataSize)
	for _, s := range samples {
		_ = binary.Write(&buf, binary.LittleEndian, s)
	}

	return buf.Bytes()
}
