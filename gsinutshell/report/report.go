package report

import (
	"fmt"
	"io"
	"strings"

	"gsinutshell/analyze"
	"gsinutshell/model"
)

// WriteText renders the report as plain text to w.
func WriteText(w io.Writer, m *model.Model, r *analyze.Report) {
	fmt.Fprintln(w, "================================================================")
	fmt.Fprintln(w, " GSI Nutshell — Health Report")
	fmt.Fprintln(w, "================================================================")
	fmt.Fprintf(w, " Source       : %s\n", m.Source)
	fmt.Fprintf(w, " Sample window: last %d samples per series\n", m.SampleWindow)
	fmt.Fprintf(w, " Flags raised : %d\n", r.FlagCount())
	fmt.Fprintln(w)

	for _, sec := range r.Sections {
		fmt.Fprintf(w, "## %s\n", sec)
		cats := r.Categories(sec)
		for _, cat := range cats {
			if cat != "" {
				fmt.Fprintf(w, "  ### %s\n", cat)
			}
			for _, f := range r.InCategory(sec, cat) {
				writeFinding(w, f, cat != "")
			}
		}
		fmt.Fprintln(w)
	}
}

func writeFinding(w io.Writer, f analyze.Finding, indented bool) {
	marker := "   "
	if f.Severity == analyze.SevFlag {
		marker = ">> "
	} else if f.Severity == analyze.SevWarn {
		marker = " ! "
	}
	pad := ""
	if indented {
		pad = "  "
	}
	// Multi-line detail (e.g. usage top-N): print the title, then each detail
	// line on its own indented row.
	if strings.Contains(f.Detail, "\n") {
		fmt.Fprintf(w, "%s%s[%-4s] %s\n", pad, marker, f.Severity, f.Title)
		for _, line := range strings.Split(f.Detail, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			fmt.Fprintf(w, "%s          %s\n", pad, line)
		}
		return
	}
	fmt.Fprintf(w, "%s%s[%-4s] %-40s %s\n", pad, marker, f.Severity, f.Title, f.Detail)
}
