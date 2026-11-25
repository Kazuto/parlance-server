package models

import (
	"encoding/json"
	"time"

	"gorm.io/datatypes"
)

// Task types
const (
	TaskTypeAITranslate = "ai_translate"
	TaskTypeGitExport   = "git_export"
	TaskTypeS3Export    = "s3_export"
)

// IsExportType tells whether tasks of the type export translations, with a secret
// to reach where they go.
func IsExportType(taskType string) bool {
	return taskType == TaskTypeGitExport || taskType == TaskTypeS3Export
}

// Events a task can run after
const (
	EventTranslationsApproved = "translations_approved"
	EventTranslationsReleased = "translations_released"
)

// What started a task run
const (
	TriggerManual   = "manual"
	TriggerSchedule = "schedule"
	TriggerEvent    = "event"
)

// Task run states
const (
	RunQueued    = "queued"
	RunRunning   = "running"
	RunSucceeded = "succeeded"
	RunFailed    = "failed"
	RunCanceled  = "canceled"
)

// Task is work the server does on its own: on a schedule, after events, or when
// someone starts it. Its Config holds the settings of its type.
type Task struct {
	ID      string `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Name    string `gorm:"type:varchar(100);not null" json:"name"`
	Type    string `gorm:"type:varchar(30);not null" json:"type"`
	Enabled bool   `gorm:"not null" json:"enabled"`
	// Schedule is a standard 5-field cron expression in UTC; empty for none.
	Schedule  string     `gorm:"type:varchar(100);not null;default:''" json:"schedule"`
	NextRunAt *time.Time `gorm:"index" json:"next_run_at,omitempty"`
	// Events the task runs after, EventDelayMinutes after the first of them.
	// Without a default in the tag, GORM saves a zero delay instead of skipping it.
	Events            datatypes.JSONSlice[string] `gorm:"type:jsonb" json:"events"`
	EventDelayMinutes int                         `gorm:"not null" json:"event_delay_minutes"`
	Config            datatypes.JSON              `gorm:"type:jsonb" json:"config"`
	// Secret, e.g. a git host token or an S3 secret key, is encrypted with the server's encryption key.
	Secret    string    `gorm:"type:text" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy *string   `gorm:"type:uuid" json:"created_by,omitempty"`
	UpdatedBy *string   `gorm:"type:uuid" json:"updated_by,omitempty"`
}

// AITranslateConfig fills missing translations with machine translations, as drafts.
type AITranslateConfig struct {
	// SourceLocaleID is the language translated from; empty for the default locale.
	SourceLocaleID string `json:"source_locale_id,omitempty"`
	// TargetLocaleIDs are the languages to fill; empty for all but the source.
	TargetLocaleIDs  []string `json:"target_locale_ids,omitempty"`
	ScopeIDs         []string `json:"scope_ids,omitempty"`
	ApplyTerminology bool     `json:"apply_terminology,omitempty"`
	// RedoMachineDrafts also translates again unedited machine drafts by models the
	// translation servers no longer serve.
	RedoMachineDrafts bool `json:"redo_machine_drafts,omitempty"`
	// MaxPairs caps the entry × language pairs one run translates.
	MaxPairs int `json:"max_pairs,omitempty"`
}

// Placeholders in an export's file paths
const (
	// LocalePlaceholder stands for a locale's code.
	LocalePlaceholder = "{locale}"
	// HashPlaceholder stands for the file's SHA-256, in an S3 export's object keys.
	HashPlaceholder = "{hash}"
)

// ExportFile is one export format and where its files go, e.g. "lang/{locale}.json".
type ExportFile struct {
	Format string `json:"format"`
	Path   string `json:"path"`
}

// ExportSelection is what an export task renders: its files, for which languages
// and entries.
type ExportSelection struct {
	Files     []ExportFile `json:"files"`
	LocaleIDs []string     `json:"locale_ids,omitempty"`
	ScopeIDs  []string     `json:"scope_ids,omitempty"`
	// TagIDs narrows each scope to the entries with any of its selected tags.
	TagIDs        []string `json:"tag_ids,omitempty"`
	IncludeDrafts bool     `json:"include_drafts,omitempty"`
}

// GitExportConfig exports translations into a repository, as a pull request.
type GitExportConfig struct {
	// APIURL is the host's API, e.g. "https://api.github.com".
	APIURL string `json:"api_url,omitempty"`
	// Repository is "owner/name".
	Repository string `json:"repository"`
	BaseBranch string `json:"base_branch"`
	// Branch is the pull request's branch, rebuilt from BaseBranch on each run.
	Branch string `json:"branch"`
	ExportSelection
}

// FullHashLength is the number of hex digits of a SHA-256.
const FullHashLength = 64

// S3ExportConfig uploads translations to an S3-compatible bucket, each file named by
// its hash, with a manifest listing them. Its secret access key is the task's Secret.
type S3ExportConfig struct {
	// Endpoint is the storage's address, e.g. "https://minio.example.com"; empty for
	// Amazon S3.
	Endpoint string `json:"endpoint,omitempty"`
	// Region is empty for "us-east-1".
	Region      string `json:"region,omitempty"`
	Bucket      string `json:"bucket"`
	AccessKeyID string `json:"access_key_id"`
	// PathStyle puts the bucket into the path instead of the host name, as MinIO needs.
	PathStyle bool `json:"path_style,omitempty"`
	// ManifestPath is the key of the manifest, e.g. "i18n/manifest.json".
	ManifestPath string `json:"manifest_path"`
	// HashLength is how many hex digits of the SHA-256 go into the keys; 0 for all.
	HashLength int `json:"hash_length,omitempty"`
	ExportSelection
}

// KeyHashLength is how many hex digits of the SHA-256 go into the keys.
func (c S3ExportConfig) KeyHashLength() int {
	if c.HashLength <= 0 || c.HashLength > FullHashLength {
		return FullHashLength
	}

	return c.HashLength
}

// AITranslate decodes the config of an ai_translate task.
func (t *Task) AITranslate() (AITranslateConfig, error) {
	var config AITranslateConfig

	return config, decodeConfig(t.Config, &config)
}

// GitExport decodes the config of a git_export task.
func (t *Task) GitExport() (GitExportConfig, error) {
	var config GitExportConfig

	return config, decodeConfig(t.Config, &config)
}

// S3Export decodes the config of an s3_export task.
func (t *Task) S3Export() (S3ExportConfig, error) {
	var config S3ExportConfig

	return config, decodeConfig(t.Config, &config)
}

func decodeConfig(raw datatypes.JSON, config any) error {
	if len(raw) == 0 {
		return nil
	}

	return json.Unmarshal(raw, config)
}

// TaskRun is one run of a task, from being queued until it ends.
type TaskRun struct {
	ID          string  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TaskID      string  `gorm:"type:uuid;not null;index" json:"task_id"`
	Task        *Task   `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Trigger     string  `gorm:"type:varchar(20);not null" json:"trigger"`
	TriggeredBy *string `gorm:"type:uuid" json:"triggered_by,omitempty"`
	// TriggeredByUser is who started the run by hand, when preloaded.
	TriggeredByUser *User  `gorm:"foreignKey:TriggeredBy" json:"triggered_by_user,omitempty"`
	Status          string `gorm:"type:varchar(20);not null;index" json:"status"`
	// RunAfter is when a queued run may start.
	RunAfter        time.Time  `gorm:"not null;index" json:"run_after"`
	CancelRequested bool       `gorm:"not null;default:false" json:"cancel_requested"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	// HeartbeatAt shows a running run is still alive; runs that stop beating failed.
	HeartbeatAt *time.Time     `json:"heartbeat_at,omitempty"`
	FinishedAt  *time.Time     `json:"finished_at,omitempty"`
	Summary     datatypes.JSON `gorm:"type:jsonb" json:"summary"`
	Log         string         `gorm:"type:text;not null;default:''" json:"log"`
	Error       string         `gorm:"type:text;not null;default:''" json:"error"`
	CreatedAt   time.Time      `json:"created_at"`
}

// TaskRunSummary is what a run did, in counts and links.
type TaskRunSummary struct {
	// Machine translation
	Translated int `json:"translated,omitempty"`
	Failed     int `json:"failed,omitempty"`
	// Remaining are the missing translations left for later runs.
	Remaining int `json:"remaining,omitempty"`
	// Git and S3 export
	PullRequestURL string `json:"pull_request_url,omitempty"`
	// ChangedFiles are a git export's changed paths, or an S3 export's uploaded keys.
	ChangedFiles []string `json:"changed_files,omitempty"`
	// ManifestURL is an S3 export's manifest, as "s3://bucket/key".
	ManifestURL string `json:"manifest_url,omitempty"`
	// Outcome says how the run ended, e.g. forge.OutcomeNoChanges.
	Outcome string `json:"outcome,omitempty"`
	// Redone counts machine drafts translated again; Translated includes them.
	Redone int `json:"redone,omitempty"`
}

// RunSummary decodes the summary of the run; one that can't be read is empty.
func (r *TaskRun) RunSummary() TaskRunSummary {
	var summary TaskRunSummary
	if err := decodeConfig(r.Summary, &summary); err != nil {
		return TaskRunSummary{}
	}

	return summary
}

// Finished tells whether the run has ended.
func (r *TaskRun) Finished() bool {
	return r.Status != RunQueued && r.Status != RunRunning
}
