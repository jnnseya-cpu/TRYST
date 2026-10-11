package profile

import (
	"bytes"
	"strings"
)

// Provenance is what an image's embedded metadata says about how it was made.
//
// Two open standards carry this, and the major image generators write them:
//   - IPTC "Digital Source Type" (in XMP), e.g. trainedAlgorithmicMedia for a generated image;
//   - C2PA Content Credentials (a signed JUMBF manifest), whose actions record the same
//     IPTC source-type URIs, e.g. a c2pa.created action with trainedAlgorithmicMedia.
//
// Metadata can be stripped, so this is only the first of several layers (03 §7.5):
// face match to the verified member, a synthetic-image detector, reverse-image search,
// and member reports. The client removes location and device identifiers before upload but
// keeps provenance; the server reads it here and then strips all metadata before storage.
type Provenance struct {
	Synthetic   bool   // declared AI-generated or AI-composited
	NotPhoto    bool   // declared digital art, screen capture or other non-photograph
	Generator   string // generator named in metadata, if any
	HasC2PA     bool   // a C2PA manifest is present
	CameraProof bool   // C2PA manifest declares a direct digital capture
}

const dst = "http://cv.iptc.org/newscodes/digitalsourcetype/"

// IPTC digital source types (IPTC NewsCodes) that are not allowed on a profile.
var syntheticTypes = []string{
	"trainedAlgorithmicMedia",              // made by a generative model
	"compositeWithTrainedAlgorithmicMedia", // photo edited with generative fill or similar
	"algorithmicMedia",                     // made by an algorithm without a capture
	"compositeSynthetic",                   // composite including synthetic elements
}

var notPhotoTypes = []string{"digitalArt", "virtualRecording", "screenCapture"}

// Generator names that appear in XMP CreatorTool / EXIF Software or C2PA claim generators.
var generators = []string{
	"midjourney", "dall-e", "dall·e", "openai", "stable diffusion", "stability ai", "adobe firefly",
	"firefly", "imagen", "gemini", "leonardo.ai", "ideogram", "flux", "runway", "craiyon",
	"nightcafe", "dreamstudio", "bing image creator", "canva ai", "getimg",
}

// ReadProvenance scans the raw bytes of an uploaded image for provenance declarations.
// It does not decode pixels; it looks for the standard URIs and labels wherever the
// container stores them (XMP packets, C2PA JUMBF boxes in JPEG APP11, PNG caBX, WebP).
func ReadProvenance(img []byte) Provenance {
	var p Provenance
	low := bytes.ToLower(img)
	for _, t := range syntheticTypes {
		if bytes.Contains(img, []byte(dst+t)) || bytes.Contains(img, []byte(`digitalsourcetype="`+t)) ||
			bytes.Contains(low, []byte(strings.ToLower(dst+t))) {
			p.Synthetic = true
		}
	}
	for _, t := range notPhotoTypes {
		if bytes.Contains(low, []byte(strings.ToLower(dst+t))) {
			p.NotPhoto = true
		}
	}
	if bytes.Contains(img, []byte("c2pa")) && (bytes.Contains(img, []byte("jumb")) || bytes.Contains(img, []byte("caBX"))) {
		p.HasC2PA = true
		if bytes.Contains(low, []byte(strings.ToLower(dst+"digitalCapture"))) && !p.Synthetic {
			p.CameraProof = true
		}
	}
	for _, g := range generators {
		if bytes.Contains(low, []byte(g)) && generatorContext(low, []byte(g)) {
			p.Generator = g
			p.Synthetic = true
			break
		}
	}
	return p
}

// generatorContext requires the generator name to sit inside a metadata field that names a
// tool (so a photo whose caption merely mentions "flux" is not rejected).
func generatorContext(low, name []byte) bool {
	for _, field := range [][]byte{[]byte("creatortool"), []byte("software"), []byte("claim_generator"),
		[]byte("softwareagent"), []byte("generator")} {
		i := bytes.Index(low, field)
		for i >= 0 {
			end := i + 200
			if end > len(low) {
				end = len(low)
			}
			if bytes.Contains(low[i:end], name) {
				return true
			}
			next := bytes.Index(low[i+1:], field)
			if next < 0 {
				break
			}
			i += 1 + next
		}
	}
	return false
}
