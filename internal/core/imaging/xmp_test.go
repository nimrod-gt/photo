package imaging

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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
