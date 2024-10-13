package ninjacrawler

import (
	"github.com/playwright-community/playwright-go"
)

func autoMoveMouse(page playwright.Page) error {
	var err error
	// Get viewport size to know the page dimensions
	viewportSize := page.ViewportSize()
	width := viewportSize.Width
	height := viewportSize.Height

	// Move the mouse from top-left to bottom-right, simulating a sweeping motion
	steps := 20 // Define how many steps the mouse should take (can adjust for smoother or faster movement)
	for y := 0; y <= height; y += height / steps {
		for x := 0; x <= width; x += width / steps {
			err = page.Mouse().Move(float64(x), float64(y))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
