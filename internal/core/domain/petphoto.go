package domain

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
)

// MaxPetPhotoSide caps each image dimension so a small file cannot expand into
// a huge bitmap (decompression bomb). The web client uploads at most 1280 px;
// 2048 keeps the worst-case decode around 16 MiB per request.
const MaxPetPhotoSide = 2048

// petPhotoJPEGQualities are tried in order until the re-encoded JPEG fits in
// MaxPetPhotoBytes: low-quality inputs grow when re-encoded at high quality.
var petPhotoJPEGQualities = []int{88, 80, 70, 60, 50}

// SanitizePetPhoto validates an uploaded image and re-encodes it so no
// metadata survives: EXIF (including GPS coordinates that could reveal the
// owner's home), comments and color profiles are dropped. The EXIF
// orientation is applied to the pixels first so phone photos stay upright.
// Only JPEG and PNG are accepted; the type comes from the bytes, never from
// the client.
func SanitizePetPhoto(data []byte) (PetPhoto, error) {
	if len(data) == 0 {
		return PetPhoto{}, fmt.Errorf("%w: empty image", ErrInvalidPetPhoto)
	}
	if len(data) > MaxPetPhotoBytes {
		return PetPhoto{}, fmt.Errorf("%w: image exceeds %d bytes", ErrInvalidPetPhoto, MaxPetPhotoBytes)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") {
		return PetPhoto{}, fmt.Errorf("%w: only JPEG or PNG images are allowed", ErrInvalidPetPhoto)
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > MaxPetPhotoSide || config.Height > MaxPetPhotoSide {
		return PetPhoto{}, fmt.Errorf("%w: image must be at most %dpx per side", ErrInvalidPetPhoto, MaxPetPhotoSide)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return PetPhoto{}, fmt.Errorf("%w: unreadable image", ErrInvalidPetPhoto)
	}

	var out bytes.Buffer
	if format == "png" {
		if err := png.Encode(&out, img); err != nil {
			return PetPhoto{}, fmt.Errorf("encode pet photo: %w", err)
		}
		if out.Len() > MaxPetPhotoBytes {
			return PetPhoto{}, fmt.Errorf("%w: image exceeds %d bytes", ErrInvalidPetPhoto, MaxPetPhotoBytes)
		}
		return PetPhoto{ContentType: "image/png", Data: out.Bytes()}, nil
	}

	img = applyOrientation(img, jpegOrientation(data))
	for _, quality := range petPhotoJPEGQualities {
		out.Reset()
		if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: quality}); err != nil {
			return PetPhoto{}, fmt.Errorf("encode pet photo: %w", err)
		}
		if out.Len() <= MaxPetPhotoBytes {
			return PetPhoto{ContentType: "image/jpeg", Data: out.Bytes()}, nil
		}
	}
	return PetPhoto{}, fmt.Errorf("%w: image exceeds %d bytes", ErrInvalidPetPhoto, MaxPetPhotoBytes)
}

// jpegOrientation returns the EXIF orientation (1-8) of a JPEG, or 1 when it
// is absent or unreadable.
func jpegOrientation(data []byte) int {
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		if marker == 0xDA || marker == 0xD9 { // start of scan / end of image
			return 1
		}
		length := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if length < 2 || i+2+length > len(data) {
			return 1
		}
		segment := data[i+4 : i+2+length]
		if marker == 0xE1 && bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			return tiffOrientation(segment[6:])
		}
		i += 2 + length
	}
	return 1
}

func tiffOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	offset := order.Uint32(tiff[4:8])
	if offset < 8 || uint64(offset)+2 > uint64(len(tiff)) {
		return 1
	}
	ifd := int(offset)
	entries := int(order.Uint16(tiff[ifd : ifd+2]))
	for n := 0; n < entries; n++ {
		entry := ifd + 2 + n*12
		if entry+12 > len(tiff) {
			return 1
		}
		if order.Uint16(tiff[entry:entry+2]) == 0x0112 {
			if value := int(order.Uint16(tiff[entry+8 : entry+10])); value >= 1 && value <= 8 {
				return value
			}
			return 1
		}
	}
	return 1
}

// applyOrientation turns the pixels so the image displays upright without
// relying on EXIF.
func applyOrientation(src image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	// One RGBA copy keeps the per-pixel loop allocation-free; at 2048 px per
	// side source and destination together stay around 32 MiB.
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(rgba, rgba.Bounds(), src, b.Min, draw.Src)

	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			var sx, sy int
			switch orientation {
			case 2: // mirror horizontal
				sx, sy = w-1-x, y
			case 3: // rotate 180
				sx, sy = w-1-x, h-1-y
			case 4: // mirror vertical
				sx, sy = x, h-1-y
			case 5: // transpose
				sx, sy = y, x
			case 6: // rotate 90 clockwise
				sx, sy = y, h-1-x
			case 7: // transverse
				sx, sy = w-1-y, h-1-x
			case 8: // rotate 90 counter-clockwise
				sx, sy = w-1-y, x
			}
			dst.SetRGBA(x, y, rgba.RGBAAt(sx, sy))
		}
	}
	return dst
}
