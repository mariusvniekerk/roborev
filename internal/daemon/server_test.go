package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/wesm/roborev/internal/config"
	"github.com/wesm/roborev/internal/storage"
)

func TestHandleCancelJob(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	defer db.Close()

	cfg := config.DefaultConfig()
	server := NewServer(db, cfg)

	// Create a repo and job
	repo, _ := db.GetOrCreateRepo(tmpDir)
	commit, _ := db.GetOrCreateCommit(repo.ID, "canceltest", "Author", "Subject", time.Now())
	job, _ := db.EnqueueJob(repo.ID, commit.ID, "canceltest", "test")

	t.Run("cancel queued job", func(t *testing.T) {
		reqBody, _ := json.Marshal(CancelJobRequest{JobID: job.ID})
		req := httptest.NewRequest(http.MethodPost, "/api/job/cancel", bytes.NewReader(reqBody))
		w := httptest.NewRecorder()

		server.handleCancelJob(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		updated, _ := db.GetJobByID(job.ID)
		if updated.Status != storage.JobStatusCanceled {
			t.Errorf("Expected status 'canceled', got '%s'", updated.Status)
		}
	})

	t.Run("cancel already canceled job fails", func(t *testing.T) {
		// Job is already canceled from previous test
		reqBody, _ := json.Marshal(CancelJobRequest{JobID: job.ID})
		req := httptest.NewRequest(http.MethodPost, "/api/job/cancel", bytes.NewReader(reqBody))
		w := httptest.NewRecorder()

		server.handleCancelJob(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for already canceled job, got %d", w.Code)
		}
	})

	t.Run("cancel nonexistent job fails", func(t *testing.T) {
		reqBody, _ := json.Marshal(CancelJobRequest{JobID: 99999})
		req := httptest.NewRequest(http.MethodPost, "/api/job/cancel", bytes.NewReader(reqBody))
		w := httptest.NewRecorder()

		server.handleCancelJob(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for nonexistent job, got %d", w.Code)
		}
	})

	t.Run("cancel with missing job_id fails", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]interface{}{})
		req := httptest.NewRequest(http.MethodPost, "/api/job/cancel", bytes.NewReader(reqBody))
		w := httptest.NewRecorder()

		server.handleCancelJob(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for missing job_id, got %d", w.Code)
		}
	})

	t.Run("cancel with wrong method fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/job/cancel", nil)
		w := httptest.NewRecorder()

		server.handleCancelJob(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405 for GET, got %d", w.Code)
		}
	})

	t.Run("cancel running job", func(t *testing.T) {
		// Create a new job and claim it
		commit2, _ := db.GetOrCreateCommit(repo.ID, "cancelrunning", "Author", "Subject", time.Now())
		job2, _ := db.EnqueueJob(repo.ID, commit2.ID, "cancelrunning", "test")
		db.ClaimJob("worker-1")

		reqBody, _ := json.Marshal(CancelJobRequest{JobID: job2.ID})
		req := httptest.NewRequest(http.MethodPost, "/api/job/cancel", bytes.NewReader(reqBody))
		w := httptest.NewRecorder()

		server.handleCancelJob(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		updated, _ := db.GetJobByID(job2.ID)
		if updated.Status != storage.JobStatusCanceled {
			t.Errorf("Expected status 'canceled', got '%s'", updated.Status)
		}
	})
}
