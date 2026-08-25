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
	"slices"
	"strconv"
	"strings"

	"photo/internal/core/model"
)

const (
	dcNamespace        = "http://purl.org/dc/elements/1.1/"
	xmpNamespace       = "http://ns.adobe.com/xap/1.0/"
	iptcCoreNamespace  = "http://iptc.org/std/Iptc4xmpCore/1.0/xmlns/"
	photoshopNamespace = "http://ns.adobe.com/photoshop/1.0/"

	ratingProperty   = "Rating"
	locationProperty = "Location"
	cityProperty     = "City"
	stateProperty    = "State"
	countryProperty  = "Country"

	dcPrefix        = "dc"
	iptcCorePrefix  = "Iptc4xmpCore"
	photoshopPrefix = "photoshop"

	sidecarIndent  = "  "
	descriptionEnd = "</rdf:Description>"
	propertyDepth  = 2
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
	place       model.Place
	rating      int
	rated       bool
}

func (s sidecarTags) tags() model.Tags {
	title := s.title
	if len(title) == 0 {
		title = s.description
	}
	return model.Tags{Title: title, Keywords: s.keywords, Place: s.place}
}

// The properties the app reads back out of a document. Everything else - the
// develop settings of Lightroom above all - is another tool's and is left where
// it was found. The namespace is matched, never the prefix: a document is free
// to spell dc or photoshop as it likes, and free to mean something else by them.
type ownedProperty struct {
	space string
	local string
	list  bool
	read  func(*sidecarTags, xmpProperty)
}

var ownedProperties = []ownedProperty{
	{space: dcNamespace, local: "title", read: func(s *sidecarTags, p xmpProperty) { s.title = p.first() }},
	{space: dcNamespace, local: "description", read: func(s *sidecarTags, p xmpProperty) { s.description = p.first() }},
	{space: dcNamespace, local: "subject", list: true, read: func(s *sidecarTags, p xmpProperty) { s.keywords = p.all() }},
	{space: xmpNamespace, local: ratingProperty, read: func(s *sidecarTags, p xmpProperty) { s.setRating(p.Text) }},
	{space: iptcCoreNamespace, local: locationProperty, read: func(s *sidecarTags, p xmpProperty) { s.place.Location = p.first() }},
	{space: photoshopNamespace, local: cityProperty, read: func(s *sidecarTags, p xmpProperty) { s.place.City = p.first() }},
	{space: photoshopNamespace, local: stateProperty, read: func(s *sidecarTags, p xmpProperty) { s.place.State = p.first() }},
	{space: photoshopNamespace, local: countryProperty, read: func(s *sidecarTags, p xmpProperty) { s.place.Country = p.first() }},
}

func ownedByName(name xml.Name) (ownedProperty, bool) {
	for _, owned := range ownedProperties {
		if owned.space == name.Space && owned.local == name.Local {
			return owned, true
		}
	}
	return ownedProperty{}, false
}

// Simple writers put the properties on rdf:Description as attributes instead of
// child elements.
func (s *sidecarTags) readAttributes(attrs []xml.Attr) {
	for _, attr := range attrs {
		if owned, ok := ownedByName(attr.Name); ok {
			owned.read(s, owned.attributeValue(attr.Value))
		}
	}
}

// A list written as an attribute is the comma-separated form, which the element
// form spells as an rdf:Bag instead.
func (o ownedProperty) attributeValue(value string) xmpProperty {
	if o.list {
		return xmpProperty{Bag: model.ParseKeywordLine(value)}
	}
	return xmpProperty{Text: value}
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
		owned, ok := ownedByName(start.Name)
		if !ok {
			parsed.readAttributes(start.Attr)
			continue
		}
		var prop xmpProperty
		if err := decoder.DecodeElement(&prop, &start); err != nil {
			return sidecarTags{}, err
		}
		owned.read(&parsed, prop)
	}
}

// The properties the app writes, and the only ones it takes out of a document it
// did not write. The title goes to both dc:title and dc:description because
// stock sites disagree on which one carries the caption.
//
// Considered for microstock and left out on purpose, so nobody surveys them
// again: xmp:Label - colour labels stay in .photo-colors.json and reach no file;
// lr:hierarchicalSubject - Lightroom's own, the agencies ignore it and our
// keywords are flat anyway; xmp:CreateDate and photoshop:DateCreated - writing
// them would move the photo's date, which the app preserves everywhere else;
// GPS, photoshop:Headline, photoshop:Instructions,
// Iptc4xmpCore:CreatorContactInfo and plus:ModelReleaseStatus - no field in the
// app produces them; crs:* - Lightroom's develop settings, kept byte for byte.
type writtenProperty struct {
	prefix    string
	namespace string
	name      string
	values    func(model.Tags) []string
	emit      func(out *strings.Builder, depth int, name string, values []string)
}

func (w writtenProperty) qualified() string {
	return w.prefix + ":" + w.name
}

var writtenProperties = []writtenProperty{
	{prefix: dcPrefix, namespace: dcNamespace, name: "title", values: titleValues, emit: writeAltProperty},
	{prefix: dcPrefix, namespace: dcNamespace, name: "description", values: titleValues, emit: writeAltProperty},
	{prefix: dcPrefix, namespace: dcNamespace, name: "subject", values: keywordValues, emit: writeBagProperty},
	{prefix: iptcCorePrefix, namespace: iptcCoreNamespace, name: locationProperty,
		values: placeValues(func(p model.Place) string { return p.Location }), emit: writeTextProperty},
	{prefix: photoshopPrefix, namespace: photoshopNamespace, name: cityProperty,
		values: placeValues(func(p model.Place) string { return p.City }), emit: writeTextProperty},
	{prefix: photoshopPrefix, namespace: photoshopNamespace, name: stateProperty,
		values: placeValues(func(p model.Place) string { return p.State }), emit: writeTextProperty},
	{prefix: photoshopPrefix, namespace: photoshopNamespace, name: countryProperty,
		values: placeValues(func(p model.Place) string { return p.Country }), emit: writeTextProperty},
}

func titleValues(tags model.Tags) []string {
	if title := strings.TrimSpace(tags.Title); len(title) != 0 {
		return []string{title}
	}
	return nil
}

func keywordValues(tags model.Tags) []string {
	return tags.Keywords
}

func placeValues(level func(model.Place) string) func(model.Tags) []string {
	return func(tags model.Tags) []string {
		if value := level(tags.Place.Trimmed()); len(value) != 0 {
			return []string{value}
		}
		return nil
	}
}

// The prefixes the writer spells, in the order a description declares them. A
// document is free to bind the same namespace to a prefix of its own; that one
// is left alone, and ours is declared beside it.
type namespaceBinding struct {
	prefix      string
	declaration string
	bound       *regexp.Regexp
	anyBinding  *regexp.Regexp
}

var namespaceBindings = buildBindings()

func buildBindings() []namespaceBinding {
	var bindings []namespaceBinding
	for _, property := range writtenProperties {
		if slices.ContainsFunc(bindings, func(b namespaceBinding) bool { return b.prefix == property.prefix }) {
			continue
		}
		prefix := regexp.QuoteMeta(property.prefix)
		bindings = append(bindings, namespaceBinding{
			prefix:      property.prefix,
			declaration: `xmlns:` + property.prefix + `="` + property.namespace + `"`,
			bound: regexp.MustCompile(`xmlns:` + prefix + `\s*=\s*['"]` +
				regexp.QuoteMeta(property.namespace) + `['"]`),
			anyBinding: regexp.MustCompile(`xmlns:` + prefix + `\s*=`),
		})
	}
	return bindings
}

func writtenBindings(tags model.Tags) []namespaceBinding {
	written := make(map[string]bool, len(namespaceBindings))
	for _, property := range writtenProperties {
		if len(property.values(tags)) != 0 {
			written[property.prefix] = true
		}
	}
	var needed []namespaceBinding
	for _, binding := range namespaceBindings {
		if written[binding.prefix] {
			needed = append(needed, binding)
		}
	}
	return needed
}

var (
	// One pattern per property, never an alternation: RE2 has no backreferences,
	// so a single pattern lets the opening tag of one property be closed by the
	// closing tag of another and swallow every develop setting in between.
	emptyPropertyPatterns, elementPropertyPatterns, attrPropertyPatterns = stripPatterns()

	descriptionOpenPattern = regexp.MustCompile(`<rdf:Description\b[^>]*>`)

	// The slash is captured so one pass over the text sees both ends of every
	// nested element.
	descriptionTagPattern = regexp.MustCompile(`<(/?)rdf:Description\b[^>]*>`)

	rdfOpenPattern = regexp.MustCompile(`<rdf:RDF\b[^>]*>`)
)

// A sidecar written by Lightroom carries develop settings we must not lose, so
// only the properties we own are replaced. Regular expressions do the surgery
// because encoding/xml cannot round-trip namespace prefixes and would rewrite
// the whole document.
func mergeSidecar(existing []byte, tags model.Tags) ([]byte, error) {
	text := string(existing)
	if len(strings.TrimSpace(text)) == 0 {
		return []byte(newSidecar(tags)), nil
	}
	text = stripProperties(text)

	opened, err := openDescription(text, writtenBindings(tags))
	if errors.Is(err, errForeignPrefix) {
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

// The element the app writes, the self-closing form an earlier save may have
// left, and the attribute form exiftool writes - per property, matched by the
// prefix, which is all a regular expression has to go on.
func stripPatterns() (empty, element, attribute []*regexp.Regexp) {
	for _, property := range writtenProperties {
		name := regexp.QuoteMeta(property.qualified())
		empty = append(empty, regexp.MustCompile(`[ \t]*<`+name+`\b[^>]*/>[ \t]*\n?`))
		element = append(element, regexp.MustCompile(
			`(?s)[ \t]*<`+name+`\b(?:[^>]*[^/>])?>.*?</`+name+`>[ \t]*\n?`))
		attribute = append(attribute, regexp.MustCompile(`\s+`+name+`\s*=\s*("[^"]*"|'[^']*')`))
	}
	return empty, element, attribute
}

// The self-closing form goes first, so that an empty property left by an earlier
// save cannot stand in as the opening tag of the paired one below it.
func stripProperties(text string) string {
	for _, patterns := range [][]*regexp.Regexp{emptyPropertyPatterns, elementPropertyPatterns, attrPropertyPatterns} {
		for _, pattern := range patterns {
			text = pattern.ReplaceAllString(text, "")
		}
	}
	return text
}

type description struct {
	text      string
	bodyStart int
	indent    string
}

// The properties and the namespace declarations have to land in the same
// rdf:Description, and exiftool writes a self-closing one that carries every
// property as an attribute, so that form is expanded into an empty element
// first. Declaring a namespace on the element itself is redundant when an
// ancestor already declares it, which XML allows, and it is the only form that
// is right no matter where the sidecar came from. Only the namespaces this save
// actually writes into are declared, so a sidecar without a place keeps the one
// declaration it always had.
func openDescription(text string, bindings []namespaceBinding) (description, error) {
	at := descriptionOpenPattern.FindStringIndex(text)
	if at == nil {
		return description{}, errors.New("no rdf:Description element to update")
	}

	tag := text[at[0]:at[1]]
	var missing strings.Builder
	for _, binding := range bindings {
		if binding.bound.MatchString(tag) {
			continue
		}
		if binding.anyBinding.MatchString(tag) {
			return description{}, fmt.Errorf("%w: %s", errForeignPrefix, binding.prefix)
		}
		missing.WriteString(" " + binding.declaration)
	}
	if missing.Len() != 0 {
		tag = strings.Replace(tag, "<rdf:Description", "<rdf:Description"+missing.String(), 1)
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

var errForeignPrefix = errors.New("the rdf:Description binds a prefix the writer uses to another vocabulary")

// XML gives a prefix one meaning per element, so an rdf:Description that already
// spells dc - or photoshop, or Iptc4xmpCore - as a vocabulary of its own cannot
// carry ours beside it: a second declaration of the same prefix on one element
// is not a document any strict parser will read.
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
	block := "\n" + ownDescription(lineIndent(text, at[0])+sidecarIndent, writtenBindings(tags), properties)
	return []byte(text[:at[1]] + block + text[at[1]:]), nil
}

func ownDescription(indent string, bindings []namespaceBinding, properties string) string {
	return indent + descriptionOpenTag(bindings) + "\n" + properties + indent + descriptionEnd
}

func descriptionOpenTag(bindings []namespaceBinding) string {
	var tag strings.Builder
	tag.WriteString(`<rdf:Description rdf:about=""`)
	for _, binding := range bindings {
		tag.WriteString(" " + binding.declaration)
	}
	tag.WriteString(">")
	return tag.String()
}

// A description an earlier save appended declares only the namespaces its own
// properties needed, so every subset the writer can produce - in the order the
// table gives them - has to read as one of ours here.
func ownDescriptionPattern() string {
	pattern := regexp.QuoteMeta(`<rdf:Description rdf:about=""`)
	var patternSb497 strings.Builder
	for _, binding := range namespaceBindings {
		patternSb497.WriteString(`(?: ` + regexp.QuoteMeta(binding.declaration) + `)?`)
	}
	pattern += patternSb497.String()
	return pattern + ">"
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
  %s
%s  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
`

// The depth matches the one mergeSidecar writes, so reopening a sidecar we
// created and saving it again changes nothing.
func newSidecar(tags model.Tags) string {
	return fmt.Sprintf(sidecarTemplate,
		descriptionOpenTag(writtenBindings(tags)), sidecarProperties(tags, propertyDepth))
}

func sidecarProperties(tags model.Tags, depth int) string {
	var out strings.Builder
	for _, property := range writtenProperties {
		if values := property.values(tags); len(values) != 0 {
			property.emit(&out, depth, property.qualified(), values)
		}
	}
	return out.String()
}

// A place is plain text in XMP, where a caption is a language alternative and
// the keywords an unordered bag.
func writeTextProperty(out *strings.Builder, depth int, name string, values []string) {
	writeLine(out, depth, "<"+name+">"+escapeXML(values[0])+"</"+name+">")
}

func writeAltProperty(out *strings.Builder, depth int, name string, values []string) {
	writeLine(out, depth, "<"+name+">")
	writeLine(out, depth+1, "<rdf:Alt>")
	writeLine(out, depth+2, `<rdf:li xml:lang="x-default">`+escapeXML(values[0])+"</rdf:li>")
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
