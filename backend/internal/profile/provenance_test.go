package profile

import "testing"

// jpegWithXMP builds a minimal JPEG-shaped byte stream with an APP1 XMP packet.
func jpegWithXMP(xmp string) []byte {
	b := []byte{0xFF, 0xD8, 0xFF, 0xE1}
	b = append(b, []byte("http://ns.adobe.com/xap/1.0/\x00")...)
	b = append(b, []byte(xmp)...)
	return append(b, 0xFF, 0xD9)
}

func TestProvenanceDetectsDeclaredAI(t *testing.T) {
	cases := map[string]string{
		"iptc generated": `<rdf:Description Iptc4xmpExt:DigitalSourceType="http://cv.iptc.org/newscodes/digitalsourcetype/trainedAlgorithmicMedia"/>`,
		"iptc gen-fill":  `<Iptc4xmpExt:DigitalSourceType>http://cv.iptc.org/newscodes/digitalsourcetype/compositeWithTrainedAlgorithmicMedia</Iptc4xmpExt:DigitalSourceType>`,
		"generator tool": `<xmp:CreatorTool>Midjourney v7</xmp:CreatorTool>`,
		"c2pa generated": "jumb\x00c2pa\x00c2pa.actions\x00c2pa.created\x00http://cv.iptc.org/newscodes/digitalsourcetype/trainedAlgorithmicMedia\x00claim_generator\x00Adobe Firefly",
	}
	for name, xmp := range cases {
		if p := ReadProvenance(jpegWithXMP(xmp)); !p.Synthetic {
			t.Errorf("%s: not detected: %+v", name, p)
		}
	}
}

func TestProvenanceAllowsRealPhotos(t *testing.T) {
	camera := jpegWithXMP("jumb\x00c2pa\x00c2pa.created\x00http://cv.iptc.org/newscodes/digitalsourcetype/digitalCapture\x00claim_generator\x00Pixel Camera")
	if p := ReadProvenance(camera); p.Synthetic || !p.HasC2PA || !p.CameraProof {
		t.Fatalf("camera capture: %+v", p)
	}
	plain := jpegWithXMP(`<dc:description>Sunday walk, flux of the tide</dc:description>`)
	if p := ReadProvenance(plain); p.Synthetic || p.HasC2PA {
		t.Fatalf("plain photo flagged: %+v", p)
	}
	if p := ReadProvenance(jpegWithXMP(`<x DigitalSourceType="http://cv.iptc.org/newscodes/digitalsourcetype/screenCapture"/>`)); !p.NotPhoto || p.Synthetic {
		t.Fatalf("screenshot: %+v", p)
	}
}
