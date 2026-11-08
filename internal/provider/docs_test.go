package provider

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// The documentation is published on the Terraform Registry, which serves the
// overview at .../docs without a trailing slash and does not resolve .md
// links: a relative link breaks there or on GitHub. Pages link each other with
// absolute registry URLs, checked here against the generated files.
const registryDocs = "https://registry.terraform.io/providers/workplace-as-code/msexchangeserver/latest/docs"

var markdownLink = regexp.MustCompile(`\]\(([^)\s]+)\)`)

func TestDocsLinks(t *testing.T) {
	pages, err := filepath.Glob("../../docs/*.md")
	if err != nil {
		t.Fatal(err)
	}
	more, err := filepath.Glob("../../docs/*/*.md")
	if err != nil {
		t.Fatal(err)
	}
	pages = append(pages, more...)
	if len(pages) == 0 {
		t.Fatal("no documentation page found")
	}
	for _, page := range pages {
		data, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimPrefix(page, "../../")
		for _, m := range markdownLink.FindAllStringSubmatch(string(data), -1) {
			link := m[1]
			switch {
			case strings.HasPrefix(link, registryDocs):
				target, _, _ := strings.Cut(strings.TrimPrefix(link, registryDocs), "#")
				file := "index"
				if target != "" {
					file = strings.TrimPrefix(target, "/")
				}
				if _, err := os.Stat("../../docs/" + file + ".md"); err != nil {
					t.Errorf("%s: %s points to a page that does not exist (docs/%s.md)", name, link, file)
				}
			case strings.HasPrefix(link, "http://"), strings.HasPrefix(link, "https://"),
				strings.HasPrefix(link, "mailto:"), strings.HasPrefix(link, "#"):
			default:
				t.Errorf("%s: relative link %q: use %s/...", name, link, registryDocs)
			}
		}
	}
}

// maxCodeLine is the widest code line the Registry shows: it does not scroll
// code blocks, so a longer line runs past the block and is cut off.
const maxCodeLine = 100

func TestDocsCodeLines(t *testing.T) {
	pages, err := filepath.Glob("../../docs/*.md")
	if err != nil {
		t.Fatal(err)
	}
	more, err := filepath.Glob("../../docs/*/*.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range append(pages, more...) {
		data, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		inCode := false
		for i, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inCode = !inCode
				continue
			}
			if inCode && utf8.RuneCountInString(line) > maxCodeLine {
				t.Errorf("%s:%d: code line of %d characters, split it to %d at most",
					strings.TrimPrefix(page, "../../"), i+1, utf8.RuneCountInString(line), maxCodeLine)
			}
		}
	}
}

// docNames returns the type names documented in docs/<kind>, for example
// msexchangeserver_transport_rule for docs/resources/transport_rule.md.
func docNames(t *testing.T, kind string) []string {
	t.Helper()
	pages, err := filepath.Glob("../../docs/" + kind + "/*.md")
	if err != nil || len(pages) == 0 {
		t.Fatalf("no page in docs/%s: %v", kind, err)
	}
	names := make([]string, len(pages))
	for i, p := range pages {
		names[i] = "msexchangeserver_" + strings.TrimSuffix(filepath.Base(p), ".md")
	}
	return names
}

// mentions reports whether text names typeName as a whole word: the name of
// msexchangeserver_safe_links_rule does not count for msexchangeserver_safe_links.
func mentions(text, typeName string) bool {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(typeName) + `\b`).MatchString(text)
}

// section returns the text of a "## <title>" section of a Markdown page.
func section(t *testing.T, page, title string) string {
	t.Helper()
	start := strings.Index(page, "\n## "+title+"\n")
	if start < 0 {
		t.Fatalf("no section %q", title)
	}
	rest := page[start+len(title)+5:]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		rest = rest[:end]
	}
	return rest
}

// TestDocsListEveryResource keeps the README and the availability guide in
// step with the provider: both list every resource, and the data sources that
// have no resource. The guide also names, in its data source section, the
// resources that have no data source.
func TestDocsListEveryResource(t *testing.T) {
	data, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := string(data)
	if data, err = os.ReadFile("../../templates/guides/availability.md"); err != nil {
		t.Fatal(err)
	}
	guide := string(data)
	dataSourcesSection := section(t, guide, "Data sources")
	resources, dataSources := map[string]bool{}, map[string]bool{}
	for _, name := range docNames(t, "resources") {
		resources[name] = true
		if !mentions(readme, name) {
			t.Errorf("README.md does not list the resource %s", name)
		}
		if !mentions(guide, name) {
			t.Errorf("templates/guides/availability.md does not list the resource %s", name)
		}
	}
	for _, name := range docNames(t, "data-sources") {
		dataSources[name] = true
		if resources[name] {
			continue
		}
		if !mentions(readme, name) {
			t.Errorf("README.md does not list the data source %s, which has no resource", name)
		}
		if !mentions(dataSourcesSection, name) {
			t.Errorf("the data sources of the availability guide do not list %s, which has no resource", name)
		}
	}
	for name := range resources {
		if !dataSources[name] && !mentions(dataSourcesSection, name) {
			t.Errorf("the data sources of the availability guide do not say that %s has no data source", name)
		}
	}
}
