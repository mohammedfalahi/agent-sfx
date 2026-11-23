package audio_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"agent-sfx/internal/audio"
)

// pcmSubFormat is KSDATAFORMAT_SUBTYPE_PCM, the GUID that marks a
// WAVE_FORMAT_EXTENSIBLE file as ordinary integer PCM.
var pcmSubFormat = []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71}

// extensibleWAV builds a WAVE_FORMAT_EXTENSIBLE (0xFFFE) file, the layout
// macOS afconvert writes, with the given valid bits and subformat GUID.
func extensibleWAV(channels uint16, validBits uint16, subFormat []byte, samples []int16) []byte {
	var b bytes.Buffer
	data := new(bytes.Buffer)
	for _, s := range samples {
		_ = binary.Write(data, binary.LittleEndian, s)
	}
	blockAlign := channels * 2
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(4+8+40+8+data.Len()))
	b.WriteString("WAVEfmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(40))
	_ = binary.Write(&b, binary.LittleEndian, uint16(0xFFFE))
	_ = binary.Write(&b, binary.LittleEndian, channels)
	_ = binary.Write(&b, binary.LittleEndian, uint32(44100))
	_ = binary.Write(&b, binary.LittleEndian, uint32(44100)*uint32(blockAlign))
	_ = binary.Write(&b, binary.LittleEndian, blockAlign)
	_ = binary.Write(&b, binary.LittleEndian, uint16(16))
	_ = binary.Write(&b, binary.LittleEndian, uint16(22)) // cbSize
	_ = binary.Write(&b, binary.LittleEndian, validBits)
	_ = binary.Write(&b, binary.LittleEndian, uint32(0x4)) // channel mask
	b.Write(subFormat)
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(data.Len()))
	b.Write(data.Bytes())
	return b.Bytes()
}

func TestValidateAndParseWAV_ExtensiblePCM16(t *testing.T) {
	wav := extensibleWAV(1, 16, pcmSubFormat, []int16{0, 1000, -1000, 0})
	info, err := audio.ValidateAndParseWAV(bytes.NewReader(wav), int64(len(wav)))
	if err != nil {
		t.Fatalf("extensible 16-bit PCM should be accepted: %v", err)
	}
	if info.NumChannels != 1 || info.BitsPerSample != 16 || info.DataSize != 8 {
		t.Errorf("unexpected info %+v", info)
	}
}

func TestValidateAndParseWAV_ExtensibleRejectsNonPCM(t *testing.T) {
	float := append([]byte{0x03, 0x00}, pcmSubFormat[2:]...) // KSDATAFORMAT_SUBTYPE_IEEE_FLOAT
	cases := map[string][]byte{
		"float subformat": extensibleWAV(1, 16, float, []int16{0, 1}),
		"20 valid bits":   extensibleWAV(1, 20, pcmSubFormat, []int16{0, 1}),
	}
	for name, wav := range cases {
		if _, err := audio.ValidateAndParseWAV(bytes.NewReader(wav), int64(len(wav))); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}
	short := extensibleWAV(1, 16, pcmSubFormat, []int16{0, 1})
	binary.LittleEndian.PutUint32(short[16:20], 18) // fmt chunk too small for the extension
	if _, err := audio.ValidateAndParseWAV(bytes.NewReader(short), int64(len(short))); err == nil {
		t.Error("truncated extensible fmt chunk: expected rejection")
	}
}

// Scaling an extensible input must produce a plain PCM (format 1) file.
func TestScaleWAVVolume_ExtensibleOutputsPlainPCM(t *testing.T) {
	wav := extensibleWAV(2, 16, pcmSubFormat, []int16{1000, -1000, 2000, -2000})
	out, err := audio.ScaleWAVVolume(bytes.NewReader(wav), int64(len(wav)), 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint16(out[20:22]); got != 1 {
		t.Errorf("scaled output format = %#x, want 1", got)
	}
	if got := int16(binary.LittleEndian.Uint16(out[44:46])); got != 500 {
		t.Errorf("first scaled sample = %d, want 500", got)
	}
}
