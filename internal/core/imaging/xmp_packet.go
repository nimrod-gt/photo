package imaging

import (
	"bytes"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"photo/internal/core/model"
)

const (
	// Sony pads its packets with lines of a hundred spaces, the shape the XMP
	// specification suggests, so clearing the tags hands the camera its own
	// bytes back.
	paddingLineWidth = 100
	rdfEnd           = "</rdf:RDF>"

	ratingOpen            = "<xmp:Rating>"
	ratingClose           = "</xmp:Rating>"
	ratingDescriptionOpen = `<rdf:Description rdf:about="" xmlns:xmp="` + xmpNamespace + `">`
)

var (
	xpacketEndPattern = regexp.MustCompile(`<\?xpacket end=['"]([wr])['"]\?>`)

	// The description an earlier save appended is empty once its properties are
	// stripped, and is dropped whole so the packet returns to what the camera
	// wrote.
	emptyDescriptionPattern = regexp.MustCompile(
		`[ \t]*` + regexp.QuoteMeta(descriptionOpen) + `\s*` + descriptionEnd + `[ \t]*\n?`)

	// The element the camera writes, its self-closing form, and the attribute
	// form Lightroom writes. Only the xmp prefix is rewritten - the one every
	// writer spells - and a rating under another prefix makes the whole write
	// fail, so that no second rating is ever added beside it.
	ratingElementPattern = regexp.MustCompile(`<xmp:Rating\b(?:[^>]*[^/>])?>[^<]*</xmp:Rating>|<xmp:Rating\s*/>`)
	ratingAttrPattern    = regexp.MustCompile(`\sxmp:Rating\s*=\s*(?:"[^"]*"|'[^']*')`)
)

// packetWithTags rewrites an XMP packet embedded in a JPEG without changing its
// length: the properties take the place of the whitespace padding behind the
// document. What does not fit there is not written here at all, and the caller
// falls back to the EXIF. Everything in front of the appended description keeps
// its offset, the rating the camera wrote included.
func packetWithTags(packet []byte, tags model.Tags) ([]byte, bool) {
	return rewritePacket(packet, func(document string) (string, bool) {
		return appendDescription(document, tags)
	})
}

// packetWithRating sets xmp:Rating the way the camera does it: the digit
// changes where it stands, and nothing else in the packet moves.
func packetWithRating(packet []byte, rating int) ([]byte, bool) {
	return rewritePacket(packet, func(document string) (string, bool) {
		return documentWithRating(document, rating)
	})
}

// rewritePacket hands the document in front of the padding to update and fits
// the result back into the same number of bytes. Only a packet whose trailer
// allows updates in place is touched.
func rewritePacket(packet []byte, update func(document string) (string, bool)) ([]byte, bool) {
	text := string(packet)
	trailer := xpacketEndPattern.FindStringSubmatchIndex(text)
	if trailer == nil || text[trailer[2]:trailer[3]] != "w" {
		return nil, false
	}
	document := strings.TrimRight(text[:trailer[0]], " \t\r\n")
	content, ok := update(document)
	if !ok {
		return nil, false
	}
	// Nothing to add and nothing to remove: the packet is kept byte for byte,
	// its own padding included, rather than re-padded for no reason.
	if content == document {
		return packet, true
	}
	// The same length means only the changed bytes move and the padding stays
	// whatever the camera wrote, so a rating flip is a single byte.
	if len(content) == len(document) {
		return slices.Concat([]byte(content), packet[len(document):]), true
	}
	room := trailer[0] - len(content)
	if room < 0 {
		return nil, false
	}
	return slices.Concat([]byte(content), xmpPadding(room), packet[trailer[0]:]), true
}

// appendDescription puts the properties into an rdf:Description of their own at
// the end of rdf:RDF, behind whatever the camera or another tool wrote, once
// the ones an earlier save left are removed. Empty tags leave only the removal.
func appendDescription(content string, tags model.Tags) (string, bool) {
	content = emptyDescriptionPattern.ReplaceAllString(stripProperties(content), "")
	if !strings.Contains(content, rdfEnd) {
		return "", false
	}
	properties := sidecarProperties(tags, propertyDepth)
	if len(properties) == 0 {
		return content, true
	}
	return appendElement(content, func(indent string) string {
		return dcDescription(indent, properties)
	})
}

// documentWithRating rewrites the rating where it already stands - as the
// camera's element or Lightroom's attribute - and only a document without one
// gets a description of its own. Zero is written out as well, since a rating
// that is merely absent would let an old EXIF rating speak instead.
//
// What comes back is read again the way the app itself reads it: the patterns
// rewrite text while the reader parses XML, and where the two disagree - a
// rating the document carries twice, or one sitting in a comment - the file
// would be patched where nothing reads it and the toggle would never move
// again. Such a packet is left alone instead.
func documentWithRating(document string, rating int) (string, bool) {
	updated, ok := documentWithRatingWritten(document, rating)
	if !ok {
		return "", false
	}
	parsed, err := parseSidecar([]byte(updated))
	if err != nil || !parsed.rated || parsed.rating != rating {
		return "", false
	}
	return updated, true
}

func documentWithRatingWritten(document string, rating int) (string, bool) {
	value := strconv.Itoa(rating)
	if ratingElementPattern.MatchString(document) {
		return ratingElementPattern.ReplaceAllStringFunc(document, func(element string) string {
			return withRatingElement(element, value)
		}), true
	}
	if ratingAttrPattern.MatchString(document) {
		return ratingAttrPattern.ReplaceAllStringFunc(document, func(attribute string) string {
			return withRatingAttribute(attribute, value)
		}), true
	}
	// A rating these patterns do not describe - one under another prefix, say -
	// would still be read, and a second one appended here would race it, so the
	// packet is left alone instead.
	if parsed, err := parseSidecar([]byte(document)); err != nil || parsed.rated {
		return "", false
	}
	return appendElement(document, func(indent string) string {
		return indent + ratingDescriptionOpen + "\n" +
			indent + sidecarIndent + ratingOpen + value + ratingClose + "\n" +
			indent + descriptionEnd
	})
}

// Only the text between the tags is replaced, so an xml:lang or any other
// attribute the element carries survives the write.
func withRatingElement(element, value string) string {
	open, _, _ := strings.Cut(element, ">")
	return strings.TrimRight(open, "/ \t") + ">" + value + ratingClose
}

func withRatingAttribute(attribute, value string) string {
	quoteAt := strings.IndexAny(attribute, `"'`)
	quote := attribute[quoteAt : quoteAt+1]
	return attribute[:quoteAt] + quote + value + quote
}

// appendElement puts the element in front of the closing rdf:RDF tag, indented
// one level deeper than it.
func appendElement(content string, element func(indent string) string) (string, bool) {
	closeAt := strings.LastIndex(content, rdfEnd)
	if closeAt < 0 {
		return "", false
	}
	head := strings.TrimRight(content[:closeAt], " \t")
	indent := content[len(head):closeAt]
	if !strings.HasSuffix(head, "\n") {
		head += "\n"
	}
	return head + element(indent+sidecarIndent) + "\n" + indent + content[closeAt:], true
}

func xmpPadding(size int) []byte {
	padding := bytes.Repeat([]byte{' '}, size)
	for at := 0; at < size; at += paddingLineWidth + 1 {
		padding[at] = '\n'
	}
	if size > 0 {
		padding[size-1] = '\n'
	}
	return padding
}
