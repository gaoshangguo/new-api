package service

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand/v2"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	captchaImageWidth  = 200
	captchaImageHeight = 64

	// 字形按 captchaGlyphScale 倍放大后逐个像素绘制，配合随机旋转与正弦扭曲。
	captchaGlyphScale = 3
	captchaGlyphGap   = 14

	// 每个像素取 captchaSupersampling² 个子采样点，使旋转后的字形边缘平滑。
	captchaSupersampling = 3

	captchaWarpAmplitude  = 2.4
	captchaWarpWavelength = 46.0
)

// 字形颜色（深色，保证与浅色背景的对比度；A 即绘制不透明度）。
var captchaGlyphColors = []color.RGBA{
	{R: 34, G: 47, B: 95, A: 255},
	{R: 118, G: 36, B: 36, A: 255},
	{R: 22, G: 82, B: 50, A: 255},
	{R: 84, G: 38, B: 100, A: 255},
	{R: 112, G: 66, B: 12, A: 255},
	{R: 16, G: 66, B: 92, A: 255},
}

// 干扰曲线与噪点颜色（半透明，避免盖住字形）。
var captchaNoiseColors = []color.RGBA{
	{R: 46, G: 96, B: 132, A: 150},
	{R: 132, G: 62, B: 46, A: 150},
	{R: 62, G: 108, B: 62, A: 130},
	{R: 104, G: 84, B: 40, A: 130},
}

// renderCaptchaImage 生成验证码图片的 PNG 字节流。
func renderCaptchaImage(code string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, captchaImageWidth, captchaImageHeight))
	drawCaptchaBackground(img)

	face := basicfont.Face7x13
	glyphWidth := face.Width * captchaGlyphScale
	totalWidth := len(code)*glyphWidth + (len(code)-1)*captchaGlyphGap
	startX := (captchaImageWidth - totalWidth) / 2
	for i := 0; i < len(code); i++ {
		centerX := float64(startX + i*(glyphWidth+captchaGlyphGap) + glyphWidth/2)
		centerY := float64(captchaImageHeight/2) + float64(rand.IntN(9)-4)
		angle := (rand.Float64()*2 - 1) * 0.42
		drawCaptchaGlyph(
			img,
			captchaGlyphMask(code[i]),
			centerX,
			centerY,
			angle,
			captchaGlyphColors[rand.IntN(len(captchaGlyphColors))],
		)
	}

	drawCaptchaCurves(img)
	drawCaptchaSpeckles(img)
	distortCaptchaImage(img)

	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// captchaGlyphMask 把单个字符渲染为位图掩码。
func captchaGlyphMask(ch byte) *image.Alpha {
	face := basicfont.Face7x13
	mask := image.NewAlpha(image.Rect(0, 0, face.Width, face.Height))
	drawer := &font.Drawer{
		Dst:  mask,
		Src:  image.NewUniform(color.Alpha{A: 255}),
		Face: face,
		Dot:  fixed.P(0, face.Ascent),
	}
	drawer.DrawString(string(rune(ch)))
	return mask
}

// drawCaptchaGlyph 将字形放大、按随机角度旋转后绘制到图片上。
func drawCaptchaGlyph(dst *image.RGBA, mask *image.Alpha, centerX, centerY, angle float64, clr color.RGBA) {
	scale := float64(captchaGlyphScale)
	halfWidth := float64(mask.Bounds().Dx()) * scale / 2
	halfHeight := float64(mask.Bounds().Dy()) * scale / 2
	sin, cos := math.Sincos(angle)

	// 把字形像素空间的坐标映射到图片坐标（先绕字形中心旋转，再平移到目标位置）。
	toImage := func(x, y float64) [2]float64 {
		dx, dy := x-halfWidth, y-halfHeight
		return [2]float64{centerX + dx*cos - dy*sin, centerY + dx*sin + dy*cos}
	}

	bounds := mask.Bounds()
	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			if mask.AlphaAt(px, py).A == 0 {
				continue
			}
			fillCaptchaQuad(dst, [4][2]float64{
				toImage(float64(px)*scale, float64(py)*scale),
				toImage(float64(px+1)*scale, float64(py)*scale),
				toImage(float64(px+1)*scale, float64(py+1)*scale),
				toImage(float64(px)*scale, float64(py+1)*scale),
			}, clr, 1)
		}
	}
}

// fillCaptchaQuad 以子采样覆盖率抗锯齿的方式填充一个凸四边形。
func fillCaptchaQuad(dst *image.RGBA, quad [4][2]float64, clr color.RGBA, coverage float64) {
	minX, maxX := quad[0][0], quad[0][0]
	minY, maxY := quad[0][1], quad[0][1]
	for _, point := range quad[1:] {
		minX, maxX = math.Min(minX, point[0]), math.Max(maxX, point[0])
		minY, maxY = math.Min(minY, point[1]), math.Max(maxY, point[1])
	}
	xStart := max(int(math.Floor(minX)), dst.Rect.Min.X)
	xEnd := min(int(math.Ceil(maxX)), dst.Rect.Max.X)
	yStart := max(int(math.Floor(minY)), dst.Rect.Min.Y)
	yEnd := min(int(math.Ceil(maxY)), dst.Rect.Max.Y)

	step := 1 / float64(captchaSupersampling)
	samples := float64(captchaSupersampling * captchaSupersampling)
	for y := yStart; y < yEnd; y++ {
		for x := xStart; x < xEnd; x++ {
			inside := 0
			for offsetY := 0; offsetY < captchaSupersampling; offsetY++ {
				for offsetX := 0; offsetX < captchaSupersampling; offsetX++ {
					sampleX := float64(x) + (float64(offsetX)+0.5)*step
					sampleY := float64(y) + (float64(offsetY)+0.5)*step
					if captchaQuadContains(quad, sampleX, sampleY) {
						inside++
					}
				}
			}
			if inside == 0 {
				continue
			}
			blendCaptchaPixel(dst, x, y, clr, coverage*float64(inside)/samples)
		}
	}
}

// captchaQuadContains 判断点是否落在凸四边形内，顶点需按顺序给出。
func captchaQuadContains(quad [4][2]float64, x, y float64) bool {
	hasPositive, hasNegative := false, false
	for i := range quad {
		from := quad[i]
		to := quad[(i+1)%len(quad)]
		cross := (to[0]-from[0])*(y-from[1]) - (to[1]-from[1])*(x-from[0])
		hasPositive = hasPositive || cross > 0
		hasNegative = hasNegative || cross < 0
		if hasPositive && hasNegative {
			return false
		}
	}
	return true
}

// drawCaptchaBackground 绘制浅色渐变背景。
func drawCaptchaBackground(img *image.RGBA) {
	base := float64(228 + rand.IntN(14))
	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		shade := base - float64(y)*0.12 + float64(rand.IntN(7)-3)
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: captchaChannel(shade + 8),
				G: captchaChannel(shade + 4),
				B: captchaChannel(shade - 6),
				A: 255,
			})
		}
	}
}

// drawCaptchaCurves 绘制随机干扰曲线，打断字形的连续边缘。
func drawCaptchaCurves(img *image.RGBA) {
	width := float64(img.Rect.Dx())
	height := float64(img.Rect.Dy())
	for i := 0; i < 4; i++ {
		start := [2]float64{rand.Float64() * width * 0.15, rand.Float64() * height}
		control := [2]float64{width * (0.35 + rand.Float64()*0.3), rand.Float64() * height}
		end := [2]float64{width * (0.85 + rand.Float64()*0.15), rand.Float64() * height}
		clr := captchaNoiseColors[rand.IntN(len(captchaNoiseColors))]

		for t := 0.0; t <= 1; t += 0.004 {
			remaining := 1 - t
			x := remaining*remaining*start[0] + 2*remaining*t*control[0] + t*t*end[0]
			y := remaining*remaining*start[1] + 2*remaining*t*control[1] + t*t*end[1]
			blendCaptchaPixel(img, int(x), int(y), clr, 1)
		}
	}
}

// drawCaptchaSpeckles 撒入随机噪点，进一步干扰自动识别。
func drawCaptchaSpeckles(img *image.RGBA) {
	for i := 0; i < 240; i++ {
		x := rand.IntN(img.Rect.Dx())
		y := rand.IntN(img.Rect.Dy())
		clr := captchaNoiseColors[rand.IntN(len(captchaNoiseColors))]
		blendCaptchaPixel(img, x, y, clr, 0.8)
		if x+1 < img.Rect.Dx() {
			blendCaptchaPixel(img, x+1, y, clr, 0.4)
		}
	}
}

// distortCaptchaImage 按正弦波逐行做水平位移，使字形边缘不再是直线。
func distortCaptchaImage(img *image.RGBA) {
	source := image.NewRGBA(img.Bounds())
	copy(source.Pix, img.Pix)

	amplitude := captchaWarpAmplitude * (0.6 + rand.Float64()*0.8)
	wavelength := captchaWarpWavelength * (0.75 + rand.Float64()*0.5)
	phase := rand.Float64() * 2 * math.Pi

	width := img.Rect.Dx()
	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		offset := int(math.Round(amplitude * math.Sin(2*math.Pi*float64(y)/wavelength+phase)))
		sourceRow := y * source.Stride
		targetRow := y * img.Stride
		for x := 0; x < width; x++ {
			sourceX := min(max(x-offset, 0), width-1)
			copy(img.Pix[targetRow+x*4:targetRow+x*4+4], source.Pix[sourceRow+sourceX*4:sourceRow+sourceX*4+4])
		}
	}
}

// blendCaptchaPixel 按颜色自带的不透明度乘以覆盖率，把颜色混合到目标像素。
func blendCaptchaPixel(dst *image.RGBA, x, y int, clr color.RGBA, coverage float64) {
	if coverage <= 0 || !image.Pt(x, y).In(dst.Rect) {
		return
	}
	alpha := math.Min(coverage*float64(clr.A)/255, 1)
	existing := dst.RGBAAt(x, y)
	dst.SetRGBA(x, y, color.RGBA{
		R: captchaChannel(float64(clr.R)*alpha + float64(existing.R)*(1-alpha)),
		G: captchaChannel(float64(clr.G)*alpha + float64(existing.G)*(1-alpha)),
		B: captchaChannel(float64(clr.B)*alpha + float64(existing.B)*(1-alpha)),
		A: 255,
	})
}

// captchaChannel 把浮点颜色分量截断到 [0, 255]。
func captchaChannel(value float64) uint8 {
	return uint8(min(255, max(0, int(math.Round(value)))))
}
