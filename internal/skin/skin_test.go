package skin

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeAcceptsModernAndLegacySkins(t *testing.T) {
	for _, height := range []int{HeightModern, HeightLegacy} {
		img := image.NewNRGBA(image.Rect(0, 0, Width, height))
		img.SetNRGBA(8, 8, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
		got, err := Decode(&buf)
		if err != nil {
			t.Fatalf("Decode(%d): %v", height, err)
		}
		if got.Width() != Width || got.Height() != height {
			t.Errorf("size = %d×%d, want %d×%d", got.Width(), got.Height(), Width, height)
		}
		if px := got.At(8, 8); px != (color.NRGBA{R: 10, G: 20, B: 30, A: 255}) {
			t.Errorf("pixel = %+v", px)
		}
	}
}

func TestDecodeRejectsWrongSizeAndNonPNG(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(&buf); err == nil {
		t.Error("32×32 skin was accepted")
	}
	if _, err := Decode(bytes.NewReader([]byte("not a png"))); err == nil {
		t.Error("non-PNG was accepted")
	}
}

func TestSaveRoundTripAndHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skins", "player.png")
	skin := Default(ModelClassic)
	if err := skin.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if st, err := os.Stat(path); err != nil || st.Size() == 0 {
		t.Fatalf("saved file: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want, err := skin.Hash()
	if err != nil {
		t.Fatal(err)
	}
	got, err := loaded.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("hash = %s, want %s", got, want)
	}
	if len(got) != 64 {
		t.Errorf("hash length = %d", len(got))
	}
}

func TestSpriteDrawsPlayerSilhouette(t *testing.T) {
	for _, s := range []*Skin{Default(ModelClassic), Default(ModelSlim), nil} {
		rows := Sprite(s, color.NRGBA{R: 1, G: 2, B: 3, A: 255})
		if len(rows) != SpriteHeight {
			t.Fatalf("rows = %d, want %d", len(rows), SpriteHeight)
		}
		for y, row := range rows {
			if len(row) != SpriteWidth {
				t.Fatalf("row %d width = %d, want %d", y, len(row), SpriteWidth)
			}
			for x, px := range row {
				if px.A != 255 {
					t.Fatalf("pixel %d,%d is not opaque: %+v", x, y, px)
				}
			}
		}
	}
	rows := Sprite(Default(ModelClassic), color.NRGBA{})
	// The face is opaque, so it must not be the transparent background.
	if rows[1][4] == (color.NRGBA{}) {
		t.Error("head pixels are transparent")
	}
}

func TestSpriteMirrorsLegacyLimbs(t *testing.T) {
	legacy := Default(ModelClassic)
	legacy.height = HeightLegacy
	// Make the legacy right arm red so the mirrored left arm can be checked.
	legacy.img.SetNRGBA(44, 20, color.NRGBA{R: 255, A: 255})
	rows := Sprite(legacy, color.NRGBA{})
	if got := rows[8][0]; got.R != 255 || got.A != 255 {
		t.Errorf("right arm = %+v", got)
	}
	if got := rows[8][15]; got.R != 255 || got.A != 255 {
		t.Errorf("mirrored left arm = %+v", got)
	}
}

func TestDefaultSkinIsValidPNG(t *testing.T) {
	for _, model := range ModelIDs {
		data, err := Default(model).Bytes()
		if err != nil {
			t.Fatalf("%s: %v", model, err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: %v", model, err)
		}
		if img.Bounds().Dx() != Width || img.Bounds().Dy() != HeightModern {
			t.Errorf("%s: size = %v", model, img.Bounds())
		}
	}
}
