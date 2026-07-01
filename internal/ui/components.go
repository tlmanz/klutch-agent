package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// mutedColor returns a low-emphasis foreground suitable for secondary text.
func mutedColor() color.Color {
	return theme.Color(theme.ColorNameForeground)
}

func currentVariant() fyne.ThemeVariant {
	return fyne.CurrentApp().Settings().ThemeVariant()
}

// surfaceColor is the card/background surface for the active theme variant.
func surfaceColor() color.Color {
	if currentVariant() == theme.VariantDark {
		return darkSurface
	}
	return lightSurface
}

func strokeColor() color.Color {
	if currentVariant() == theme.VariantDark {
		return darkSeparator
	}
	return lightSeparator
}

// card wraps content in a rounded, subtly bordered surface — the building block
// for every panel in the UI.
func card(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(surfaceColor())
	bg.CornerRadius = 12
	bg.StrokeColor = strokeColor()
	bg.StrokeWidth = 1
	return container.NewStack(bg, container.NewPadded(content))
}

// statusBadge is a pill with a colored dot and a status label, updated live from
// the agent snapshot.
type statusBadge struct {
	dot   *canvas.Circle
	label *canvas.Text
	bg    *canvas.Rectangle
	obj   fyne.CanvasObject
}

func newStatusBadge() *statusBadge {
	b := &statusBadge{
		dot:   canvas.NewCircle(brandPrimary),
		label: canvas.NewText("Starting…", mutedColor()),
		bg:    canvas.NewRectangle(color.NRGBA{A: 0}),
	}
	b.label.TextStyle = fyne.TextStyle{Bold: true}
	b.label.TextSize = 13
	b.bg.CornerRadius = 20
	b.bg.StrokeWidth = 1

	dotBox := container.NewGridWrap(fyne.NewSize(10, 10), b.dot)
	row := container.NewHBox(container.NewCenter(dotBox), b.label)
	b.obj = container.NewStack(b.bg, container.NewPadded(row))
	return b
}

// set repaints the badge for a status label and accent color.
func (b *statusBadge) set(text string, accent color.NRGBA) {
	b.dot.FillColor = accent
	b.label.Text = text
	b.label.Color = accent
	tint := accent
	tint.A = 0x1F
	b.bg.FillColor = tint
	stroke := accent
	stroke.A = 0x66
	b.bg.StrokeColor = stroke
	b.dot.Refresh()
	b.label.Refresh()
	b.bg.Refresh()
}

// statCard is a dashboard tile: a large value over a caption, with a colored
// accent stripe down the left edge.
type statCard struct {
	value   *canvas.Text
	caption *canvas.Text
	obj     fyne.CanvasObject
}

func newStatCard(caption string, accent color.NRGBA) *statCard {
	c := &statCard{
		value:   canvas.NewText("0", theme.Color(theme.ColorNameForeground)),
		caption: canvas.NewText(caption, mutedColor()),
	}
	c.value.TextStyle = fyne.TextStyle{Bold: true}
	c.value.TextSize = 30
	c.caption.TextSize = 12

	stripe := canvas.NewRectangle(accent)
	stripe.CornerRadius = 3
	stripeBox := container.NewGridWrap(fyne.NewSize(4, 40), stripe)

	body := container.NewVBox(c.value, c.caption)
	content := container.NewHBox(container.NewCenter(stripeBox), body)
	c.obj = card(content)
	return c
}

func (c *statCard) set(v string) {
	c.value.Text = v
	c.value.Refresh()
}

// sectionTitle renders a bold heading with an optional muted subtitle beneath it.
func sectionTitle(title, subtitle string) fyne.CanvasObject {
	t := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	if subtitle == "" {
		return t
	}
	sub := canvas.NewText(subtitle, mutedColor())
	sub.TextSize = 12
	return container.NewVBox(t, sub)
}
