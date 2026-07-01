package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// klutchTheme is the Klutch "Garage Dark" brand theme layered over Fyne's
// default. Colors are lifted verbatim from the klutch.lk web app design tokens
// (Signal Amber primary, teal for success, coral/red for danger) so the desktop
// agent matches the dashboard. Everything not overridden falls through to the
// base theme.
type klutchTheme struct{ base fyne.Theme }

func newKlutchTheme() fyne.Theme { return &klutchTheme{base: theme.DefaultTheme()} }

// Brand palette (klutch.lk design tokens). Exported to the package so components
// paint status dots / accents with the exact same values.
var (
	// Signal Amber — the primary brand color; dark ink sits on top of it.
	brandPrimary   = color.NRGBA{R: 0xFF, G: 0xC5, B: 0x3D, A: 0xFF} // #FFC53D
	brandOnPrimary = color.NRGBA{R: 0x16, G: 0x18, B: 0x1D, A: 0xFF} // #16181D

	// Teal — sync / success. Coral — credit due / danger. Variant-specific.
	successDark  = color.NRGBA{R: 0x2D, G: 0xD4, B: 0xBF, A: 0xFF} // #2DD4BF
	successLight = color.NRGBA{R: 0x0D, G: 0x94, B: 0x88, A: 0xFF} // #0D9488
	errorDark    = color.NRGBA{R: 0xFF, G: 0x6B, B: 0x5E, A: 0xFF} // #FF6B5E
	errorLight   = color.NRGBA{R: 0xD9, G: 0x2D, B: 0x20, A: 0xFF} // #D92D20

	// Dark variant surfaces.
	darkBackground = color.NRGBA{R: 0x0C, G: 0x0F, B: 0x14, A: 0xFF} // --color-bg
	darkSurface    = color.NRGBA{R: 0x0E, G: 0x11, B: 0x16, A: 0xFF} // --color-base (cards)
	darkControl    = color.NRGBA{R: 0x17, G: 0x1B, B: 0x22, A: 0xFF} // --color-surface
	darkForeground = color.NRGBA{R: 0xF2, G: 0xF4, B: 0xF7, A: 0xFF} // --color-ink
	darkSeparator  = color.NRGBA{R: 0x1C, G: 0x22, B: 0x2B, A: 0xFF} // --color-edge

	// Light variant surfaces.
	lightBackground = color.NRGBA{R: 0xF4, G: 0xF2, B: 0xEF, A: 0xFF} // --color-bg
	lightSurface    = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // --color-base
	lightControl    = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // --color-surface
	lightForeground = color.NRGBA{R: 0x16, G: 0x18, B: 0x1D, A: 0xFF} // --color-ink
	lightSeparator  = color.NRGBA{R: 0xE7, G: 0xE1, B: 0xD9, A: 0xFF} // --color-edge
)

// brandSuccess / brandError resolve to the right variant at call time.
func brandSuccess() color.NRGBA {
	if currentVariant() == theme.VariantDark {
		return successDark
	}
	return successLight
}

func brandError() color.NRGBA {
	if currentVariant() == theme.VariantDark {
		return errorDark
	}
	return errorLight
}

func (t *klutchTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	dark := v == theme.VariantDark
	switch n {
	case theme.ColorNamePrimary, theme.ColorNameHyperlink, theme.ColorNameFocus:
		return brandPrimary
	case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnWarning:
		return brandOnPrimary
	case theme.ColorNameSuccess:
		if dark {
			return successDark
		}
		return successLight
	case theme.ColorNameWarning:
		return brandPrimary
	case theme.ColorNameError:
		if dark {
			return errorDark
		}
		return errorLight
	case theme.ColorNameBackground:
		if dark {
			return darkBackground
		}
		return lightBackground
	case theme.ColorNameButton, theme.ColorNameInputBackground, theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground, theme.ColorNameHeaderBackground:
		if dark {
			return darkControl
		}
		return lightControl
	case theme.ColorNameForeground:
		if dark {
			return darkForeground
		}
		return lightForeground
	case theme.ColorNameSeparator, theme.ColorNameInputBorder:
		if dark {
			return darkSeparator
		}
		return lightSeparator
	case theme.ColorNameSelection:
		if dark {
			return color.NRGBA{R: 0xFF, G: 0xC5, B: 0x3D, A: 0x2E}
		}
		return color.NRGBA{R: 0xFF, G: 0xC5, B: 0x3D, A: 0x3D}
	}
	return t.base.Color(n, v)
}

func (t *klutchTheme) Font(s fyne.TextStyle) fyne.Resource { return t.base.Font(s) }

func (t *klutchTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return t.base.Icon(n) }

func (t *klutchTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNamePadding:
		return 5
	case theme.SizeNameInnerPadding:
		return 10
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 8
	case theme.SizeNameScrollBarRadius:
		return 4
	}
	return t.base.Size(n)
}
