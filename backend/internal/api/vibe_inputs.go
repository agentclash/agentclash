package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/backend/internal/vibe/inputs"
	"github.com/google/uuid"
)

func inputError(w http.ResponseWriter, err error) {
	var invalid *inputs.ValidationError
	switch {
	case errors.Is(err, inputs.ErrUnavailable):
		err = &vibe.Fault{Code: "not_found", Message: "This material is unavailable."}
	case errors.Is(err, inputs.ErrConflict):
		err = &vibe.Fault{Code: "idempotency_conflict", Message: "This request ID belongs to different material."}
	case errors.Is(err, inputs.ErrParserUnavailable):
		err = &vibe.Fault{Code: "hosted_disabled", Message: err.Error()}
	case errors.As(err, &invalid):
		err = &vibe.Fault{Code: "invalid_input", Message: invalid.Error()}
	}
	vibeError(w, err)
}
func (h *VibeHandler) createInput(w http.ResponseWriter, r *http.Request) {
	if !h.Service.Config.MaterialBuild {
		vibeError(w, &vibe.Fault{Code: "hosted_disabled", Message: "Materials are unavailable. Existing files can still be read or deleted."})
		return
	}
	v, err := h.authorizedSession(r, true)
	if err != nil {
		vibeError(w, err)
		return
	}
	if err = h.Service.Gate.Check(r.Context(), "input:"+v.Actor, h.Service.Config.Limits(v.Anonymous)); err != nil {
		vibeError(w, err)
		return
	}
	if e := r.Header.Get("Content-Encoding"); e != "" && e != "identity" {
		vibeError(w, &vibe.Fault{Code: "unsupported_media", Message: "Compressed uploads are unsupported."})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, inputs.MaxPDFBytes+64_000)
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		vibeError(w, &vibe.Fault{Code: "unsupported_media", Message: "Choose a PDF or pasted text."})
		return
	}
	var client uuid.UUID
	var data []byte
	kind, name := "", ""
	switch media {
	case "application/json":
		r.Body = http.MaxBytesReader(w, r.Body, 400_000)
		var in struct {
			ClientID uuid.UUID `json:"client_id"`
			Kind     string    `json:"kind"`
			Text     string    `json:"text"`
			Name     string    `json:"name"`
		}
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&in)
		if err == nil {
			var trailing any
			if decoder.Decode(&trailing) != io.EOF {
				err = errors.New("Send one JSON object.")
			}
		}
		if err == nil && in.Kind != "text" {
			err = errors.New("Use pasted text or a PDF file.")
		}
		client, kind, name, data = in.ClientID, in.Kind, in.Name, []byte(in.Text)
		if name == "" {
			name = "Pasted text"
		}
	case "multipart/form-data":
		reader, e := r.MultipartReader()
		if e != nil {
			inputError(w, inputs.Invalid("Could not read this upload. Choose the PDF again."))
			return
		}
		fields := map[string]bool{}
		for {
			part, e := reader.NextPart()
			if e == io.EOF {
				break
			}
			if e != nil {
				err = e
				break
			}
			field := part.FormName()
			if fields[field] || (field != "client_id" && field != "file") {
				err = errors.New("Send one PDF and one request ID.")
				break
			}
			fields[field] = true
			limit := int64(100)
			if field == "file" {
				limit = inputs.MaxPDFBytes + 1
			}
			b, e := io.ReadAll(io.LimitReader(part, limit))
			part.Close()
			if e != nil {
				err = e
				break
			}
			if field == "client_id" {
				client, err = uuid.Parse(string(b))
			} else {
				if len(b) > inputs.MaxPDFBytes {
					vibeError(w, &http.MaxBytesError{Limit: inputs.MaxPDFBytes})
					return
				}
				partType, _, typeError := mime.ParseMediaType(part.Header.Get("Content-Type"))
				if typeError != nil || partType != "application/pdf" {
					vibeError(w, &vibe.Fault{Code: "unsupported_media", Message: "Choose a PDF."})
					return
				}
				kind, name, data = "pdf", part.FileName(), b
			}
			if err != nil {
				break
			}
		}
	default:
		vibeError(w, &vibe.Fault{Code: "unsupported_media", Message: "Choose a PDF or pasted text."})
		return
	}
	if err != nil {
		var size *http.MaxBytesError
		if !errors.As(err, &size) {
			err = inputs.Invalid("Could not read this submission. Use one PDF or pasted text with a valid request ID.")
		}
		inputError(w, err)
		return
	}
	record, err := h.Service.Store.Inputs.Create(r.Context(), v.ID, v.Actor, client, kind, name, data)
	if err != nil {
		inputError(w, err)
		return
	}
	status := http.StatusCreated
	if record.Status == "uploaded" || record.Status == "extracting" {
		status = http.StatusAccepted
	}
	vibeJSON(w, status, record)
}
func (h *VibeHandler) listInputs(w http.ResponseWriter, r *http.Request) {
	v, err := h.authorizedSession(r, false)
	if err != nil {
		vibeError(w, err)
		return
	}
	records, err := h.Service.Store.Inputs.List(r.Context(), v.ID)
	if err != nil {
		vibeError(w, err)
		return
	}
	vibeJSON(w, 200, records)
}
func (h *VibeHandler) input(r *http.Request) (inputs.Record, error) {
	v, err := h.authorizedSession(r, false)
	if err != nil {
		return inputs.Record{}, err
	}
	id, err := vibeID(r, "inputID")
	if err != nil {
		return inputs.Record{}, err
	}
	return h.Service.Store.Inputs.Get(r.Context(), v.ID, id)
}
func (h *VibeHandler) getInput(w http.ResponseWriter, r *http.Request) {
	record, err := h.input(r)
	if err != nil {
		inputError(w, err)
		return
	}
	vibeJSON(w, 200, record)
}
func (h *VibeHandler) deleteInput(w http.ResponseWriter, r *http.Request) {
	v, err := h.authorizedSession(r, true)
	if err != nil {
		vibeError(w, err)
		return
	}
	id, err := vibeID(r, "inputID")
	if err == nil {
		err = h.Service.Store.Inputs.Delete(r.Context(), v.ID, id, v.Actor)
	}
	if err != nil {
		inputError(w, err)
		return
	}
	vibeJSON(w, 202, map[string]string{"status": "deleted", "message": "The file is unavailable for new runs. Existing conversations and results remain and may contain information from it."})
}
func (h *VibeHandler) downloadInput(w http.ResponseWriter, r *http.Request) {
	record, err := h.input(r)
	if err != nil {
		inputError(w, err)
		return
	}
	body, err := h.Service.Store.Inputs.Download(r.Context(), record)
	if err != nil {
		inputError(w, err)
		return
	}
	defer body.Close()
	media := "text/plain; charset=utf-8"
	if record.Kind == "pdf" {
		media = "application/pdf"
	}
	w.Header().Set("Content-Type", media)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": record.Name}))
	io.Copy(w, body)
}
