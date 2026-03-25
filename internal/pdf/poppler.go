package pdf

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var pagesLine = regexp.MustCompile(`(?m)^Pages:\s+(\d+)`)
var encryptedLine = regexp.MustCompile(`(?m)^Encrypted:\s+(\S+)`)

// PageCount returns the number of pages using pdfinfo.
func PageCount(pdfPath string) (int, error) {
	out, err := exec.Command("pdfinfo", pdfPath).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("pdfinfo: %w: %s", err, strings.TrimSpace(string(out)))
	}
	m := pagesLine.FindStringSubmatch(string(out))
	if len(m) < 2 {
		return 0, fmt.Errorf("pdfinfo: could not parse page count")
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 1 {
		return 0, fmt.Errorf("pdfinfo: invalid page count")
	}
	return n, nil
}

// IsEncrypted reports whether the PDF is password-protected.
func IsEncrypted(pdfPath string) (bool, error) {
	out, err := exec.Command("pdfinfo", pdfPath).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("pdfinfo: %w: %s", err, strings.TrimSpace(string(out)))
	}
	m := encryptedLine.FindStringSubmatch(string(out))
	if len(m) < 2 {
		return false, nil
	}
	return strings.EqualFold(m[1], "yes"), nil
}

// ExtractText extracts plain text per page, pages joined with "\n\n" (legacy parity).
func ExtractText(pdfPath string) (string, error) {
	n, err := PageCount(pdfPath)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for i := 1; i <= n; i++ {
		cmd := exec.Command("pdftotext", "-f", strconv.Itoa(i), "-l", strconv.Itoa(i), pdfPath, "-")
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("pdftotext page %d: %w: %s", i, err, strings.TrimSpace(out.String()))
		}
		s := strings.TrimSpace(out.String())
		if i > 1 {
			b.WriteString("\n\n")
		}
		b.WriteString(s)
	}
	return b.String(), nil
}

// StitchToPNG renders all pages with pdftoppm, optionally crops white margins, writes one vertical PNG.
func StitchToPNG(pdfPath, outputPNG string, cropMargins bool, dpi int) error {
	tmpDir, err := os.MkdirTemp("", "pdfppm-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	prefix := filepath.Join(tmpDir, "p")
	args := []string{"-png", "-r", strconv.Itoa(dpi), pdfPath, prefix}
	cmd := exec.Command("pdftoppm", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pdftoppm: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".png") && strings.HasPrefix(name, "p-") {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return fmt.Errorf("pdftoppm: no PNG pages generated")
	}
	sort.Slice(names, func(i, j int) bool {
		return pageSuffix(names[i]) < pageSuffix(names[j])
	})

	var pages []image.Image
	for _, name := range names {
		path := filepath.Join(tmpDir, name)
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		img, err := png.Decode(f)
		_ = f.Close()
		if err != nil {
			return fmt.Errorf("decode %s: %w", name, err)
		}
		if cropMargins {
			img = cropWhiteMargins(img, 240, 10)
		}
		pages = append(pages, img)
	}

	combined := stackVertical(pages)
	out, err := os.Create(outputPNG)
	if err != nil {
		return err
	}
	defer out.Close()
	if err := png.Encode(out, combined); err != nil {
		return err
	}
	return nil
}

func pageSuffix(filename string) int {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	parts := strings.Split(base, "-")
	if len(parts) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(parts[len(parts)-1])
	return n
}

func stackVertical(pages []image.Image) image.Image {
	if len(pages) == 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	maxW := 0
	totalH := 0
	for _, im := range pages {
		b := im.Bounds()
		if b.Dx() > maxW {
			maxW = b.Dx()
		}
		totalH += b.Dy()
	}
	dst := image.NewRGBA(image.Rect(0, 0, maxW, totalH))
	// white background
	draw.Draw(dst, dst.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	y := 0
	for _, im := range pages {
		b := im.Bounds()
		r := image.Rect(0, y, b.Dx(), y+b.Dy())
		draw.Draw(dst, r, im, b.Min, draw.Over)
		y += b.Dy()
	}
	return dst
}

func cropWhiteMargins(img image.Image, threshold uint8, margin int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return img
	}

	minX, minY := w, h
	maxX, maxY := 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			lum := luminanceAt(img, b.Min.X+x, b.Min.Y+y)
			if lum <= threshold {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if minX > maxX || minY > maxY {
		return img
	}
	left := max(0, minX-margin)
	top := max(0, minY-margin)
	right := minInt(w-1, maxX+margin)
	bottom := minInt(h-1, maxY+margin)
	sub := image.Rect(b.Min.X+left, b.Min.Y+top, b.Min.X+right+1, b.Min.Y+bottom+1)
	type subImage interface {
		SubImage(r image.Rectangle) image.Image
	}
	if s, ok := img.(subImage); ok {
		return s.SubImage(sub)
	}
	// fallback copy
	rgba := image.NewRGBA(sub)
	draw.Draw(rgba, rgba.Bounds(), img, sub.Min, draw.Src)
	return rgba
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func luminanceAt(img image.Image, x, y int) uint8 {
	r, g, b, _ := img.At(x, y).RGBA()
	// 8-bit approx
	yv := (299*r + 587*g + 114*b + 500000) / 1000000
	if yv > 255 {
		return 255
	}
	return uint8(yv)
}
