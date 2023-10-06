package terminal

import "github.com/fatih/color"

var (
	colorHeader      = color.New(color.Bold)
	colorDebug       = color.New(color.FgHiBlue)
	colorInfo        = color.New()
	colorError       = color.New(color.FgRed)
	colorErrorBold   = color.New(color.FgRed, color.Bold)
	colorSuccess     = color.New(color.FgGreen)
	colorSuccessBold = color.New(color.FgGreen, color.Bold)
	colorTrace       = color.New(color.FgCyan)
	colorWarning     = color.New(color.FgYellow)
	colorWarningBold = color.New(color.FgYellow, color.Bold)
)
