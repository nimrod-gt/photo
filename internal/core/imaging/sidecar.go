package imaging

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"photo/internal/core/model"
)

const (
	dcNamespace     = "http://purl.org/dc/elements/1.1/"
	xmpNamespace    = "http://ns.adobe.com/xap/1.0/"
	ratingProperty  = "Rating"
	sidecarIndent   = "  "
	descriptionOpen = `<rdf:Description rdf:about="" xmlns:dc="` + dcNamespace + `">`
	descriptionEnd  = "</rdf:Description>"
	propertyDepth   = 2
)

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
	if bytes.Equal(updated, existing) {
		return nil
	}
	return replaceFile(path, updated)
}

type sidecarTags struct {
	title       string
	description string
	keywords    []string
	rating      int
	rated       bool
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
		switch {
		case attr.Name.Space == dcNamespace:
			s.readDCAttribute(attr.Name.Local, attr.Value)
		case isRatingName(attr.Name):
			s.setRating(attr.Value)
		}
	}
}

func (s *sidecarTags) readDCAttribute(name, value string) {
	switch name {
	case "title":
		s.title = strings.TrimSpace(value)
	case "description":
		s.description = strings.TrimSpace(value)
	case "subject":
		s.keywords = model.ParseKeywordLine(value)
	}
}

func (s *sidecarTags) setRating(text string) {
	if rating, ok := parseRating(text); ok {
		s.rating, s.rated = rating, true
	}
}

// Lightroom writes whole numbers, the camera a single digit; a fraction is
// accepted all the same and rounded, which is how Bridge shows it.
func parseRating(text string) (int, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return int(math.Round(value)), true
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
		rating := isRatingName(start.Name)
		if start.Name.Space != dcNamespace && !rating {
			parsed.readAttributes(start.Attr)
			continue
		}
		var prop xmpProperty
		if err := decoder.DecodeElement(&prop, &start); err != nil {
			return sidecarTags{}, err
		}
		if rating {
			parsed.setRating(prop.Text)
			continue
		}
		parsed.readElement(start.Name.Local, prop)
	}
}

func isRatingName(name xml.Name) bool {
	return name.Space == xmpNamespace && name.Local == ratingProperty
}

var dcProperties = []string{"title", "description", "subject"}

var (
	// One pattern per property, never an alternation: RE2 has no backreferences,
	// so a single pattern lets the opening tag of one property be closed by the
	// closing tag of another and swallow every develop setting in between.
	dcElementPatterns = elementPatterns()

	dcEmptyPattern = regexp.MustCompile(`[ \t]*<dc:(?:title|description|subject)\b[^>]*/>[ \t]*\n?`)
	dcAttrPattern  = regexp.MustCompile(`\s+dc:(?:title|description|subject)\s*=\s*("[^"]*"|'[^']*')`)

	descriptionOpenPattern = regexp.MustCompile(`<rdf:Description\b[^>]*>`)

	// The slash is captured so one pass over the text sees both ends of every
	// nested element.
	descriptionTagPattern = regexp.MustCompile(`<(/?)rdf:Description\b[^>]*>`)

	// The URI alone is not enough: another sidecar may bind it to a prefix of its
	// own, and the properties written below always spell the prefix dc.
	dcBindingPattern = regexp.MustCompile(`xmlns:dc\s*=\s*['"]` + regexp.QuoteMeta(dcNamespace) + `['"]`)

	anyDCBindingPattern = regexp.MustCompile(`xmlns:dc\s*=`)

	rdfOpenPattern = regexp.MustCompile(`<rdf:RDF\b[^>]*>`)
)

// A sidecar written by Lightroom carries develop settings we must not lose, so
// only the Dublin Core properties we own are replaced. Regular expressions do
// the surgery because encoding/xml cannot round-trip namespace prefixes and
// would rewrite the whole document.
func mergeSidecar(existing []byte, tags model.Tags) ([]byte, error) {
	text := string(existing)
	if len(strings.TrimSpace(text)) == 0 {
		return []byte(newSidecar(tags)), nil
	}
	text = stripProperties(text)

	opened, err := openDescription(text)
	if errors.Is(err, errForeignDC) {
		return insertDescription(text, tags)
	}
	if err != nil {
		return nil, err
	}
	cut, err := descriptionClose(opened.text, opened.bodyStart)
	if err != nil {
		return nil, err
	}
	return []byte(withProperties(opened, cut, sidecarProperties(tags, propertyDepth))), nil
}

// descriptionClose finds the close that pairs with the element openDescription
// opened, not the first one in the text. Camera Raw writes whole rdf:Description
// elements inside its structured values - crs:Look and its crs:Parameters among
// them - and properties written before their close would describe the preset
// instead of the photo, where no other tool looks for them.
func descriptionClose(text string, from int) (int, error) {
	depth := 0
	for _, at := range descriptionTagPattern.FindAllStringSubmatchIndex(text[from:], -1) {
		closing := at[2] != at[3]
		switch {
		case closing && depth == 0:
			return from + at[0], nil
		case closing:
			depth--
		case !strings.HasSuffix(text[from+at[0]:from+at[1]], "/>"):
			depth++
		}
	}
	return 0, errors.New("no rdf:Description element to update")
}

func elementPatterns() []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, 0, len(dcProperties))
	for _, name := range dcProperties {
		patterns = append(patterns, regexp.MustCompile(
			`(?s)[ \t]*<dc:`+name+`\b(?:[^>]*[^/>])?>.*?</dc:`+name+`>[ \t]*\n?`))
	}
	return patterns
}

// The self-closing form goes first, so that an empty property left by an earlier
// save cannot stand in as the opening tag of the paired one below it.
func stripProperties(text string) string {
	text = dcEmptyPattern.ReplaceAllString(text, "")
	for _, pattern := range dcElementPatterns {
		text = pattern.ReplaceAllString(text, "")
	}
	return dcAttrPattern.ReplaceAllString(text, "")
}

type description struct {
	text      string
	bodyStart int
	indent    string
}

// The properties and the namespace declaration have to land in the same
// rdf:Description, and exiftool writes a self-closing one that carries every
// property as an attribute, so that form is expanded into an empty element
// first. Declaring xmlns:dc on the element itself is redundant when an ancestor
// already declares it, which XML allows, and it is the only form that is right
// no matter where the sidecar came from.
func openDescription(text string) (description, error) {
	at := descriptionOpenPattern.FindStringIndex(text)
	if at == nil {
		return description{}, errors.New("no rdf:Description element to update")
	}

	tag := text[at[0]:at[1]]
	if !dcBindingPattern.MatchString(tag) {
		if anyDCBindingPattern.MatchString(tag) {
			return description{}, errForeignDC
		}
		tag = strings.Replace(tag, "<rdf:Description", `<rdf:Description xmlns:dc="`+dcNamespace+`"`, 1)
	}
	bodyStart := len(tag)
	if open, ok := strings.CutSuffix(tag, "/>"); ok {
		tag = open + ">"
		bodyStart = len(tag)
		tag += descriptionEnd
	}

	return description{
		text:      text[:at[0]] + tag + text[at[1]:],
		bodyStart: at[0] + bodyStart,
		indent:    lineIndent(text, at[0]),
	}, nil
}

var errForeignDC = errors.New("the rdf:Description binds dc to another vocabulary")

// XML gives a prefix one meaning per element, so an rdf:Description that already
// spells dc as a vocabulary of its own cannot carry ours beside it - a second
// xmlns:dc on the same element is not a document any strict parser will read.
// The properties go into an rdf:Description of their own instead, which is how
// XMP describes one resource across as many of them as it likes, and the foreign
// element is left exactly as it was found. Ours is written first, so the save
// after this one finds it and edits it in place.
func insertDescription(text string, tags model.Tags) ([]byte, error) {
	properties := sidecarProperties(tags, propertyDepth)
	if len(properties) == 0 {
		return []byte(text), nil
	}
	at := rdfOpenPattern.FindStringIndex(text)
	if at == nil {
		return nil, errors.New("no rdf:RDF element to update")
	}
	block := "\n" + dcDescription(lineIndent(text, at[0])+sidecarIndent, properties)
	return []byte(text[:at[1]] + block + text[at[1]:]), nil
}

func dcDescription(indent, properties string) string {
	return indent + descriptionOpen + "\n" + properties + indent + descriptionEnd
}

func withProperties(opened description, cut int, properties string) string {
	head := strings.TrimRight(opened.text[:cut], " \t")
	indent := opened.text[len(head):cut]
	if !strings.HasSuffix(head, "\n") {
		head += "\n"
		if len(indent) == 0 {
			indent = opened.indent
		}
	}
	return head + properties + indent + opened.text[cut:]
}

func lineIndent(text string, at int) string {
	return text[strings.LastIndex(text[:at], "\n")+1 : at]
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

// The depth matches the one mergeSidecar writes, so reopening a sidecar we
// created and saving it again changes nothing.
func newSidecar(tags model.Tags) string {
	return fmt.Sprintf(sidecarTemplate, sidecarProperties(tags, propertyDepth))
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
