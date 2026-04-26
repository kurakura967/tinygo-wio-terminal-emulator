//go:build !tinygo

package initdisplay

import (
	"github.com/kurakura967/tinygo-wio-terminal-emulator/driver/ili9341"
	"github.com/kurakura967/tinygo-wio-terminal-emulator/machine"
)

// InitDisplay returns a pre-configured fake ili9341 device backed by Ebitengine.
func InitDisplay() *ili9341.Device {
	return ili9341.NewSPI(machine.SPI3, machine.LCD_DC, machine.LCD_SS_PIN, machine.LCD_RESET)
}
