package imaging

import (
	"bytes"
	"regexp"
	"slices"
	"strings"

	"photo/internal/core/model"
)

const (
	// Sony pads its packets with lines of a hundred spaces, the shape the XMP
	// specification suggests, so clearing the tags hands the camera its own
	// bytes back.
	paddingLineWidth = 100
	rdfEnd           = "</rdf:RDF>"
)

var (
	xpacketEndPattern = regexp.MustCompile(`<\?xpacket end=['"]([wr])['"]\?>`)

	// The description an earlier save appended is empty once its properties are
	// stripped, and is dropped whole so the packet returns to what the camera
	// wrote.
	emptyDescriptionPattern = regexp.MustCompile(
		`[ \t]*` + regexp.QuoteMeta(descriptionOpen) + `\s*` + descriptionEnd + `[ \t]*\n?`)
)

// packetWithTags rewrites an XMP packet embedded in a JPEG without changing its
// length: the properties take the place of the whitespace padding behind the
// document. What does not fit there is not written here at all, and the caller
// falls back to the EXIF. Only a packet whose trailer allows updates in place
// is touched, and everything in front of the appended description keeps its
// offset, the rating the camera wrote included.
func packetWithTags(packet []byte, tags model.Tags) ([]byte, bool) {
	text := string(packet)
	trailer := xpacketEndPattern.FindStringSubmatchIndex(text)
	if trailer == nil || text[trailer[2]:trailer[3]] != "w" {
		return nil, false
	}
	document := strings.TrimRight(text[:trailer[0]], " \t\r\n")
	content, ok := appendDescription(document, tags)
	if !ok {
		return nil, false
	}
	// Nothing to add and nothing to remove: the packet is kept byte for byte,
	// its own padding included, rather than re-padded for no reason.
	if content == document {
		return packet, true
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
	closeAt := strings.LastIndex(content, rdfEnd)
	if closeAt < 0 {
		return "", false
	}
	properties := sidecarProperties(tags, propertyDepth)
	if len(properties) == 0 {
		return content, true
	}
	head := strings.TrimRight(content[:closeAt], " \t")
	indent := content[len(head):closeAt]
	if !strings.HasSuffix(head, "\n") {
		head += "\n"
	}
	return head + dcDescription(indent+sidecarIndent, properties) + "\n" + indent + content[closeAt:], true
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
