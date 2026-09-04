package imaging

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"photo/internal/core/model"
)

const lightroomSidecar = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about=""
    xmlns:dc="http://purl.org/dc/elements/1.1/"
    xmlns:crs="http://ns.adobe.com/camera-raw-settings/1.0/"
    crs:Exposure2012="+0.35"
    crs:Contrast2012="+12">
   <dc:title>
    <rdf:Alt>
     <rdf:li xml:lang="x-default">Old title</rdf:li>
    </rdf:Alt>
   </dc:title>
   <dc:subject>
    <rdf:Bag>
     <rdf:li>old</rdf:li>
    </rdf:Bag>
   </dc:subject>
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
`

const exiftoolSidecar = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about=""
    xmlns:tiff="http://ns.adobe.com/tiff/1.0/"
   tiff:Make="SONY"/>
  <rdf:Description rdf:about=""
    xmlns:crs="http://ns.adobe.com/camera-raw-settings/1.0/"
   crs:Exposure2012="+0.35">
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
`

func TestSidecar_RoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "DSC001.xmp")
	written := model.Tags{
		Title:    "Fish & chips <served> on a paper.",
		Keywords: []string{"food", "street food", "lisbon"},
	}

	require.NoError(t, WriteSidecar(path, written))
	read, err := ReadSidecar(path)

	require.NoError(t, err)
	assert.Equal(t, written, read)
}

func TestReadSidecar(t *testing.T) {
	t.Parallel()

	t.Run("a missing sidecar carries no tags", func(t *testing.T) {
		t.Parallel()

		tags, err := ReadSidecar(filepath.Join(t.TempDir(), "absent.xmp"))

		require.NoError(t, err)
		assert.Equal(t, model.Tags{}, tags)
	})

	t.Run("falls back to the description when there is no title", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/">
   <dc:description>
    <rdf:Alt>
     <rdf:li xml:lang="x-default">A caption only.</rdf:li>
    </rdf:Alt>
   </dc:description>
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>`)

		tags, err := ReadSidecar(path)

		require.NoError(t, err)
		assert.Equal(t, "A caption only.", tags.Title)
	})

	t.Run("reads the properties written as attributes", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, `<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/"
   dc:title="Attribute title." dc:subject="lake, forest"/>
 </rdf:RDF>
</x:xmpmeta>`)

		tags, err := ReadSidecar(path)

		require.NoError(t, err)
		assert.Equal(t, "Attribute title.", tags.Title)
		assert.Equal(t, []string{"lake", "forest"}, tags.Keywords)
	})

	t.Run("reports malformed XML", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, "<x:xmpmeta><unclosed>")

		_, err := ReadSidecar(path)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing sidecar")
	})
}

func TestWriteSidecar(t *testing.T) {
	t.Parallel()

	t.Run("keeps the develop settings of an existing sidecar", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, lightroomSidecar)

		require.NoError(t, WriteSidecar(path, model.Tags{Title: "New title.", Keywords: []string{"new"}}))

		content := readFile(t, path)
		assert.Contains(t, content, `crs:Exposure2012="+0.35"`)
		assert.NotContains(t, content, "Old title")
		assert.NotContains(t, content, "<rdf:li>old</rdf:li>")

		tags, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, model.Tags{Title: "New title.", Keywords: []string{"new"}}, tags)
	})

	t.Run("replaces the properties written as attributes", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, `<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/"
   dc:title="Old title." dc:subject="old"></rdf:Description>
 </rdf:RDF>
</x:xmpmeta>`)

		require.NoError(t, WriteSidecar(path, model.Tags{Title: "New title.", Keywords: []string{"new"}}))

		tags, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, model.Tags{Title: "New title.", Keywords: []string{"new"}}, tags)
		assert.NotContains(t, readFile(t, path), "Old title.")
	})

	t.Run("declares the Dublin Core namespace when the sidecar lacks it", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, `<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about=""></rdf:Description>
 </rdf:RDF>
</x:xmpmeta>`)

		require.NoError(t, WriteSidecar(path, model.Tags{Title: "New title."}))

		tags, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, "New title.", tags.Title)
	})

	t.Run("writes into a self-closing description", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, exiftoolSidecar)

		require.NoError(t, WriteSidecar(path, model.Tags{Title: "New title.", Keywords: []string{"new"}}))

		tags, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, model.Tags{Title: "New title.", Keywords: []string{"new"}}, tags)

		content := readFile(t, path)
		assert.Contains(t, content, `tiff:Make="SONY"`)
		assert.Contains(t, content, `crs:Exposure2012="+0.35"`)
	})

	t.Run("writes into a description that only carries attributes", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, `<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/"
   dc:title="Old title." dc:subject="old"/>
 </rdf:RDF>
</x:xmpmeta>`)

		require.NoError(t, WriteSidecar(path, model.Tags{Title: "New title.", Keywords: []string{"new"}}))

		tags, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, model.Tags{Title: "New title.", Keywords: []string{"new"}}, tags)
		assert.NotContains(t, readFile(t, path), "Old title.")
	})

	t.Run("leaves a sidecar it cannot update alone", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, "not xmp at all")

		err := WriteSidecar(path, model.Tags{Title: "New title."})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "rdf:Description")
		assert.Equal(t, "not xmp at all", readFile(t, path))
	})
}

func writeSidecarFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "DSC001.xmp")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestWriteSidecar_LeavesAnUnchangedSidecarAlone(t *testing.T) {
	t.Parallel()

	path := writeSidecarFile(t, lightroomSidecar)
	written := model.Tags{Title: "New title.", Keywords: []string{"new"}}
	require.NoError(t, WriteSidecar(path, written))
	before, err := os.Stat(path)
	require.NoError(t, err)

	require.NoError(t, WriteSidecar(path, written))

	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime())
}

func TestWriteSidecar_RewritesASidecarItCreatedUnchanged(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "DSC001.xmp")
	written := model.Tags{Title: "A tram climbs the hill.", Keywords: []string{"lisbon", "tram"}}
	require.NoError(t, WriteSidecar(path, written))
	created := readFile(t, path)

	require.NoError(t, WriteSidecar(path, written))

	assert.Equal(t, created, readFile(t, path))
}

func TestWriteSidecar_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not enforced on Windows")
	}
	t.Parallel()

	written := model.Tags{Title: "A tram climbs the hill.", Keywords: []string{"lisbon"}}

	t.Run("a new sidecar is readable by the tools the user points at it", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		require.NoError(t, WriteSidecar(path, written))

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
	})

	t.Run("an existing sidecar keeps the mode it had", func(t *testing.T) {
		t.Parallel()

		path := writeSidecarFile(t, lightroomSidecar)
		require.NoError(t, os.Chmod(path, 0o640))

		require.NoError(t, WriteSidecar(path, written))

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
	})
}

func TestMergeSidecar_DeclaresTheDCPrefixItWrites(t *testing.T) {
	t.Parallel()

	// The Dublin Core URI is bound to a prefix of its own, so nothing in the file
	// declares dc: and the properties written below have to bring it themselves.
	const foreignPrefix = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:dcx="http://purl.org/dc/elements/1.1/">
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
`

	merged, err := mergeSidecar([]byte(foreignPrefix), model.Tags{
		Title:    "A tram climbs the hill.",
		Keywords: []string{"lisbon"},
	})
	require.NoError(t, err)
	assert.Contains(t, string(merged), `xmlns:dc="`+dcNamespace+`"`)

	parsed, err := parseSidecar(merged)
	require.NoError(t, err)
	assert.Equal(t, "A tram climbs the hill.", parsed.tags().Title)
	assert.Equal(t, []string{"lisbon"}, parsed.tags().Keywords)
}

// A second xmlns:dc on one element is not a document a strict parser will read,
// and the develop settings the regex surgery exists to preserve would go down
// with it.
func TestMergeSidecar_DescriptionBoundToAnotherVocabulary(t *testing.T) {
	t.Parallel()

	const foreignDC = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/terms/" xmlns:crs="http://ns.adobe.com/camera-raw-settings/1.0/">
   <crs:Exposure2012>+0.50</crs:Exposure2012>
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
`
	written := model.Tags{Title: "A tram climbs the hill.", Keywords: []string{"lisbon"}}

	merged, err := mergeSidecar([]byte(foreignDC), written)
	require.NoError(t, err)

	text := string(merged)
	assert.Equal(t, 1, strings.Count(text, `xmlns:dc="`+dcNamespace+`"`))
	assert.Contains(t, text, `xmlns:dc="http://purl.org/dc/terms/"`,
		"the foreign binding must be left as it was found")
	assert.Contains(t, text, "<crs:Exposure2012>+0.50</crs:Exposure2012>",
		"the develop settings must survive")
	requireWellFormed(t, merged)

	parsed, err := parseSidecar(merged)
	require.NoError(t, err)
	assert.Equal(t, written, parsed.tags())

	// The description written here comes first, so the next save edits it in
	// place instead of adding another one beside it.
	again, err := mergeSidecar(merged, written)
	require.NoError(t, err)
	assert.Equal(t, text, string(again))
}

func TestMergeSidecar_NoRDFToHoldADescriptionOfItsOwn(t *testing.T) {
	t.Parallel()

	const noRDF = `<rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/terms/">
</rdf:Description>
`

	_, err := mergeSidecar([]byte(noRDF), model.Tags{Title: "A tram climbs the hill."})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no rdf:RDF element")
}

func requireWellFormed(t *testing.T, data []byte) {
	t.Helper()
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	for {
		_, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return
		}
		require.NoError(t, err)
	}
}

// A sidecar an earlier save left with an empty property used to lose everything
// between it and the next Dublin Core element, develop settings included.
func TestMergeSidecar_KeepsWhatSitsBetweenTheProperties(t *testing.T) {
	t.Parallel()

	const unpaired = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/"
    xmlns:crs="http://ns.adobe.com/camera-raw-settings/1.0/">
   <dc:subject/>
   <crs:Exposure2012>+1.25</crs:Exposure2012>
   <crs:Contrast2012>+30</crs:Contrast2012>
   <dc:title>
    <rdf:Alt>
     <rdf:li xml:lang="x-default">Old title</rdf:li>
    </rdf:Alt>
   </dc:title>
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
`

	merged, err := mergeSidecar([]byte(unpaired), model.Tags{Title: "New title.", Keywords: []string{"new"}})
	require.NoError(t, err)

	content := string(merged)
	assert.Contains(t, content, "<crs:Exposure2012>+1.25</crs:Exposure2012>")
	assert.Contains(t, content, "<crs:Contrast2012>+30</crs:Contrast2012>")
	assert.NotContains(t, content, "Old title")
	assert.NotContains(t, content, "<dc:subject/>")

	parsed, err := parseSidecar(merged)
	require.NoError(t, err)
	assert.Equal(t, model.Tags{Title: "New title.", Keywords: []string{"new"}}, parsed.tags())
}

// Camera Raw writes structured values as nested rdf:Description elements, and
// properties left inside one describe the preset rather than the photo, where no
// other tool goes looking for them.
func TestMergeSidecar_WritesIntoTheDescriptionItOpened(t *testing.T) {
	t.Parallel()

	const nested = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about=""
    xmlns:dc="http://purl.org/dc/elements/1.1/"
    xmlns:crs="http://ns.adobe.com/camera-raw-settings/1.0/"
   crs:Exposure2012="+1.25">
   <crs:Look>
    <rdf:Description
     crs:Name="Adobe Color">
     <crs:Parameters>
      <rdf:Description
       crs:Version="15.0"/>
     </crs:Parameters>
    </rdf:Description>
   </crs:Look>
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
`

	written := model.Tags{Title: "A tram climbs the hill.", Keywords: []string{"lisbon"}}
	merged, err := mergeSidecar([]byte(nested), written)
	require.NoError(t, err)

	content := string(merged)
	assert.Contains(t, content, `crs:Exposure2012="+1.25"`)
	assert.Contains(t, content, `crs:Name="Adobe Color"`)
	for _, property := range []string{"<dc:title>", "<dc:description>", "<dc:subject>"} {
		assert.Greater(t, strings.Index(content, property), strings.Index(content, "</crs:Look>"),
			"%s belongs to the photo, not to the preset", property)
	}

	parsed, err := parseSidecar(merged)
	require.NoError(t, err)
	assert.Equal(t, written, parsed.tags())
}

func TestDescriptionClose(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "the only close", text: "|</rdf:Description>", want: 1},
		{
			name: "a close behind a nested element",
			text: "|<rdf:Description a=\"1\"></rdf:Description></rdf:Description>",
			want: 42,
		},
		{
			name: "a close behind a self-closing element",
			text: "|<rdf:Description a=\"1\"/></rdf:Description>",
			want: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			at, err := descriptionClose(tt.text, strings.Index(tt.text, "|")+1)
			require.NoError(t, err)
			assert.Equal(t, tt.want, at)
			assert.True(t, strings.HasPrefix(tt.text[at:], descriptionEnd))
		})
	}

	t.Run("no close at all", func(t *testing.T) {
		t.Parallel()
		_, err := descriptionClose("<rdf:Description>", 17)
		require.Error(t, err)
	})
}

func TestStripProperties(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "a bare property", in: "<dc:title>x</dc:title>", want: ""},
		{name: "a property with attributes", in: `<dc:subject rdf:parseType="Resource">x</dc:subject>`, want: ""},
		{name: "a self-closing property", in: "<dc:subject/>", want: ""},
		{name: "a self-closing property with attributes", in: `<dc:subject rdf:about=""/>`, want: ""},
		{name: "a foreign element between two properties", in: "<dc:subject/><crs:Keep>1</crs:Keep><dc:title>x</dc:title>", want: "<crs:Keep>1</crs:Keep>"},
		{name: "a property of another vocabulary", in: "<xmp:title>x</xmp:title>", want: "<xmp:title>x</xmp:title>"},
		{name: "an element whose name only starts like ours", in: "<dc:titles>x</dc:titles>", want: "<dc:titles>x</dc:titles>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, stripProperties(tt.in))
		})
	}
}

func TestParseSidecar_Rating(t *testing.T) {
	t.Parallel()

	const xmpBinding = `xmlns:xmp="http://ns.adobe.com/xap/1.0/"`
	tests := []struct {
		name        string
		description string
		rating      int
		rated       bool
	}{
		{
			name:        "the element the camera writes",
			description: `<rdf:Description rdf:about="" ` + xmpBinding + `><xmp:Rating>3</xmp:Rating></rdf:Description>`,
			rating:      3,
			rated:       true,
		},
		{
			name:        "the attribute Lightroom writes",
			description: `<rdf:Description rdf:about="" ` + xmpBinding + ` xmp:Rating="5"/>`,
			rating:      5,
			rated:       true,
		},
		{
			name:        "a fraction is rounded",
			description: `<rdf:Description rdf:about="" ` + xmpBinding + `><xmp:Rating> 3.0 </xmp:Rating></rdf:Description>`,
			rating:      3,
			rated:       true,
		},
		{
			name:        "a rejected photo",
			description: `<rdf:Description rdf:about="" ` + xmpBinding + `><xmp:Rating>-1</xmp:Rating></rdf:Description>`,
			rating:      -1,
			rated:       true,
		},
		{
			name:        "the namespace under another prefix",
			description: `<rdf:Description rdf:about="" xmlns:xap="http://ns.adobe.com/xap/1.0/"><xap:Rating>4</xap:Rating></rdf:Description>`,
			rating:      4,
			rated:       true,
		},
		{
			name:        "no rating",
			description: `<rdf:Description rdf:about=""/>`,
		},
		{
			name:        "a value that is no number",
			description: `<rdf:Description rdf:about="" ` + xmpBinding + `><xmp:Rating>many</xmp:Rating></rdf:Description>`,
		},
		{
			name:        "a prefix bound to nothing",
			description: `<rdf:Description rdf:about="" xmp:Rating="5"/>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			document := `<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">` +
				tt.description + `</rdf:RDF></x:xmpmeta>`

			parsed, err := parseSidecar([]byte(document))

			require.NoError(t, err)
			assert.Equal(t, tt.rated, parsed.rated)
			assert.Equal(t, tt.rating, parsed.rating)
		})
	}

	t.Run("the rating does not leak into the tags", func(t *testing.T) {
		t.Parallel()
		parsed, err := parseSidecar(sonyPacket(200))

		require.NoError(t, err)
		assert.True(t, parsed.rated)
		assert.Equal(t, model.Tags{}, parsed.tags())
	})
}

func TestParseSidecar_Place(t *testing.T) {
	t.Parallel()

	const bindings = `xmlns:Iptc4xmpCore="http://iptc.org/std/Iptc4xmpCore/1.0/xmlns/" ` +
		`xmlns:photoshop="http://ns.adobe.com/photoshop/1.0/"`
	tests := []struct {
		name        string
		description string
		want        model.Place
	}{
		{
			name: "the element form",
			description: `<rdf:Description rdf:about="" ` + bindings + `>` +
				`<Iptc4xmpCore:Location>Praia do Guincho</Iptc4xmpCore:Location>` +
				`<photoshop:City>Cascais</photoshop:City>` +
				`<photoshop:State>Lisboa</photoshop:State>` +
				`<photoshop:Country>Portugal</photoshop:Country>` +
				`</rdf:Description>`,
			want: model.Place{Location: "Praia do Guincho", City: "Cascais", State: "Lisboa", Country: "Portugal"},
		},
		{
			name: "the attribute form",
			description: `<rdf:Description rdf:about="" ` + bindings +
				` Iptc4xmpCore:Location="Praia do Guincho" photoshop:City="Cascais"` +
				` photoshop:State="Lisboa" photoshop:Country="Portugal"/>`,
			want: model.Place{Location: "Praia do Guincho", City: "Cascais", State: "Lisboa", Country: "Portugal"},
		},
		{
			name: "the namespaces under other prefixes",
			description: `<rdf:Description rdf:about="" xmlns:iptc="http://iptc.org/std/Iptc4xmpCore/1.0/xmlns/"` +
				` xmlns:ps="http://ns.adobe.com/photoshop/1.0/">` +
				`<iptc:Location>Praia do Guincho</iptc:Location><ps:City>Cascais</ps:City></rdf:Description>`,
			want: model.Place{Location: "Praia do Guincho", City: "Cascais"},
		},
		{
			name: "a level the generator left out",
			description: `<rdf:Description rdf:about="" ` + bindings + `>` +
				`<Iptc4xmpCore:Location>Somewhere at sea</Iptc4xmpCore:Location>` +
				`<photoshop:Country>Portugal</photoshop:Country></rdf:Description>`,
			want: model.Place{Location: "Somewhere at sea", Country: "Portugal"},
		},
		{
			name:        "no place at all",
			description: `<rdf:Description rdf:about=""/>`,
		},
		{
			name: "a prefix bound to another vocabulary",
			description: `<rdf:Description rdf:about="" xmlns:photoshop="http://example.com/of-our-own/">` +
				`<photoshop:City>Cascais</photoshop:City></rdf:Description>`,
		},
		{
			name:        "a prefix bound to nothing",
			description: `<rdf:Description rdf:about="" photoshop:City="Cascais"/>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			document := `<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">` +
				tt.description + `</rdf:RDF></x:xmpmeta>`

			parsed, err := parseSidecar([]byte(document))

			require.NoError(t, err)
			assert.Equal(t, tt.want, parsed.place)
			assert.Equal(t, tt.want, parsed.tags().Place)
		})
	}

	t.Run("a Lightroom sidecar carrying a place beside its develop settings", func(t *testing.T) {
		t.Parallel()
		document := strings.Replace(lightroomSidecar,
			"   <dc:subject>",
			"   <Iptc4xmpCore:Location>Praia do Guincho</Iptc4xmpCore:Location>\n"+
				"   <photoshop:City>Cascais</photoshop:City>\n"+
				"   <dc:subject>", 1)
		document = strings.Replace(document,
			`    xmlns:crs=`,
			`    xmlns:Iptc4xmpCore="`+iptcCoreNamespace+`"`+"\n"+
				`    xmlns:photoshop="`+photoshopNamespace+`"`+"\n"+
				`    xmlns:crs=`, 1)

		parsed, err := parseSidecar([]byte(document))

		require.NoError(t, err)
		assert.Equal(t, model.Tags{
			Title:    "Old title",
			Keywords: []string{"old"},
			Place:    model.Place{Location: "Praia do Guincho", City: "Cascais"},
		}, parsed.tags())
	})
}

// The prefix is all a regular expression sees, so a document that spells
// photoshop as a vocabulary of its own would have had its own City taken for
// ours and deleted.
func TestMergeSidecar_KeepsThePropertiesOfAForeignPrefix(t *testing.T) {
	t.Parallel()

	const foreignPhotoshop = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:photoshop="http://example.com/of-our-own/">
   <photoshop:City>a city of another vocabulary</photoshop:City>
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
`

	merged, err := mergeSidecar([]byte(foreignPhotoshop), placedTags())
	require.NoError(t, err)

	text := string(merged)
	assert.Contains(t, text, "<photoshop:City>a city of another vocabulary</photoshop:City>",
		"the property of the foreign vocabulary must survive")
	assert.Contains(t, text, "<photoshop:City>Cascais</photoshop:City>", "ours is written beside it")
	requireWellFormed(t, merged)
}

func placedTags() model.Tags {
	return model.Tags{
		Title:    "A tram climbs the hill.",
		Keywords: []string{"lisbon", "tram"},
		Place: model.Place{
			Location: "Praia do Guincho",
			City:     "Cascais",
			State:    "Lisboa",
			Country:  "Portugal",
		},
	}
}

func TestWriteSidecar_Place(t *testing.T) {
	t.Parallel()

	t.Run("a new sidecar carries the place back", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")

		require.NoError(t, WriteSidecar(path, placedTags()))

		content := readFile(t, path)
		assert.Contains(t, content, "<Iptc4xmpCore:Location>Praia do Guincho</Iptc4xmpCore:Location>")
		assert.Contains(t, content, "<photoshop:City>Cascais</photoshop:City>")
		requireWellFormed(t, []byte(content))

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, placedTags(), read)
	})

	t.Run("only the namespaces being written are declared", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")

		require.NoError(t, WriteSidecar(path, model.Tags{Title: "A tram climbs the hill."}))

		content := readFile(t, path)
		assert.Contains(t, content, `xmlns:dc="`+dcNamespace+`"`)
		assert.NotContains(t, content, "xmlns:photoshop=")
		assert.NotContains(t, content, "xmlns:Iptc4xmpCore=")
	})

	t.Run("a level the generator left out is not written", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		written := model.Tags{
			Title: "A tram climbs the hill.",
			Place: model.Place{Location: "Somewhere at sea", Country: "Portugal"},
		}

		require.NoError(t, WriteSidecar(path, written))

		content := readFile(t, path)
		assert.NotContains(t, content, "<photoshop:City>")
		assert.NotContains(t, content, "<photoshop:State>")

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, written, read)
	})

	t.Run("keeps the develop settings of a Lightroom sidecar", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, lightroomSidecar)

		require.NoError(t, WriteSidecar(path, placedTags()))

		content := readFile(t, path)
		assert.Contains(t, content, `crs:Exposure2012="+0.35"`)
		assert.Contains(t, content, `crs:Contrast2012="+12"`)
		requireWellFormed(t, []byte(content))

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, placedTags(), read)
	})

	t.Run("a place already in the sidecar is replaced, not doubled", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		require.NoError(t, WriteSidecar(path, placedTags()))

		moved := placedTags()
		moved.Place = model.Place{Location: "Praia de Carcavelos", City: "Oeiras", Country: "Portugal"}
		require.NoError(t, WriteSidecar(path, moved))

		content := readFile(t, path)
		assert.Equal(t, 1, strings.Count(content, "<Iptc4xmpCore:Location>"))
		assert.Equal(t, 1, strings.Count(content, `xmlns:photoshop="`+photoshopNamespace+`"`))
		assert.NotContains(t, content, "Praia do Guincho")
		assert.NotContains(t, content, "<photoshop:State>")

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, moved, read)
	})

	t.Run("a place cleared leaves nothing behind", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		require.NoError(t, WriteSidecar(path, placedTags()))

		cleared := placedTags()
		cleared.Place = model.Place{}
		require.NoError(t, WriteSidecar(path, cleared))

		content := readFile(t, path)
		assert.NotContains(t, content, "Iptc4xmpCore:Location")
		assert.NotContains(t, content, "photoshop:City")

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, cleared, read)
	})

	t.Run("the attribute form is replaced too", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, `<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:photoshop="`+photoshopNamespace+`"
   photoshop:City="Porto" photoshop:Country="Portugal"/>
 </rdf:RDF>
</x:xmpmeta>`)

		require.NoError(t, WriteSidecar(path, placedTags()))

		content := readFile(t, path)
		assert.NotContains(t, content, "Porto")
		requireWellFormed(t, []byte(content))

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, placedTags(), read)
	})
}

func conceivedTags() model.Tags {
	return model.Tags{
		Title:    "A tram climbs the hill.",
		Keywords: []string{"lisbon", "tram"},
		Concept:  "tram 28 seen head-on, morning light",
	}
}

func TestWriteSidecar_Concept(t *testing.T) {
	t.Parallel()

	t.Run("a new sidecar carries the concept back", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")

		require.NoError(t, WriteSidecar(path, conceivedTags()))

		content := readFile(t, path)
		assert.Contains(t, content, "<photo:Concept>tram 28 seen head-on, morning light</photo:Concept>")
		assert.Contains(t, content, `xmlns:photo="`+photoNamespace+`"`)
		requireWellFormed(t, []byte(content))

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, conceivedTags(), read)
	})

	t.Run("a sidecar without a concept never declares the namespace", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")

		require.NoError(t, WriteSidecar(path, placedTags()))

		assert.NotContains(t, readFile(t, path), "xmlns:photo=")
	})

	t.Run("keeps the develop settings of a Lightroom sidecar", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, lightroomSidecar)

		require.NoError(t, WriteSidecar(path, conceivedTags()))

		content := readFile(t, path)
		assert.Contains(t, content, `crs:Exposure2012="+0.35"`)
		assert.Contains(t, content, `crs:Contrast2012="+12"`)
		requireWellFormed(t, []byte(content))

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, conceivedTags(), read)
	})

	t.Run("a concept already in the sidecar is replaced, not doubled", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		require.NoError(t, WriteSidecar(path, conceivedTags()))

		rethought := conceivedTags()
		rethought.Concept = "the tram from the side, empty street"
		require.NoError(t, WriteSidecar(path, rethought))

		content := readFile(t, path)
		assert.Equal(t, 1, strings.Count(content, "<photo:Concept>"))
		assert.Equal(t, 1, strings.Count(content, `xmlns:photo="`+photoNamespace+`"`))
		assert.NotContains(t, content, "head-on")

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, rethought, read)
	})

	t.Run("a concept cleared leaves nothing behind", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		require.NoError(t, WriteSidecar(path, conceivedTags()))

		cleared := conceivedTags()
		cleared.Concept = ""
		require.NoError(t, WriteSidecar(path, cleared))

		content := readFile(t, path)
		assert.NotContains(t, content, "photo:Concept")

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, cleared, read)
	})

	t.Run("a concept alone is written without any tags to go with it", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		written := model.Tags{Concept: "tram 28 seen head-on, morning light"}

		require.NoError(t, WriteSidecar(path, written))

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, written, read)
	})

	t.Run("a sidecar the writer already wrote is edited in place", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		require.NoError(t, WriteSidecar(path, conceivedTags()))
		first := readFile(t, path)

		require.NoError(t, WriteSidecar(path, conceivedTags()))

		content := readFile(t, path)
		assert.Equal(t, first, content)
		assert.Equal(t, 1, strings.Count(content, "<rdf:Description"))
	})
}

func notedTags() model.Tags {
	return model.Tags{
		Title:    "A tram climbs the hill.",
		Keywords: []string{"lisbon", "tram"},
		Notes:    "the tram is a replica, the line reopened in 2024",
	}
}

func TestWriteSidecar_Notes(t *testing.T) {
	t.Parallel()

	t.Run("a new sidecar carries the notes back", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")

		require.NoError(t, WriteSidecar(path, notedTags()))

		content := readFile(t, path)
		assert.Contains(t, content, "<photo:Notes>the tram is a replica, the line reopened in 2024</photo:Notes>")
		assert.Contains(t, content, `xmlns:photo="`+photoNamespace+`"`)
		requireWellFormed(t, []byte(content))

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, notedTags(), read)
	})

	t.Run("keeps the develop settings of a Lightroom sidecar", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, lightroomSidecar)

		require.NoError(t, WriteSidecar(path, notedTags()))

		content := readFile(t, path)
		assert.Contains(t, content, `crs:Exposure2012="+0.35"`)
		requireWellFormed(t, []byte(content))

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, notedTags(), read)
	})

	t.Run("notes already in the sidecar are replaced, not doubled", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		require.NoError(t, WriteSidecar(path, notedTags()))

		rewritten := notedTags()
		rewritten.Notes = "the line reopened in 2025"
		require.NoError(t, WriteSidecar(path, rewritten))

		content := readFile(t, path)
		assert.Equal(t, 1, strings.Count(content, "<photo:Notes>"))
		assert.NotContains(t, content, "replica")

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, rewritten, read)
	})

	t.Run("notes cleared leave nothing behind", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		require.NoError(t, WriteSidecar(path, notedTags()))

		cleared := notedTags()
		cleared.Notes = ""
		require.NoError(t, WriteSidecar(path, cleared))

		content := readFile(t, path)
		assert.NotContains(t, content, "photo:Notes")

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, cleared, read)
	})

	t.Run("notes alone are written without any tags to go with them", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		written := model.Tags{Notes: "the tram is a replica"}

		require.NoError(t, WriteSidecar(path, written))

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, written, read)
	})

	// The notes stand beside the concept in the same namespace, so the two are
	// written and read back together rather than one taking the other's value.
	t.Run("the notes and the concept keep to themselves", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		both := conceivedTags()
		both.Notes = "the tram is a replica"

		require.NoError(t, WriteSidecar(path, both))

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, both, read)
	})

	// The field is a multi-line one, so a note of several lines is the ordinary
	// case rather than an odd one, and the lines have to survive the document.
	t.Run("notes of several lines come back whole", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		written := notedTags()
		written.Notes = "the tram is a replica\nthe line reopened in 2024\n\nask before selling it as editorial"

		require.NoError(t, WriteSidecar(path, written))

		requireWellFormed(t, []byte(readFile(t, path)))
		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, written, read)
	})

	// XML says nothing about what a & or a < mean in a note, so they are escaped
	// on the way in and are the same characters again on the way out.
	t.Run("notes with markup characters survive", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		written := notedTags()
		written.Notes = `Tom & Jerry <the mural>, "quoted" & signed 'TM'`

		require.NoError(t, WriteSidecar(path, written))

		requireWellFormed(t, []byte(readFile(t, path)))
		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, written, read)
	})

	t.Run("reads the notes an attribute writer left", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:photo="`+photoNamespace+`"
   photo:Notes="the tram is a replica"/>
 </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
`)

		read, err := ReadSidecar(path)

		require.NoError(t, err)
		assert.Equal(t, "the tram is a replica", read.Notes)
	})
}

func editorialTags() model.Tags {
	return model.Tags{
		Title:     "A tram climbs the hill.",
		Keywords:  []string{"lisbon", "tram"},
		Editorial: model.Editorial{Marked: true, Date: time.Date(2026, time.June, 13, 0, 0, 0, 0, time.UTC)},
	}
}

func TestWriteSidecar_Editorial(t *testing.T) {
	t.Parallel()

	t.Run("a new sidecar carries the mark and the day back", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")

		require.NoError(t, WriteSidecar(path, editorialTags()))

		content := readFile(t, path)
		assert.Contains(t, content, "<photo:Editorial>True</photo:Editorial>")
		assert.Contains(t, content, "<photo:EditorialDate>2026-06-13</photo:EditorialDate>")
		assert.Contains(t, content, `xmlns:photo="`+photoNamespace+`"`)
		requireWellFormed(t, []byte(content))

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, editorialTags(), read)
	})

	t.Run("an unmarked photo writes neither property", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		unmarked := editorialTags()
		unmarked.Editorial.Marked = false

		require.NoError(t, WriteSidecar(path, unmarked))

		content := readFile(t, path)
		assert.NotContains(t, content, "photo:Editorial")
		assert.NotContains(t, content, "xmlns:photo=")

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.True(t, read.Editorial.IsEmpty(), "a date without the mark is nothing to read back")
	})

	t.Run("the mark is marked without a day of its own", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		dateless := editorialTags()
		dateless.Editorial.Date = time.Time{}

		require.NoError(t, WriteSidecar(path, dateless))

		content := readFile(t, path)
		assert.Contains(t, content, "<photo:Editorial>True</photo:Editorial>")
		assert.NotContains(t, content, "photo:EditorialDate")

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, dateless, read)
	})

	t.Run("a second save replaces the properties instead of doubling them", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		require.NoError(t, WriteSidecar(path, editorialTags()))

		moved := editorialTags()
		moved.Editorial.Date = time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, WriteSidecar(path, moved))

		content := readFile(t, path)
		assert.Equal(t, 1, strings.Count(content, "<photo:Editorial>"))
		assert.Equal(t, 1, strings.Count(content, "<photo:EditorialDate>"))
		assert.NotContains(t, content, "2026-06-13")

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, moved, read)
	})

	// The mark and its date share a name up to the last four characters, so a
	// pattern that stripped one by its prefix alone would take the other with it.
	t.Run("clearing the day leaves the mark standing", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		require.NoError(t, WriteSidecar(path, editorialTags()))

		dateless := editorialTags()
		dateless.Editorial.Date = time.Time{}
		require.NoError(t, WriteSidecar(path, dateless))

		content := readFile(t, path)
		assert.Equal(t, 1, strings.Count(content, "<photo:Editorial>"))
		assert.NotContains(t, content, "photo:EditorialDate")

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, dateless, read)
	})

	t.Run("clearing the mark takes both properties with it", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		require.NoError(t, WriteSidecar(path, editorialTags()))

		cleared := editorialTags()
		cleared.Editorial = model.Editorial{}
		require.NoError(t, WriteSidecar(path, cleared))

		content := readFile(t, path)
		assert.NotContains(t, content, "photo:Editorial")
		assert.NotContains(t, content, "2026-06-13")

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, cleared, read)
	})

	t.Run("a mark alone is written without any tags to go with it", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		written := model.Tags{Editorial: model.Editorial{Marked: true,
			Date: time.Date(2026, time.June, 13, 0, 0, 0, 0, time.UTC)}}

		require.NoError(t, WriteSidecar(path, written))

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, written, read)
	})

	t.Run("keeps the develop settings of a Lightroom sidecar", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, lightroomSidecar)

		require.NoError(t, WriteSidecar(path, editorialTags()))

		content := readFile(t, path)
		assert.Contains(t, content, `crs:Exposure2012="+0.35"`)
		assert.Contains(t, content, `crs:Contrast2012="+12"`)
		requireWellFormed(t, []byte(content))

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, editorialTags(), read)
	})

	t.Run("the concept and the mark stand side by side", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "DSC001.xmp")
		both := editorialTags()
		both.Concept = "tram 28 seen head-on, morning light"

		require.NoError(t, WriteSidecar(path, both))

		content := readFile(t, path)
		assert.Equal(t, 1, strings.Count(content, `xmlns:photo="`+photoNamespace+`"`))
		requireWellFormed(t, []byte(content))

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, both, read)
	})

	t.Run("a whole timestamp another tool left is read as the day it names", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, `<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:photo="`+photoNamespace+`">
   <photo:Editorial>True</photo:Editorial>
   <photo:EditorialDate>2026-06-13T18:24:05+01:00</photo:EditorialDate>
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>`)

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, model.Editorial{Marked: true,
			Date: time.Date(2026, time.June, 13, 0, 0, 0, 0, time.UTC)}, read.Editorial)
	})

	t.Run("the attribute form is read and replaced too", func(t *testing.T) {
		t.Parallel()
		path := writeSidecarFile(t, `<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:photo="`+photoNamespace+`"
   photo:Editorial="True" photo:EditorialDate="2020-01-02"/>
 </rdf:RDF>
</x:xmpmeta>`)

		read, err := ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, model.Editorial{Marked: true,
			Date: time.Date(2020, time.January, 2, 0, 0, 0, 0, time.UTC)}, read.Editorial)

		require.NoError(t, WriteSidecar(path, editorialTags()))

		content := readFile(t, path)
		assert.NotContains(t, content, "2020-01-02")
		requireWellFormed(t, []byte(content))

		read, err = ReadSidecar(path)
		require.NoError(t, err)
		assert.Equal(t, editorialTags(), read)
	})
}

// A document is free to spell photo as a vocabulary of its own, and that element
// is another tool's to keep - the same promise TestMergeSidecar_KeepsThePropertiesOfAForeignPrefix
// makes for photoshop, and just as far: the bindings of the whole document decide,
// so a document that also binds photo to ours has spent the prefix on us.
func TestMergeSidecar_KeepsAConceptOfAForeignPrefix(t *testing.T) {
	t.Parallel()

	const foreignPhoto = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:photo="http://example.com/of-our-own/">
   <photo:Concept>a concept of another vocabulary</photo:Concept>
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
`

	merged, err := mergeSidecar([]byte(foreignPhoto), conceivedTags())
	require.NoError(t, err)

	text := string(merged)
	assert.Contains(t, text, "<photo:Concept>a concept of another vocabulary</photo:Concept>",
		"the property of the foreign vocabulary must survive")
	assert.Contains(t, text, "<photo:Concept>tram 28 seen head-on, morning light</photo:Concept>",
		"ours is written beside it")
	assert.Contains(t, text, `xmlns:photo="http://example.com/of-our-own/"`,
		"the foreign binding must be left as it was found")
	requireWellFormed(t, merged)

	parsed, err := parseSidecar(merged)
	require.NoError(t, err)
	assert.Equal(t, conceivedTags(), parsed.tags())
}

// The prefix means something else here, so a second declaration of it on the
// same element would be a document no strict parser reads.
func TestMergeSidecar_PhotoshopPrefixBoundToAnotherVocabulary(t *testing.T) {
	t.Parallel()

	const foreignPhotoshop = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:photoshop="http://example.com/of-our-own/"
    xmlns:crs="http://ns.adobe.com/camera-raw-settings/1.0/">
   <crs:Exposure2012>+0.50</crs:Exposure2012>
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
`
	written := placedTags()

	merged, err := mergeSidecar([]byte(foreignPhotoshop), written)
	require.NoError(t, err)

	text := string(merged)
	assert.Contains(t, text, `xmlns:photoshop="http://example.com/of-our-own/"`,
		"the foreign binding must be left as it was found")
	assert.Contains(t, text, "<crs:Exposure2012>+0.50</crs:Exposure2012>", "the develop settings must survive")
	assert.Equal(t, 1, strings.Count(text, `xmlns:photoshop="`+photoshopNamespace+`"`))
	requireWellFormed(t, merged)

	parsed, err := parseSidecar(merged)
	require.NoError(t, err)
	assert.Equal(t, written, parsed.tags())

	again, err := mergeSidecar(merged, written)
	require.NoError(t, err)
	assert.Equal(t, text, string(again))
}
