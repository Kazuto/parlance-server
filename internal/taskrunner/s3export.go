package taskrunner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kazuto/parlance-server/internal/forge"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/objectstore"
	"github.com/kazuto/parlance-server/internal/server/export"
)

// manifestVersion changes when the manifest changes in ways its readers notice.
const manifestVersion = 1

// Cache-Control of an S3 export's objects: a file never changes under its hash,
// while the manifest is fetched anew each time.
const (
	fileCacheControl     = "public, max-age=31536000, immutable"
	manifestCacheControl = "no-cache"
)

// contentTypes are the types of the formats' files.
var contentTypes = map[string]string{
	export.FormatJSON: "application/json; charset=utf-8",
	export.FormatPHP:  "text/x-php; charset=utf-8",
	export.FormatPO:   "text/x-gettext-translation; charset=utf-8",
}

// manifest lists an S3 export's files, for clients to find each language's file.
type manifest struct {
	Version       int            `json:"version"`
	GeneratedAt   string         `json:"generated_at"`
	Task          string         `json:"task"`
	IncludeDrafts bool           `json:"include_drafts"`
	Files         []manifestFile `json:"files"`
}

// manifestFile is one of the files a manifest lists.
type manifestFile struct {
	Locale string `json:"locale"`
	Format string `json:"format"`
	// Template is the path the task gives the file, e.g. "i18n/{locale}.{hash}.json".
	Template string `json:"template"`
	Key      string `json:"key"`
	// SHA256 is the whole hash, however much of it the key holds.
	SHA256 string `json:"sha256"`
	Size   int    `json:"size"`
}

// s3Export renders the task's files for each locale and uploads them under keys
// holding their SHA-256, then the manifest listing them. Files the bucket has are not
// uploaded again, and an unchanged manifest is not written.
func (r *Runner) s3Export(ctx context.Context, task *models.Task, runLog *runLog) (models.TaskRunSummary, error) {
	var summary models.TaskRunSummary

	config, err := task.S3Export()
	if err != nil {
		return summary, err
	}

	if task.Secret == "" {
		return summary, stderrors.New("the task has no secret access key")
	}

	secretKey, err := r.box.Decrypt(task.Secret)
	if err != nil {
		return summary, stderrors.New("the secret access key can't be read, e.g. because ENCRYPTION_KEY changed; save it again")
	}

	rendered, locales, err := r.renderFiles(ctx, config.ExportSelection)
	if err != nil {
		return summary, err
	}

	runLog.Printf("Rendered %d files for %d languages", len(rendered), len(locales))

	files, err := hashedFiles(rendered, config.KeyHashLength())
	if err != nil {
		return summary, err
	}

	store := r.NewStore(objectstore.Config{
		Endpoint:        config.Endpoint,
		Region:          config.Region,
		Bucket:          config.Bucket,
		AccessKeyID:     config.AccessKeyID,
		SecretAccessKey: secretKey,
		PathStyle:       config.PathStyle,
	})

	previous, err := readManifest(ctx, store, config.ManifestPath, runLog)
	if err != nil {
		return summary, err
	}

	summary.ManifestURL = "s3://" + config.Bucket + "/" + config.ManifestPath

	for i, file := range files {
		uploaded, err := uploadFile(ctx, store, file, []byte(rendered[i].Content))
		if err != nil {
			return summary, err
		}

		if uploaded {
			summary.ChangedFiles = append(summary.ChangedFiles, file.Key)
			runLog.Printf("Uploaded %s", file.Key)
		}
	}

	if previous != nil && previous.IncludeDrafts == config.IncludeDrafts && slices.Equal(previous.Files, files) {
		if len(summary.ChangedFiles) == 0 {
			summary.Outcome = forge.OutcomeNoChanges

			runLog.Printf("The bucket already has these translations")
		} else {
			summary.Outcome = forge.OutcomeUpdated

			runLog.Printf("Uploaded %d files the bucket lacked; the manifest already lists them", len(summary.ChangedFiles))
		}

		return summary, nil
	}

	if err := writeManifest(ctx, store, config, task.Name, files); err != nil {
		return summary, err
	}

	summary.Outcome = forge.OutcomeUpdated

	runLog.Printf("Wrote %s, listing %d files", summary.ManifestURL, len(files))

	return summary, nil
}

// hashedFiles names each rendered file by its SHA-256, cut to hashLength digits.
// Files whose keys meet must have the same content.
func hashedFiles(rendered []renderedFile, hashLength int) ([]manifestFile, error) {
	files := make([]manifestFile, len(rendered))
	hashes := make(map[string]string, len(rendered))

	for i, file := range rendered {
		sum := sha256.Sum256([]byte(file.Content))
		hash := hex.EncodeToString(sum[:])
		key := strings.ReplaceAll(file.path(), models.HashPlaceholder, hash[:hashLength])

		if other, ok := hashes[key]; ok && other != hash {
			return nil, fmt.Errorf("two files with different content are both named %s; use a longer hash", key)
		}

		hashes[key] = hash
		files[i] = manifestFile{
			Locale:   file.Locale,
			Format:   file.File.Format,
			Template: file.File.Path,
			Key:      key,
			SHA256:   hash,
			Size:     len(file.Content),
		}
	}

	return files, nil
}

// readManifest returns the manifest the bucket holds, or nil for none. One that
// isn't a manifest is replaced.
func readManifest(ctx context.Context, store objectstore.Store, key string, runLog *runLog) (*manifest, error) {
	data, err := store.Get(ctx, key)
	if stderrors.Is(err, objectstore.ErrNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read %s: %w", key, err)
	}

	var previous manifest
	if err := json.Unmarshal(data, &previous); err != nil || previous.Version == 0 {
		runLog.Printf("%s isn't a manifest; it is replaced", key)

		return nil, nil
	}

	return &previous, nil
}

// uploadFile uploads the file unless the bucket has it, telling whether it did. An
// object stored without its hash is uploaded again, as it can't be told apart.
func uploadFile(ctx context.Context, store objectstore.Store, file manifestFile, body []byte) (bool, error) {
	stat, err := store.Stat(ctx, file.Key)
	if err != nil {
		return false, fmt.Errorf("look up %s: %w", file.Key, err)
	}

	if stat.Exists && stat.SHA256 == file.SHA256 {
		return false, nil
	}

	if stat.Exists && stat.SHA256 != "" {
		return false, fmt.Errorf("%s already holds other content, as the hash in its name is too short to tell them apart; use a longer hash", file.Key)
	}

	err = store.Put(ctx, objectstore.Object{
		Key:          file.Key,
		Body:         body,
		ContentType:  contentTypes[file.Format],
		CacheControl: fileCacheControl,
		SHA256:       file.SHA256,
	})
	if err != nil {
		return false, fmt.Errorf("upload %s: %w", file.Key, err)
	}

	return true, nil
}

// writeManifest writes the manifest listing the files.
func writeManifest(ctx context.Context, store objectstore.Store, config models.S3ExportConfig, taskName string, files []manifestFile) error {
	var body bytes.Buffer

	encoder := json.NewEncoder(&body)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(manifest{
		Version:       manifestVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Task:          taskName,
		IncludeDrafts: config.IncludeDrafts,
		Files:         files,
	}); err != nil {
		return fmt.Errorf("encode the manifest: %w", err)
	}

	sum := sha256.Sum256(body.Bytes())

	err := store.Put(ctx, objectstore.Object{
		Key:          config.ManifestPath,
		Body:         body.Bytes(),
		ContentType:  contentTypes[export.FormatJSON],
		CacheControl: manifestCacheControl,
		SHA256:       hex.EncodeToString(sum[:]),
	})
	if err != nil {
		return fmt.Errorf("write %s: %w", config.ManifestPath, err)
	}

	return nil
}
