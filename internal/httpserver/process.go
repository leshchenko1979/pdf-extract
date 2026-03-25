package httpserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/leshchenko/pdf-extract/internal/config"
	"github.com/leshchenko/pdf-extract/internal/pdf"
	"github.com/leshchenko/pdf-extract/internal/storage"
)

// Service wires config, HTTP fetch, and storage.
type Service struct {
	Cfg         *config.Config
	FetchClient *http.Client
	Store       *storage.Storage
}

func (s *Service) absFileURL(id string) string {
	base := strings.TrimRight(s.Cfg.PublicBaseURL, "/")
	return fmt.Sprintf("%s/v1/files/%s", base, id)
}

func validatePDFHeader(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, 5)
	if _, err := io.ReadFull(f, buf); err != nil {
		return fmt.Errorf("read pdf header: %w", err)
	}
	if string(buf) != "%PDF-" {
		return fmt.Errorf("file is not a valid PDF")
	}
	return nil
}

func (s *Service) runPipeline(pdfPath string, renderImage, cropMargins bool) (text string, imageID string, outPNG string, err error) {
	st, statErr := os.Stat(pdfPath)
	if statErr != nil {
		return "", "", "", statErr
	}
	if st.Size() == 0 {
		return "", "", "", fmt.Errorf("PDF file is empty")
	}
	if err := validatePDFHeader(pdfPath); err != nil {
		return "", "", "", err
	}
	enc, err := pdf.IsEncrypted(pdfPath)
	if err != nil {
		return "", "", "", err
	}
	if enc {
		return "", "", "", fmt.Errorf("PDF is encrypted or password-protected")
	}
	text, err = pdf.ExtractText(pdfPath)
	if err != nil {
		return "", "", "", err
	}
	if !renderImage {
		return text, "", "", nil
	}
	id := uuid.NewString()
	outPath := filepath.Join(s.Cfg.OutputDir, id+".png")
	if err := pdf.StitchToPNG(pdfPath, outPath, cropMargins, s.Cfg.RenderDPI); err != nil {
		return "", "", "", err
	}
	return text, id, outPath, nil
}

// HandleProcessJSON handles application/json POST /v1/process.
func (s *Service) HandleProcessJSON(w http.ResponseWriter, r *http.Request) {
	var req ProcessJSONRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid JSON", err.Error())
		return
	}
	if !strings.EqualFold(strings.TrimSpace(req.Source.Type), "url") {
		writeProblem(w, http.StatusBadRequest, "Invalid source", `source.type must be "url" for JSON requests`)
		return
	}
	urlStr := strings.TrimSpace(req.Source.URL)
	if urlStr == "" {
		writeProblem(w, http.StatusBadRequest, "Invalid source", "source.url is required")
		return
	}
	renderImage, cropMargins := req.Options.resolved()
	if !renderImage {
		cropMargins = false
	}

	id := uuid.NewString()
	pdfPath := filepath.Join(s.Cfg.UploadDir, id+".pdf")
	if err := DownloadPDF(s.FetchClient, urlStr, s.Cfg.MaxDownloadBytes, pdfPath); err != nil {
		_ = os.Remove(pdfPath)
		writeProblem(w, http.StatusBadRequest, "Failed to fetch PDF", err.Error())
		return
	}

	text, imgID, outPNG, err := s.runPipeline(pdfPath, renderImage, cropMargins)
	if err != nil {
		_ = os.Remove(pdfPath)
		if outPNG != "" {
			_ = os.Remove(outPNG)
		}
		writeProblem(w, http.StatusBadRequest, "PDF processing failed", err.Error())
		return
	}

	if renderImage && imgID != "" && outPNG != "" {
		s.Store.ScheduleDelete(pdfPath, outPNG)
	} else {
		s.Store.ScheduleDelete(pdfPath)
	}

	var img *ImageRef
	if renderImage && imgID != "" {
		img = &ImageRef{ID: imgID, URL: s.absFileURL(imgID)}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ProcessResponse{
		Status: "success",
		Text:   text,
		Image:  img,
	})
}

// HandleProcessMultipart handles multipart/form-data POST /v1/process.
func (s *Service) HandleProcessMultipart(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(s.Cfg.MaxUploadBytes); err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid multipart form", err.Error())
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "Missing file", `form field "file" with PDF is required`)
		return
	}
	defer file.Close()

	opts := Options{}
	if raw := strings.TrimSpace(r.FormValue("options")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &opts); err != nil {
			writeProblem(w, http.StatusBadRequest, "Invalid options JSON", err.Error())
			return
		}
	}
	renderImage, cropMargins := opts.resolved()
	if !renderImage {
		cropMargins = false
	}

	id := uuid.NewString()
	pdfPath := filepath.Join(s.Cfg.UploadDir, id+".pdf")
	out, err := os.Create(pdfPath)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Storage error", err.Error())
		return
	}
	lim := io.LimitReader(file, s.Cfg.MaxUploadBytes+1)
	n, err := io.Copy(out, lim)
	_ = out.Close()
	if err != nil {
		_ = os.Remove(pdfPath)
		writeProblem(w, http.StatusBadRequest, "Failed to save upload", err.Error())
		return
	}
	if n > s.Cfg.MaxUploadBytes {
		_ = os.Remove(pdfPath)
		writeProblem(w, http.StatusRequestEntityTooLarge, "Upload too large", "file exceeds MAX_UPLOAD_BYTES")
		return
	}
	if hdr.Size > 0 && hdr.Size > s.Cfg.MaxUploadBytes {
		_ = os.Remove(pdfPath)
		writeProblem(w, http.StatusRequestEntityTooLarge, "Upload too large", "file exceeds MAX_UPLOAD_BYTES")
		return
	}

	text, imgID, outPNG, err := s.runPipeline(pdfPath, renderImage, cropMargins)
	if err != nil {
		_ = os.Remove(pdfPath)
		if outPNG != "" {
			_ = os.Remove(outPNG)
		}
		writeProblem(w, http.StatusBadRequest, "PDF processing failed", err.Error())
		return
	}

	if renderImage && imgID != "" && outPNG != "" {
		s.Store.ScheduleDelete(pdfPath, outPNG)
	} else {
		s.Store.ScheduleDelete(pdfPath)
	}

	var img *ImageRef
	if renderImage && imgID != "" {
		img = &ImageRef{ID: imgID, URL: s.absFileURL(imgID)}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ProcessResponse{
		Status: "success",
		Text:   text,
		Image:  img,
	})
}
