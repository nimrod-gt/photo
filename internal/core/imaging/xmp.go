package imaging

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"photo/internal/core/model"
)

const (
	dcNamespace    = "http://purl.org/dc/elements/1.1/"
	sidecarExt     = ".xmp"
	sidecarIndent  = "  "
	descriptionEnd = "</rdf:Description>"
)

// A RAW file cannot carry EXIF written by us, so its metadata goes into an XMP
// sidecar named after the RAW file - the convention Lightroom and Bridge use.
func SidecarPath(rawPath string) string {
	return strings.TrimSuffix(rawPath, filepath.Ext(rawPath)) + sidecarExt
}

func ReadSidecar(path string) (model.Tags, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return model.Tags{}, nil
		}
		return model.Tags{}, fmt.Errorf("reading sidecar %s: %w", path, err)
	}
	parsed, err := parseSidecar(data)
	if err != nil {
		return model.Tags{}, fmt.Errorf("parsing sidecar %s: %w", path, err)
	}
	return parsed.tags(), nil
}

func WriteSidecar(path string, tags model.Tags) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading sidecar %s: %w", path, err)
	}
	updated, err := mergeSidecar(existing, tags)
	if err != nil {
		return fmt.Errorf("updating sidecar %s: %w", path, err)
	}
	return replaceFile(path, updated)
}

type sidecarTags struct {
	title       string
	description string
	keywords    []string
}

func (s sidecarTags) tags() model.Tags {
	title := s.title
	if len(title) == 0 {
		title = s.description
	}
	return model.Tags{Title: title, Keywords: s.keywords}
}

func (s *sidecarTags) readElement(name string, prop xmpProperty) {
	switch name {
	case "title":
		s.title = prop.first()
	case "description":
		s.description = prop.first()
	case "subject":
		s.keywords = prop.all()
	}
}

// Simple writers put the properties on rdf:Description as attributes instead of
// child elements.
func (s *sidecarTags) readAttributes(attrs []xml.Attr) {
	for _, attr := range attrs {
		if attr.Name.Space != dcNamespace {
			continue
		}
		switch attr.Name.Local {
		case "title":
			s.title = strings.TrimSpace(attr.Value)
		case "description":
			s.description = strings.TrimSpace(attr.Value)
		case "subject":
			s.keywords = model.ParseKeywordLine(attr.Value)
		}
	}
}

type xmpProperty struct {
	Text string   `xml:",chardata"`
	Alt  []string `xml:"Alt>li"`
	Bag  []string `xml:"Bag>li"`
	Seq  []string `xml:"Seq>li"`
}

func (p xmpProperty) first() string {
	if values := p.all(); len(values) != 0 {
		return values[0]
	}
	return ""
}

func (p xmpProperty) all() []string {
	for _, list := range [][]string{p.Alt, p.Bag, p.Seq} {
		if len(list) != 0 {
			return trimmed(list)
		}
	}
	if text := strings.TrimSpace(p.Text); len(text) != 0 {
		return []string{text}
	}
	return nil
}

func trimmed(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value := strings.TrimSpace(value); len(value) != 0 {
			result = append(result, value)
		}
	}
	return result
}

func parseSidecar(data []byte) (sidecarTags, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var parsed sidecarTags
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return parsed, nil
		}
		if err != nil {
			return sidecarTags{}, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Space != dcNamespace {
			parsed.readAttributes(start.Attr)
			continue
		}
		var prop xmpProperty
		if err := decoder.DecodeElement(&prop, &start); err != nil {
			return sidecarTags{}, err
		}
		parsed.readElement(start.Name.Local, prop)
	}
}

var (
	dcElementPattern = regexp.MustCompile(
		`(?s)[ \t]*<dc:(?:title|description|subject)\b.*?</dc:(?:title|description|subject)>[ \t]*\n?`)
	dcEmptyPattern = regexp.MustCompile(`[ \t]*<dc:(?:title|description|subject)\b[^>]*/>[ \t]*\n?`)
	dcAttrPattern  = regexp.MustCompile(`\s+dc:(?:title|description|subject)\s*=\s*("[^"]*"|'[^']*')`)
)

// A sidecar written by Lightroom carries develop settings we must not lose, so
// only the Dublin Core properties we own are replaced.
func mergeSidecar(existing []byte, tags model.Tags) ([]byte, error) {
	text := string(existing)
	if len(strings.TrimSpace(text)) == 0 {
		return []byte(newSidecar(tags)), nil
	}
	if !strings.Contains(text, descriptionEnd) {
		return nil, errors.New("no rdf:Description element to update")
	}
	text = dcElementPattern.ReplaceAllString(text, "")
	text = dcEmptyPattern.ReplaceAllString(text, "")
	text = dcAttrPattern.ReplaceAllString(text, "")
	text = ensureDCNamespace(text)

	cut := strings.Index(text, descriptionEnd)
	if cut < 0 {
		return nil, errors.New("no rdf:Description element to update")
	}
	head := strings.TrimRight(text[:cut], " \t")
	indent := text[len(head):cut]
	if !strings.HasSuffix(head, "\n") {
		head += "\n"
	}
	return []byte(head + sidecarProperties(tags, 2) + indent + text[cut:]), nil
}

func ensureDCNamespace(text string) string {
	if strings.Contains(text, dcNamespace) {
		return text
	}
	return strings.Replace(text, "<rdf:Description", `<rdf:Description xmlns:dc="`+dcNamespace+`"`, 1)
}

const sidecarTemplate = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:dc="` + dcNamespace + `">
%s  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
`

func newSidecar(tags model.Tags) string {
	return fmt.Sprintf(sidecarTemplate, sidecarProperties(tags, 3))
}

// The title is written to both dc:title and dc:description because stock sites
// disagree on which one carries the caption.
func sidecarProperties(tags model.Tags, depth int) string {
	var out strings.Builder
	if title := strings.TrimSpace(tags.Title); len(title) != 0 {
		writeAltProperty(&out, depth, "dc:title", title)
		writeAltProperty(&out, depth, "dc:description", title)
	}
	if len(tags.Keywords) != 0 {
		writeBagProperty(&out, depth, "dc:subject", tags.Keywords)
	}
	return out.String()
}

func writeAltProperty(out *strings.Builder, depth int, name, value string) {
	writeLine(out, depth, "<"+name+">")
	writeLine(out, depth+1, "<rdf:Alt>")
	writeLine(out, depth+2, `<rdf:li xml:lang="x-default">`+escapeXML(value)+"</rdf:li>")
	writeLine(out, depth+1, "</rdf:Alt>")
	writeLine(out, depth, "</"+name+">")
}

func writeBagProperty(out *strings.Builder, depth int, name string, values []string) {
	writeLine(out, depth, "<"+name+">")
	writeLine(out, depth+1, "<rdf:Bag>")
	for _, value := range values {
		writeLine(out, depth+2, "<rdf:li>"+escapeXML(value)+"</rdf:li>")
	}
	writeLine(out, depth+1, "</rdf:Bag>")
	writeLine(out, depth, "</"+name+">")
}

func writeLine(out *strings.Builder, depth int, text string) {
	out.WriteString(strings.Repeat(sidecarIndent, depth) + text + "\n")
}

var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")

func escapeXML(text string) string {
	return xmlEscaper.Replace(text)
}
