//go:build !tinygo

package machine

import "github.com/kurakura967/tinygo-wio-terminal-emulator/emulator"

// SPI is a no-op SPI bus stub.
type SPI struct{}

type SPIConfig struct {
	SCK       Pin
	SDO       Pin
	SDI       Pin
	Frequency uint32
}

func (s SPI) Configure(c SPIConfig) {}
func (s SPI) Tx(w, r []byte) error  { return nil }

var SPI3 SPI

// Pin is a GPIO pin stub.
type Pin uint8

type PinMode uint8

const (
	PinInput       PinMode = 0
	PinInputPullup PinMode = 1
	PinOutput      PinMode = 2
)

type PinConfig struct {
	Mode PinMode
}

func (p Pin) Configure(c PinConfig) {}
func (p Pin) High()                 {}
func (p Pin) Low()                  {}

// Get returns the current state of the pin.
// Wio Terminal buttons are active-low: Get() returns false when pressed.
func (p Pin) Get() bool {
	switch p {
	case WIO_KEY_A:
		return !emulator.Buttons.A
	case WIO_KEY_B:
		return !emulator.Buttons.B
	case WIO_KEY_C:
		return !emulator.Buttons.C
	case WIO_5S_UP:
		return !emulator.Buttons.Up
	case WIO_5S_DOWN:
		return !emulator.Buttons.Down
	case WIO_5S_LEFT:
		return !emulator.Buttons.Left
	case WIO_5S_RIGHT:
		return !emulator.Buttons.Right
	case WIO_5S_PRESS:
		return !emulator.Buttons.Center
	}
	return true // default: not pressed
}

// LCD pin constants.
const (
	LCD_SCK_PIN   Pin = 0
	LCD_SDO_PIN   Pin = 1
	LCD_SDI_PIN   Pin = 2
	LCD_DC        Pin = 3
	LCD_SS_PIN    Pin = 4
	LCD_RESET     Pin = 5
	LCD_BACKLIGHT Pin = 6
)

// Wio Terminal button and joystick pin constants.
const (
	WIO_KEY_A  Pin = 10
	WIO_KEY_B  Pin = 11
	WIO_KEY_C  Pin = 12
	WIO_5S_UP    Pin = 13
	WIO_5S_DOWN  Pin = 14
	WIO_5S_LEFT  Pin = 15
	WIO_5S_RIGHT Pin = 16
	WIO_5S_PRESS Pin = 17
)
