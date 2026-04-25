package emulator

import (
	"bytes"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/kurakura967/tinygo-wio-terminal-emulator/assets"
)

const (
	ScreenWidth  = 320
	ScreenHeight = 240

	// Device image dimensions (= game logical size).
	imgWidth  = 1190
	imgHeight = 950

	// LCD transparent area inside the device image.
	lcdX = 29
	lcdY = 41
	lcdW = 1133
	lcdH = 801
)

// Buttons holds the current pressed state of emulated buttons.
// true = pressed. machine.Pin.Get() inverts this (active-low).
var Buttons struct {
	A, B, C                       bool
	Up, Down, Left, Right, Center bool
}

// GlobalScreen is the shared drawing buffer between the fake driver and Ebitengine.
var GlobalScreen = newScreen()

type Screen struct {
	mu     sync.Mutex
	buf    [ScreenWidth * ScreenHeight]color.RGBA
	pixels [ScreenWidth * ScreenHeight * 4]byte
}

func newScreen() *Screen {
	s := &Screen{}
	for i := range s.buf {
		s.buf[i] = color.RGBA{0, 0, 0, 255}
	}
	return s
}

func (s *Screen) DrawPixel(x, y int16, c color.RGBA) {
	if x < 0 || x >= ScreenWidth || y < 0 || y >= ScreenHeight {
		return
	}
	s.mu.Lock()
	s.buf[int(y)*ScreenWidth+int(x)] = c
	s.mu.Unlock()
}

func (s *Screen) FillRectangle(x, y, w, h int16, c color.RGBA) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for dy := int16(0); dy < h; dy++ {
		for dx := int16(0); dx < w; dx++ {
			px, py := x+dx, y+dy
			if px < 0 || px >= ScreenWidth || py < 0 || py >= ScreenHeight {
				continue
			}
			s.buf[int(py)*ScreenWidth+int(px)] = c
		}
	}
}

func (s *Screen) copyToImage(img *ebiten.Image) {
	s.mu.Lock()
	for i, c := range s.buf {
		s.pixels[i*4] = c.R
		s.pixels[i*4+1] = c.G
		s.pixels[i*4+2] = c.B
		s.pixels[i*4+3] = c.A
	}
	s.mu.Unlock()
	img.WritePixels(s.pixels[:])
}

// Hit area types for button input detection.
type rectHit struct{ x0, y0, x1, y1 int }

func (r rectHit) contains(mx, my int) bool {
	return mx >= r.x0 && mx <= r.x1 && my >= r.y0 && my <= r.y1
}

type circleHit struct{ cx, cy, r int }

func (c circleHit) contains(mx, my int) bool {
	dx, dy := mx-c.cx, my-c.cy
	return dx*dx+dy*dy <= c.r*c.r
}

// Button hit areas in game coordinates (1190x950 space).
var (
	hitA        = rectHit{168, 0, 274, 30}
	hitB        = rectHit{389, 0, 495, 30}
	hitC        = rectHit{610, 0, 715, 30}
	hitJoystick = circleHit{1042, 843, 40}
)

// game implements ebiten.Game.
type game struct {
	lcdImage    *ebiten.Image
	deviceImage *ebiten.Image
}

func (g *game) Update() error {
	mx, my := ebiten.CursorPosition()
	lmb := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)

	Buttons.A = (lmb && hitA.contains(mx, my)) || ebiten.IsKeyPressed(ebiten.KeyZ)
	Buttons.B = (lmb && hitB.contains(mx, my)) || ebiten.IsKeyPressed(ebiten.KeyX)
	Buttons.C = (lmb && hitC.contains(mx, my)) || ebiten.IsKeyPressed(ebiten.KeyC)
	Buttons.Center = (lmb && hitJoystick.contains(mx, my)) || ebiten.IsKeyPressed(ebiten.KeyEnter)
	Buttons.Up = ebiten.IsKeyPressed(ebiten.KeyArrowUp)
	Buttons.Down = ebiten.IsKeyPressed(ebiten.KeyArrowDown)
	Buttons.Left = ebiten.IsKeyPressed(ebiten.KeyArrowLeft)
	Buttons.Right = ebiten.IsKeyPressed(ebiten.KeyArrowRight)

	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	// 1. Fill the LCD area with black (handles letterboxing if aspect ratios differ).
	vector.DrawFilledRect(screen, float32(lcdX), float32(lcdY), float32(lcdW), float32(lcdH),
		color.RGBA{0, 0, 0, 255}, false)

	// 2. Draw LCD content with uniform scaling (maintain 4:3 aspect ratio), centred in the hole.
	scaleX := float64(lcdW) / float64(ScreenWidth)
	scaleY := float64(lcdH) / float64(ScreenHeight)
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}
	scaledW := float64(ScreenWidth) * scale
	scaledH := float64(ScreenHeight) * scale
	offsetX := float64(lcdX) + (float64(lcdW)-scaledW)/2
	offsetY := float64(lcdY) + (float64(lcdH)-scaledH)/2

	GlobalScreen.copyToImage(g.lcdImage)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(offsetX, offsetY)
	screen.DrawImage(g.lcdImage, op)

	// 3. Draw the device image on top — transparent LCD hole reveals the content below.
	screen.DrawImage(g.deviceImage, nil)

	// 4. Visual feedback for button presses.
	const pressAlpha = 120
	pressColor := color.RGBA{255, 255, 255, pressAlpha}
	if Buttons.A {
		vector.DrawFilledRect(screen,
			float32(hitA.x0), float32(hitA.y0),
			float32(hitA.x1-hitA.x0), float32(hitA.y1-hitA.y0),
			pressColor, false)
	}
	if Buttons.B {
		vector.DrawFilledRect(screen,
			float32(hitB.x0), float32(hitB.y0),
			float32(hitB.x1-hitB.x0), float32(hitB.y1-hitB.y0),
			pressColor, false)
	}
	if Buttons.C {
		vector.DrawFilledRect(screen,
			float32(hitC.x0), float32(hitC.y0),
			float32(hitC.x1-hitC.x0), float32(hitC.y1-hitC.y0),
			pressColor, false)
	}
	if Buttons.Center {
		vector.DrawFilledCircle(screen,
			float32(hitJoystick.cx), float32(hitJoystick.cy),
			float32(hitJoystick.r), pressColor, false)
	}
}

func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return imgWidth, imgHeight
}

// Run starts the Ebitengine window. Call this from main().
func Run() {
	ebiten.SetWindowSize(imgWidth/2, imgHeight/2)
	ebiten.SetWindowTitle("Wio Terminal Emulator")

	img, _, err := image.Decode(bytes.NewReader(assets.WioTerminalBody))
	if err != nil {
		log.Fatalf("loading device image: %v", err)
	}
	deviceImage := ebiten.NewImageFromImage(img)
	lcdImage := ebiten.NewImage(ScreenWidth, ScreenHeight)

	if err := ebiten.RunGame(&game{lcdImage: lcdImage, deviceImage: deviceImage}); err != nil {
		log.Fatal(err)
	}
}
