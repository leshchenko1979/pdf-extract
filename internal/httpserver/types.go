package httpserver

// ProcessJSONRequest is the JSON body for POST /v1/process (URL source).
type ProcessJSONRequest struct {
	Source  Source  `json:"source"`
	Options Options `json:"options"`
}

// Source describes where the PDF comes from (type "url").
type Source struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Options controls text vs image output.
type Options struct {
	RenderImage *bool `json:"render_image"`
	CropMargins *bool `json:"crop_margins"`
}

func (o Options) resolved() (renderImage, cropMargins bool) {
	renderImage = false
	cropMargins = true
	if o.RenderImage != nil {
		renderImage = *o.RenderImage
	}
	if o.CropMargins != nil {
		cropMargins = *o.CropMargins
	}
	return renderImage, cropMargins
}

// ProcessResponse is the success payload for POST /v1/process.
type ProcessResponse struct {
	Status string       `json:"status"`
	Text   string       `json:"text"`
	Image  *ImageRef    `json:"image"`
}

// ImageRef points to a generated PNG.
type ImageRef struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}
