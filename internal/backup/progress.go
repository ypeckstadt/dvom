package backup

import (
	"fmt"
	"io"
	"time"

	"github.com/cheggaaa/pb/v3"
)

// ProgressReader wraps an io.Reader with a progress bar
type ProgressReader struct {
	reader io.Reader
	bar    *pb.ProgressBar
}

// NewProgressReader creates a new progress reader
func NewProgressReader(r io.Reader, size int64, description string) *ProgressReader {
	tmpl := fmt.Sprintf(`{{ "%s" }} {{ bar . "[" "=" ">" " " "]"}} {{speed . }} {{percent . }} {{rtime . " ETA"}}`, description)
	
	bar := pb.New64(size)
	bar.Set(pb.SIBytesPrefix, true)
	bar.SetTemplateString(tmpl)
	bar.SetRefreshRate(100 * time.Millisecond)
	bar.Start()

	return &ProgressReader{
		reader: bar.NewProxyReader(r),
		bar:    bar,
	}
}

// Read implements io.Reader
func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	return pr.reader.Read(p)
}

// Close finishes the progress bar
func (pr *ProgressReader) Close() error {
	pr.bar.Finish()
	return nil
}

// ProgressWriter wraps an io.Writer with a progress bar
type ProgressWriter struct {
	writer io.Writer
	bar    *pb.ProgressBar
}

// NewProgressWriter creates a new progress writer
func NewProgressWriter(w io.Writer, size int64, description string) *ProgressWriter {
	tmpl := fmt.Sprintf(`{{ "%s" }} {{ bar . "[" "=" ">" " " "]"}} {{speed . }} {{percent . }} {{rtime . " ETA"}}`, description)
	
	bar := pb.New64(size)
	bar.Set(pb.SIBytesPrefix, true)
	bar.SetTemplateString(tmpl)
	bar.SetRefreshRate(100 * time.Millisecond)
	bar.Start()

	return &ProgressWriter{
		writer: bar.NewProxyWriter(w),
		bar:    bar,
	}
}

// Write implements io.Writer
func (pw *ProgressWriter) Write(p []byte) (n int, err error) {
	return pw.writer.Write(p)
}

// Close finishes the progress bar
func (pw *ProgressWriter) Close() error {
	pw.bar.Finish()
	return nil
}

// IndeterminateProgress shows a spinner for operations without known size
type IndeterminateProgress struct {
	description string
	spinner     *pb.ProgressBar
}

// NewIndeterminateProgress creates a new indeterminate progress indicator
func NewIndeterminateProgress(description string) *IndeterminateProgress {
	tmpl := fmt.Sprintf(`{{ "%s" }} {{ cycle . "⠋" "⠙" "⠹" "⠸" "⠼" "⠴" "⠦" "⠧" "⠇" "⠏" }}`, description)
	
	spinner := pb.New(0)
	spinner.SetTemplateString(tmpl)
	spinner.SetRefreshRate(100 * time.Millisecond)
	spinner.Start()

	return &IndeterminateProgress{
		description: description,
		spinner:     spinner,
	}
}

// Stop stops the spinner
func (ip *IndeterminateProgress) Stop() {
	ip.spinner.Finish()
}

// Update updates the spinner description
func (ip *IndeterminateProgress) Update(description string) {
	ip.description = description
	tmpl := fmt.Sprintf(`{{ "%s" }} {{ cycle . "⠋" "⠙" "⠹" "⠸" "⠼" "⠴" "⠦" "⠧" "⠇" "⠏" }}`, description)
	ip.spinner.SetTemplateString(tmpl)
}