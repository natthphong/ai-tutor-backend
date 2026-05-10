package ai

import (
	"bytes"
	"io"
	"mime/multipart"
)

// MultipartWriter wraps multipart.Writer for convenience
type MultipartWriter struct {
	*multipart.Writer
}

func NewMultipartWriter(buf *bytes.Buffer) *MultipartWriter {
	return &MultipartWriter{multipart.NewWriter(buf)}
}

func (w *MultipartWriter) CreateFormFile(fieldname, filename string) (io.Writer, error) {
	return w.Writer.CreateFormFile(fieldname, filename)
}

func (w *MultipartWriter) WriteField(fieldname, value string) error {
	return w.Writer.WriteField(fieldname, value)
}

func (w *MultipartWriter) Close() error {
	return w.Writer.Close()
}

func (w *MultipartWriter) FormDataContentType() string {
	return w.Writer.FormDataContentType()
}
