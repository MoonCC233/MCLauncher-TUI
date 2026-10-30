// Package skin validates and stores Minecraft player skins and serves them to
// the game through a small local Yggdrasil API.
package skin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"

	"github.com/MoonCC233/MCLauncher-TUI/internal/util"
)

const (
	// Width is the only width a Minecraft skin may have.
	Width = 64
	// HeightModern is the current skin size, with a separate left arm and leg.
	HeightModern = 64
	// HeightLegacy is the pre-1.8 size; its left limbs mirror the right ones.
	HeightLegacy = 32

	ModelClassic = "classic"
	ModelSlim    = "slim"
)

// ModelIDs lists the selectable skin models in picker order.
var ModelIDs = []string{ModelClassic, ModelSlim}

// ValidModel reports whether model is a selectable skin model.
func ValidModel(model string) bool {
	return model == ModelClassic || model == ModelSlim
}

// NormalizeModel maps an empty or unknown model to classic.
func NormalizeModel(model string) string {
	if model == ModelSlim {
		return ModelSlim
	}
	return ModelClassic
}

// Skin is a validated Minecraft skin kept as NRGBA pixels so it can be
// re-encoded, hashed and sampled for the terminal preview.
type Skin struct {
	width  int
	height int
	img    *image.NRGBA
}

// maxSkinFile bounds the compressed input so a crafted PNG cannot exhaust
// memory before its dimensions are checked.
const maxSkinFile = 8 << 20

// Decode parses a PNG skin. Both 64×64 and the legacy 64×32 layout are
// accepted; anything else is rejected before it can reach the game.
func Decode(r io.Reader) (*Skin, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxSkinFile+1))
	if err != nil {
		return nil, fmt.Errorf("read PNG: %w", err)
	}
	if len(data) > maxSkinFile {
		return nil, fmt.Errorf("skin file is larger than %d MB", maxSkinFile>>20)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode PNG: %w", err)
	}
	if cfg.Width != Width || (cfg.Height != HeightModern && cfg.Height != HeightLegacy) {
		return nil, fmt.Errorf("skin must be %d×%d or %d×%d pixels, got %d×%d",
			Width, HeightModern, Width, HeightLegacy, cfg.Width, cfg.Height)
	}
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode PNG: %w", err)
	}
	b := src.Bounds()
	img := image.NewNRGBA(image.Rect(0, 0, Width, b.Dy()))
	draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)
	return &Skin{width: Width, height: b.Dy(), img: img}, nil
}

// Load reads and validates a skin file.
func Load(path string) (*Skin, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Decode(f)
}

// Width returns the skin width in pixels.
func (s *Skin) Width() int { return s.width }

// Height returns the skin height in pixels (64 or 32).
func (s *Skin) Height() int { return s.height }

// Legacy reports whether the skin uses the 64×32 layout.
func (s *Skin) Legacy() bool { return s.height == HeightLegacy }

// At returns one pixel, or a transparent pixel outside the image.
func (s *Skin) At(x, y int) color.NRGBA {
	if s == nil || s.img == nil || !(image.Point{X: x, Y: y}).In(s.img.Bounds()) {
		return color.NRGBA{}
	}
	return s.img.NRGBAAt(x, y)
}

// Bytes encodes the skin as PNG. It is the canonical form written to disk and
// served to the game, so its SHA-256 is stable across runs.
func (s *Skin) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, s.img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Hash returns the SHA-256 of the canonical PNG encoding.
func (s *Skin) Hash() (string, error) {
	data, err := s.Bytes()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Save writes the canonical PNG form atomically.
func (s *Skin) Save(path string) error {
	data, err := s.Bytes()
	if err != nil {
		return err
	}
	return util.WriteFileAtomic(path, data, 0o644)
}

// SpriteSize is the player silhouette returned by Sprite.
const (
	SpriteWidth  = 16
	SpriteHeight = 32
)

// Sprite draws a front-facing player silhouette. Overlay layers are composited
// over the base skin and transparent pixels are filled with bg, which makes the
// result directly usable for half-block terminal rendering. Legacy skins mirror
// the right arm and leg, matching the vanilla conversion.
func Sprite(s *Skin, bg color.NRGBA) [][]color.NRGBA {
	out := make([][]color.NRGBA, SpriteHeight)
	for y := range out {
		row := make([]color.NRGBA, SpriteWidth)
		for x := range row {
			row[x] = bg
		}
		out[y] = row
	}
	if s == nil {
		return out
	}
	modern := !s.Legacy()

	// Head, then the hat overlay.
	copyPart(s, out, 4, 0, 8, 8, 8, 8, false)
	if modern {
		copyPart(s, out, 4, 0, 40, 8, 8, 8, false)
	}
	// Body and jacket.
	copyPart(s, out, 4, 8, 20, 20, 8, 12, false)
	if modern {
		copyPart(s, out, 4, 8, 20, 36, 8, 12, false)
	}
	// Right arm (viewer's left), then left arm.
	copyPart(s, out, 0, 8, 44, 20, 4, 12, false)
	if modern {
		copyPart(s, out, 12, 8, 36, 52, 4, 12, false)
	} else {
		copyPart(s, out, 12, 8, 44, 20, 4, 12, true)
	}
	// Sleeves.
	if modern {
		copyPart(s, out, 0, 8, 44, 36, 4, 12, false)
		copyPart(s, out, 12, 8, 52, 52, 4, 12, false)
	}
	// Right leg, then left leg.
	copyPart(s, out, 4, 20, 4, 20, 4, 12, false)
	if modern {
		copyPart(s, out, 8, 20, 20, 52, 4, 12, false)
	} else {
		copyPart(s, out, 8, 20, 4, 20, 4, 12, true)
	}
	// Pants overlays.
	if modern {
		copyPart(s, out, 4, 20, 4, 36, 4, 12, false)
		copyPart(s, out, 8, 20, 4, 52, 4, 12, false)
	}
	return out
}

func copyPart(s *Skin, dst [][]color.NRGBA, dx, dy, sx, sy, w, h int, mirror bool) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcX := sx + x
			if mirror {
				srcX = sx + w - 1 - x
			}
			px := s.At(srcX, sy+y)
			if px.A == 0 {
				continue
			}
			dst[dy+y][dx+x] = blend(dst[dy+y][dx+x], px)
		}
	}
}

// blend composites src over dst using straight alpha.
func blend(dst, src color.NRGBA) color.NRGBA {
	if src.A == 255 {
		return src
	}
	a := float64(src.A) / 255
	inv := 1 - a
	return color.NRGBA{
		R: uint8(float64(src.R)*a + float64(dst.R)*inv),
		G: uint8(float64(src.G)*a + float64(dst.G)*inv),
		B: uint8(float64(src.B)*a + float64(dst.B)*inv),
		A: 255,
	}
}

// Default returns a generated Steve-like skin, used as a preview when an
// offline account has no custom skin yet. It is never written automatically.
func Default(model string) *Skin {
	img := image.NewNRGBA(image.Rect(0, 0, Width, HeightModern))
	fill := func(x, y, w, h int, c color.NRGBA) {
		draw.Draw(img, image.Rect(x, y, x+w, y+h), &image.Uniform{C: c}, image.Point{}, draw.Src)
	}
	skinTone := color.NRGBA{R: 0xc8, G: 0x9d, B: 0x74, A: 0xff}
	hair := color.NRGBA{R: 0x3a, G: 0x2a, B: 0x1d, A: 0xff}
	shirt := color.NRGBA{R: 0x3e, G: 0x9e, B: 0x93, A: 0xff}
	shirtDark := color.NRGBA{R: 0x2f, G: 0x7c, B: 0x74, A: 0xff}
	// Denim, darker at the shoes.
	denim := color.NRGBA{R: 0x3b, G: 0x4a, B: 0x9e, A: 0xff}
	shoe := color.NRGBA{R: 0x3a, G: 0x3a, B: 0x42, A: 0xff}
	eye := color.NRGBA{R: 0xf2, G: 0xf2, B: 0xf2, A: 0xff}
	pupil := color.NRGBA{R: 0x4a, G: 0x6d, B: 0xa8, A: 0xff}

	// Head: hair cap, face, eyes and mouth.
	fill(8, 8, 8, 8, skinTone)
	fill(8, 8, 8, 2, hair)
	fill(8, 8, 1, 4, hair)
	fill(15, 8, 1, 4, hair)
	fill(9, 11, 2, 1, eye)
	fill(13, 11, 2, 1, eye)
	fill(10, 11, 1, 1, pupil)
	fill(14, 11, 1, 1, pupil)
	fill(10, 14, 4, 1, color.NRGBA{R: 0x8a, G: 0x5f, B: 0x4b, A: 0xff})
	// Body: shirt with a darker belt.
	fill(20, 20, 8, 12, shirt)
	fill(20, 20, 8, 2, shirtDark)
	fill(20, 29, 8, 3, shirtDark)
	// Right arm, left arm, right leg, left leg (front faces).
	fill(44, 20, 4, 12, skinTone)
	fill(44, 20, 4, 4, shirt)
	fill(36, 52, 4, 12, skinTone)
	fill(36, 52, 4, 4, shirt)
	fill(4, 20, 4, 12, denim)
	fill(4, 28, 4, 4, shoe)
	fill(20, 52, 4, 12, denim)
	fill(20, 60, 4, 4, shoe)
	// A slim skin leaves the outer column of each arm transparent.
	if NormalizeModel(model) == ModelSlim {
		fill(47, 20, 1, 12, color.NRGBA{})
		fill(39, 52, 1, 12, color.NRGBA{})
	}
	return &Skin{width: Width, height: HeightModern, img: img}
}
