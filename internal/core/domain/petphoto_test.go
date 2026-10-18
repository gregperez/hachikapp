package domain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// testImage is 40x20 with a red left half so rotations are observable.
func testImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			c := color.RGBA{0, 0, 255, 255}
			if x < 20 {
				c = color.RGBA{255, 0, 0, 255}
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// exifSegment builds an APP1 Exif segment with an orientation tag and a GPS
// IFD pointer, mimicking what phone cameras embed.
func exifSegment(orientation uint16) []byte {
	tiff := new(bytes.Buffer)
	tiff.WriteString("MM")
	_ = binary.Write(tiff, binary.BigEndian, uint16(42))
	_ = binary.Write(tiff, binary.BigEndian, uint32(8))
	_ = binary.Write(tiff, binary.BigEndian, uint16(2)) // entries
	// Orientation (0x0112), SHORT, count 1, value left-justified.
	_ = binary.Write(tiff, binary.BigEndian, []uint16{0x0112, 3})
	_ = binary.Write(tiff, binary.BigEndian, uint32(1))
	_ = binary.Write(tiff, binary.BigEndian, []uint16{orientation, 0})
	// GPSInfo IFD pointer (0x8825), LONG.
	_ = binary.Write(tiff, binary.BigEndian, []uint16{0x8825, 4})
	_ = binary.Write(tiff, binary.BigEndian, uint32(1))
	_ = binary.Write(tiff, binary.BigEndian, uint32(38))
	_ = binary.Write(tiff, binary.BigEndian, uint32(0)) // next IFD
	tiff.WriteString("GPS-33.4489,-70.6693")

	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	segment := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(payload)+2))
	return append(segment, payload...)
}

func withExif(jpegBytes []byte, orientation uint16) []byte {
	out := append([]byte{}, jpegBytes[:2]...) // SOI
	out = append(out, exifSegment(orientation)...)
	return append(out, jpegBytes[2:]...)
}

func TestSanitizePetPhotoStripsMetadataFromJPEG(t *testing.T) {
	raw := withExif(encodeJPEG(t, testImage()), 1)
	if !bytes.Contains(raw, []byte("GPS-33.4489")) {
		t.Fatal("fixture must carry location metadata")
	}
	photo, err := SanitizePetPhoto(raw)
	if err != nil {
		t.Fatal(err)
	}
	if photo.ContentType != "image/jpeg" {
		t.Fatalf("content type=%q", photo.ContentType)
	}
	if bytes.Contains(photo.Data, []byte("Exif")) || bytes.Contains(photo.Data, []byte("GPS-33.4489")) {
		t.Fatal("sanitized photo still carries EXIF/GPS metadata")
	}
	img, err := jpeg.Decode(bytes.NewReader(photo.Data))
	if err != nil || img.Bounds().Dx() != 40 || img.Bounds().Dy() != 20 {
		t.Fatalf("decoded=%v err=%v", img.Bounds(), err)
	}
}

func TestSanitizePetPhotoAppliesExifOrientation(t *testing.T) {
	// Orientation 6 = the camera stored the image rotated; viewers turn it 90° clockwise.
	photo, err := SanitizePetPhoto(withExif(encodeJPEG(t, testImage()), 6))
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(photo.Data))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 20 || img.Bounds().Dy() != 40 {
		t.Fatalf("expected 20x40 after rotating, got %v", img.Bounds())
	}
	// Rotated clockwise, the red left half ends up on top.
	r, _, b, _ := img.At(10, 5).RGBA()
	if r < b {
		t.Fatalf("top of the rotated image should be red, got r=%d b=%d", r, b)
	}
}

func TestSanitizePetPhotoKeepsPNG(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, testImage()); err != nil {
		t.Fatal(err)
	}
	photo, err := SanitizePetPhoto(buf.Bytes())
	if err != nil || photo.ContentType != "image/png" {
		t.Fatalf("photo=%q err=%v", photo.ContentType, err)
	}
	if _, err := png.Decode(bytes.NewReader(photo.Data)); err != nil {
		t.Fatalf("output must be a valid PNG: %v", err)
	}
}

func TestSanitizePetPhotoRejectsInvalidImages(t *testing.T) {
	cases := map[string][]byte{
		"empty":            nil,
		"svg":              []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`),
		"gif":              []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
		"webp":             []byte("RIFF\x24\x00\x00\x00WEBPVP8 "),
		"truncated jpeg":   encodeJPEG(t, testImage())[:40],
		"too many bytes":   make([]byte, MaxPetPhotoBytes+1),
		"oversized pixels": oversizedPNGHeader(),
	}
	for name, data := range cases {
		if _, err := SanitizePetPhoto(data); !errors.Is(err, ErrInvalidPetPhoto) {
			t.Fatalf("%s: expected ErrInvalidPetPhoto, got %v", name, err)
		}
	}
}

// oversizedPNGHeader declares 20000x20000 pixels; it must be rejected from the
// header alone, before allocating the image (decompression bomb guard).
func oversizedPNGHeader() []byte {
	var buf bytes.Buffer
	buf.WriteString("\x89PNG\r\n\x1a\n")
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], 20000)
	binary.BigEndian.PutUint32(ihdr[4:], 20000)
	ihdr[8], ihdr[9] = 8, 2
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(ihdr)))
	buf.WriteString("IHDR")
	buf.Write(ihdr)
	buf.Write([]byte{0, 0, 0, 0}) // CRC is not checked by DecodeConfig
	return buf.Bytes()
}

func TestSanitizePetPhotoOtherOrientations(t *testing.T) {
	cases := []struct {
		orientation uint16
		w, h        int
		x, y        int
		wantRed     bool
	}{
		{3, 40, 20, 5, 10, false}, // 180°: the red left half moves right
		{8, 20, 40, 10, 5, false}, // 90° counter-clockwise: red ends at the bottom
		{8, 20, 40, 10, 35, true},
	}
	for _, tc := range cases {
		photo, err := SanitizePetPhoto(withExif(encodeJPEG(t, testImage()), tc.orientation))
		if err != nil {
			t.Fatal(err)
		}
		img, err := jpeg.Decode(bytes.NewReader(photo.Data))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != tc.w || img.Bounds().Dy() != tc.h {
			t.Fatalf("orientation %d: size %v", tc.orientation, img.Bounds())
		}
		r, _, b, _ := img.At(tc.x, tc.y).RGBA()
		if (r > b) != tc.wantRed {
			t.Fatalf("orientation %d at (%d,%d): r=%d b=%d wantRed=%v", tc.orientation, tc.x, tc.y, r, b, tc.wantRed)
		}
	}
}

func TestSanitizePetPhotoCapsDimensionsAt2048(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 2049, 4))); err != nil {
		t.Fatal(err)
	}
	if _, err := SanitizePetPhoto(buf.Bytes()); !errors.Is(err, ErrInvalidPetPhoto) {
		t.Fatalf("2049px wide must be rejected to bound decode memory, got %v", err)
	}
	buf.Reset()
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 2048, 4))); err != nil {
		t.Fatal(err)
	}
	if _, err := SanitizePetPhoto(buf.Bytes()); err != nil {
		t.Fatalf("2048px must be accepted: %v", err)
	}
}

// A compressed (low-quality) JPEG under 1 MiB grows when re-encoded at high
// quality; it must still be accepted by stepping the quality down.
func TestSanitizePetPhotoKeepsLowQualityJPEGUnderTheLimit(t *testing.T) {
	const side = 1200
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	seed := uint32(1)
	for i := range img.Pix {
		seed = seed*1664525 + 1013904223
		img.Pix[i] = uint8(seed >> 24)
		if i%4 == 3 {
			img.Pix[i] = 255
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 50}); err != nil {
		t.Fatal(err)
	}
	if buf.Len() > MaxPetPhotoBytes {
		t.Fatalf("fixture must start under the limit, got %d bytes", buf.Len())
	}
	photo, err := SanitizePetPhoto(buf.Bytes())
	if err != nil {
		t.Fatalf("a %d-byte JPEG within the limit was rejected: %v", buf.Len(), err)
	}
	if len(photo.Data) > MaxPetPhotoBytes {
		t.Fatalf("output %d bytes exceeds the limit", len(photo.Data))
	}
}
