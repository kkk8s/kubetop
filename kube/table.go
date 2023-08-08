package kube

import (
	"os"

	"github.com/olekukonko/tablewriter"
)

type table struct {
	*tablewriter.Table
}

func NewTable() *table {
	table := &table{
		Table: tablewriter.NewWriter(os.Stdout),
	}
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("\t") // pad with tabs
	table.SetNoWhiteSpace(true)

	return table
}

func (t *table) SetHeader(header []string) {
	t.Table.SetHeader(header)
}