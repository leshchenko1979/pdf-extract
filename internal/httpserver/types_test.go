package httpserver

import "testing"

func TestOptionsResolved(t *testing.T) {
	t.Parallel()
	r, c := (Options{}).resolved()
	if r != false || c != true {
		t.Fatalf("defaults: got render=%v crop=%v", r, c)
	}
	tr, fa := true, false
	r, c = (Options{RenderImage: &tr, CropMargins: &fa}).resolved()
	if r != true || c != false {
		t.Fatalf("explicit: got render=%v crop=%v", r, c)
	}
}
