package tui

import (
	"fmt"

	"github.com/FBakkensen/bc-insights-tui/logging"
)

// columnLayout captures computed column presentation details for tables.
// visible: headers actually rendered (no ellipsis column). If truncated is true
// an additional synthetic column ("(+N)") should be appended by caller.
type columnLayout struct {
	headers     []string
	visible     []string
	hiddenCount int
	truncated   bool
	dataCols    int
	colWidth    int
}

// computeColumnLayout determines how many headers fit in totalWidth given minColWidth.
// If space is insufficient for all headers, it truncates and reserves one column for ellipsis
// only when at least one data column can remain; otherwise it shows the first header only
// without ellipsis (favor data over meta indication). totalWidth <= 0 defaults to 80.
func computeColumnLayout(headers []string, totalWidth, minColWidth int) columnLayout {
	if minColWidth <= 0 {
		minColWidth = 10
	}
	if totalWidth <= 0 {
		totalWidth = 80
	}
	if len(headers) == 0 {
		return columnLayout{headers: headers}
	}
	maxFit := totalWidth / minColWidth
	if maxFit < 1 {
		maxFit = 1
	}
	if len(headers) <= maxFit { // all fit
		colWidth := totalWidth / len(headers)
		if colWidth < minColWidth {
			colWidth = minColWidth
		}
		return columnLayout{headers: headers, visible: headers, hiddenCount: 0, truncated: false, dataCols: len(headers), colWidth: colWidth}
	}
	if maxFit == 1 { // cannot show ellipsis meaningfully
		colWidth := totalWidth
		logging.Info("Column layout severely constrained", "totalWidth", fmt.Sprintf("%d", totalWidth), "shown", "1", "hidden", fmt.Sprintf("%d", len(headers)-1))
		return columnLayout{headers: headers, visible: headers[:1], hiddenCount: len(headers) - 1, truncated: true, dataCols: 1, colWidth: colWidth}
	}
	dataCols := maxFit - 1
	if dataCols > len(headers) {
		dataCols = len(headers)
	}
	hidden := len(headers) - dataCols
	colWidth := totalWidth / maxFit
	if colWidth < minColWidth {
		colWidth = minColWidth
	}
	return columnLayout{headers: headers, visible: headers[:dataCols], hiddenCount: hidden, truncated: true, dataCols: dataCols, colWidth: colWidth}
}
