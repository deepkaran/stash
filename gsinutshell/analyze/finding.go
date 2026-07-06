package analyze

// Severity ranks a finding.
type Severity int

const (
	SevInfo Severity = iota
	SevWarn
	SevFlag // matches a "flag if ..." rule in the spec
)

func (s Severity) String() string {
	switch s {
	case SevFlag:
		return "FLAG"
	case SevWarn:
		return "WARN"
	default:
		return "INFO"
	}
}

// Finding is one line in the health report.
type Finding struct {
	Section  string // report section, e.g. "Sizing/Memory"
	Category string // optional sub-heading within the section (e.g. "Fragmentation")
	Severity Severity
	Title    string
	Detail   string // human-readable value / explanation
}

// Report groups findings by section, preserving section order.
type Report struct {
	Sections []string
	byID     map[string][]Finding
}

func NewReport() *Report { return &Report{byID: map[string][]Finding{}} }

func (r *Report) Add(f Finding) {
	if _, ok := r.byID[f.Section]; !ok {
		r.Sections = append(r.Sections, f.Section)
	}
	r.byID[f.Section] = append(r.byID[f.Section], f)
}

// add is a convenience for an uncategorised finding.
func (r *Report) add(section string, sev Severity, title, detail string) {
	r.Add(Finding{Section: section, Severity: sev, Title: title, Detail: detail})
}

// addCat is a convenience for a categorised finding.
func (r *Report) addCat(section, category string, sev Severity, title, detail string) {
	r.Add(Finding{Section: section, Category: category, Severity: sev, Title: title, Detail: detail})
}

func (r *Report) In(section string) []Finding { return r.byID[section] }

// Categories returns the distinct categories within a section, in first-seen
// order. An empty string is included if any finding has no category.
func (r *Report) Categories(section string) []string {
	var cats []string
	seen := map[string]bool{}
	for _, f := range r.byID[section] {
		if !seen[f.Category] {
			seen[f.Category] = true
			cats = append(cats, f.Category)
		}
	}
	return cats
}

// InCategory returns the findings for a section/category pair, in order.
func (r *Report) InCategory(section, category string) []Finding {
	var out []Finding
	for _, f := range r.byID[section] {
		if f.Category == category {
			out = append(out, f)
		}
	}
	return out
}

// FlagCount returns the number of FLAG-severity findings.
func (r *Report) FlagCount() int {
	n := 0
	for _, fs := range r.byID {
		for _, f := range fs {
			if f.Severity == SevFlag {
				n++
			}
		}
	}
	return n
}
