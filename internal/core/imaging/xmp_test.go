package imaging

import (
	"os"
	"path/filepath"
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

func TestSidecarPath(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "/photos/DSC001.xmp", SidecarPath("/photos/DSC001.ARW"))
	assert.Equal(t, "/photos/DSC001.xmp", SidecarPath("/photos/DSC001.arw"))
	assert.Equal(t, "/photos/no-extension.xmp", SidecarPath("/photos/no-extension"))
}

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
