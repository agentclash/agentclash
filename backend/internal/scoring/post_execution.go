package scoring

const (
	PostExecutionCheckTypeFileCapture     = "file_capture"
	PostExecutionCheckTypeDirectoryListing = "directory_listing"

	DefaultMaxFileSizeBytes     int64 = 1 << 20  // 1MB per file
	DefaultMaxTotalCaptureBytes int64 = 10 << 20 // 10MB total per run
)

// PostExecutionCheck specifies a file or directory to capture from the sandbox
// after agent execution completes, before the sandbox is destroyed. Results are
// emitted as grader.verification.* events and made available to the scoring
// engine via the file: evidence prefix.
type PostExecutionCheck struct {
	Key          string `json:"key"`
	Type         string `json:"type"` // "file_capture" or "directory_listing"
	Path         string `json:"path"`
	Recursive    bool   `json:"recursive,omitempty"`
	MaxSizeBytes int64  `json:"max_size_bytes,omitempty"`
}

// EffectiveMaxSizeBytes returns the configured max size or the default.
func (c PostExecutionCheck) EffectiveMaxSizeBytes() int64 {
	if c.MaxSizeBytes > 0 {
		return c.MaxSizeBytes
	}
	return DefaultMaxFileSizeBytes
}

// FileCaptureResult holds the content of a file read from the sandbox.
type FileCaptureResult struct {
	Key       string `json:"key"`
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	Content   string `json:"content,omitempty"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated,omitempty"`
}

// DirectoryListingResult holds the listing of a directory from the sandbox.
type DirectoryListingResult struct {
	Key     string           `json:"key"`
	Path    string           `json:"path"`
	Entries []DirectoryEntry `json:"entries"`
}

// DirectoryEntry represents a single file or directory within a listing.
type DirectoryEntry struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir,omitempty"`
}
