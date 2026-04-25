//go:build !tinygo

package ili9341

import (
	"image/color"

	"github.com/kurakura967/tinygo-wio-terminal-emulator/emulator"
	"github.com/kurakura967/tinygo-wio-terminal-emulator/machine"
)

// Rotation represents display rotation.
type Rotation uint8

const Rotation270 Rotation = 3

// Config holds display configuration (unused in emulator).
type Config struct{}

// Device is the fake ili9341 display that forwards drawing calls to Ebitengine.
type Device struct{}

// NewSPI creates a new fake ili9341 device. Arguments are accepted for API
// compatibility with the real driver but are ignored.
func NewSPI(bus machine.SPI, dc, cs, rst machine.Pin) *Device {
	return &Device{}
}

func (d *Device) Configure(c Config) {}

func (d *Device) SetRotation(r Rotation) {}

// SetPixel satisfies the tinyfont.Displayer interface.
func (d *Device) SetPixel(x, y int16, c color.RGBA) {
	emulator.GlobalScreen.DrawPixel(x, y, c)
}

func (d *Device) DrawPixel(x, y int16, c color.RGBA) {
	emulator.GlobalScreen.DrawPixel(x, y, c)
}

func (d *Device) FillRectangle(x, y, w, h int16, c color.RGBA) {
	emulator.GlobalScreen.FillRectangle(x, y, w, h, c)
}

func (d *Device) FillScreen(c color.RGBA) {
	emulator.GlobalScreen.FillRectangle(0, 0, emulator.ScreenWidth, emulator.ScreenHeight, c)
}

// Display satisfies the drivers.Displayer interface (no-op for emulator).
func (d *Device) Display() error { return nil }

// Size returns the display dimensions.
func (d *Device) Size() (x, y int16) {
	return emulator.ScreenWidth, emulator.ScreenHeight
}
