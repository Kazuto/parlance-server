package task

import (
	"encoding/json"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/converter"
	"github.com/kazuto/parlance-server/internal/errors"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/taskqueue"
	"gorm.io/gorm"
)

// Limits of a task's settings
const (
	maxNameLength   = 100
	maxEventDelay   = 24 * 60
	maxPairsLimit   = 5000
	maxExportFiles  = 10
	maxBranchLength = 200
	// S3 limits keys to 1024 bytes; a template leaves room for the code and hash.
	maxKeyLength  = 900
	minHashLength = 8
)

var (
	repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	branchPattern     = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	// Lenient, as S3-compatible storages differ in what they allow.
	bucketPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{1,254}$`)
	regionPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

// settings are a task's settings as a create or update request gives them.
type settings struct {
	name      string
	enabled   bool
	schedule  string
	events    []pb.TaskEvent
	delay     int32
	aiConfig  *pb.AiTranslateConfig
	gitConfig *pb.GitExportConfig
	s3Config  *pb.S3ExportConfig
	now       time.Time
}

// apply checks the settings and sets them on the task, whose Type must be set.
// The next scheduled run is worked out anew.
func (in settings) apply(db *gorm.DB, task *models.Task) error {
	name := strings.TrimSpace(in.name)
	if name == "" {
		return errors.InvalidInput(errors.ErrInvalidInput, "name is required")
	}

	if len([]rune(name)) > maxNameLength {
		return errors.InvalidInput(errors.ErrInvalidInput, "name is too long")
	}

	schedule := strings.TrimSpace(in.schedule)
	if schedule != "" {
		if _, err := taskqueue.ParseSchedule(schedule); err != nil {
			return errors.InvalidInput(errors.ErrInvalidInput, "schedule is not a valid cron expression")
		}
	}

	if in.delay < 0 || in.delay > maxEventDelay {
		return errors.InvalidInput(errors.ErrInvalidInput, "event delay must be between 0 and 1440 minutes")
	}

	events := []string{}

	for _, value := range in.events {
		event := converter.TaskEventFromProto(value)
		if event == "" {
			return errors.InvalidInput(errors.ErrInvalidInput, "unknown event")
		}

		if !slices.Contains(events, event) {
			events = append(events, event)
		}
	}

	var (
		config any
		err    error
	)

	switch task.Type {
	case models.TaskTypeAITranslate:
		config, err = aiTranslateConfig(db, in.aiConfig)
	case models.TaskTypeGitExport:
		config, err = gitExportConfig(db, in.gitConfig)
	case models.TaskTypeS3Export:
		config, err = s3ExportConfig(db, in.s3Config)
	default:
		err = errors.InvalidInput(errors.ErrInvalidInput, "type is required")
	}

	if err != nil {
		return err
	}

	encoded, err := json.Marshal(config)
	if err != nil {
		return errors.InternalCause(err, "failed to save the task settings")
	}

	task.Name = name
	task.Enabled = in.enabled
	task.Schedule = schedule
	task.Events = events
	task.EventDelayMinutes = int(in.delay)
	task.Config = encoded

	next, err := taskqueue.NextRun(task, in.now)
	if err != nil {
		return errors.InvalidInput(errors.ErrInvalidInput, "schedule is not a valid cron expression")
	}

	task.NextRunAt = next

	return nil
}

func aiTranslateConfig(db *gorm.DB, in *pb.AiTranslateConfig) (models.AITranslateConfig, error) {
	var config models.AITranslateConfig

	if in == nil {
		in = &pb.AiTranslateConfig{}
	}

	if in.MaxPairs < 0 || in.MaxPairs > maxPairsLimit {
		return config, errors.InvalidInput(errors.ErrInvalidInput, "max pairs must be between 0 and 5000")
	}

	if in.SourceLocaleId != "" {
		if err := requireLocales(db, []string{in.SourceLocaleId}); err != nil {
			return config, err
		}
	}

	targets := common.UniqueIDs(in.TargetLocaleIds)
	if slices.Contains(targets, in.SourceLocaleId) {
		return config, errors.InvalidInput(errors.ErrInvalidInput, "the source language can't be a target")
	}

	if err := requireLocales(db, targets); err != nil {
		return config, err
	}

	scopes := common.UniqueIDs(in.ScopeIds)
	if err := requireScopes(db, scopes); err != nil {
		return config, err
	}

	config = models.AITranslateConfig{
		SourceLocaleID:    in.SourceLocaleId,
		TargetLocaleIDs:   targets,
		ScopeIDs:          scopes,
		ApplyTerminology:  in.ApplyTerminology,
		RedoMachineDrafts: in.RedoMachineDrafts,
		MaxPairs:          int(in.MaxPairs),
	}

	return config, nil
}

// gitExportConfig checks a git export's settings. They don't touch the host; that
// is what CheckTaskAccess is for.
func gitExportConfig(db *gorm.DB, in *pb.GitExportConfig) (models.GitExportConfig, error) {
	config, err := checkRepository(in)
	if err != nil {
		return config, err
	}

	config.ExportSelection, err = exportSelection(db, in, validateFilePath)

	return config, err
}

// s3ExportConfig checks an S3 export's settings. They don't touch the storage; that
// is what CheckTaskAccess is for.
func s3ExportConfig(db *gorm.DB, in *pb.S3ExportConfig) (models.S3ExportConfig, error) {
	config, err := checkBucket(in)
	if err != nil {
		return config, err
	}

	config.ExportSelection, err = exportSelection(db, in, validateKeyTemplate)
	if err != nil {
		return config, err
	}

	if slices.ContainsFunc(config.Files, func(file models.ExportFile) bool { return file.Path == config.ManifestPath }) {
		return config, errors.InvalidInput(errors.ErrInvalidInput, "the manifest path must differ from the file paths")
	}

	return config, nil
}

// exportInput is what the settings of git and S3 exports share.
type exportInput interface {
	GetFiles() []*pb.GitExportFile
	GetLocaleIds() []string
	GetScopeIds() []string
	GetTagIds() []string
	GetIncludeDrafts() bool
}

// exportSelection checks what an export renders: its files, whose paths must pass
// validatePath, and the languages, scopes and tags.
func exportSelection(db *gorm.DB, in exportInput, validatePath func(string) error) (models.ExportSelection, error) {
	var selection models.ExportSelection

	files := in.GetFiles()
	if len(files) == 0 {
		return selection, errors.InvalidInput(errors.ErrInvalidInput, "at least one file is required")
	}

	if len(files) > maxExportFiles {
		return selection, errors.InvalidInput(errors.ErrInvalidInput, "too many files")
	}

	for _, file := range files {
		format := converter.ExportFormatFromProto(file.Format)
		if format == "" {
			return selection, errors.InvalidInput(errors.ErrInvalidInput, "each file needs a format")
		}

		filePath := strings.TrimSpace(file.Path)
		if err := validatePath(filePath); err != nil {
			return selection, err
		}

		if slices.ContainsFunc(selection.Files, func(other models.ExportFile) bool { return other.Path == filePath }) {
			return selection, errors.InvalidInput(errors.ErrInvalidInput, "two files have the same path")
		}

		selection.Files = append(selection.Files, models.ExportFile{Format: format, Path: filePath})
	}

	selection.LocaleIDs = common.UniqueIDs(in.GetLocaleIds())
	if err := requireLocales(db, selection.LocaleIDs); err != nil {
		return selection, err
	}

	selection.ScopeIDs = common.UniqueIDs(in.GetScopeIds())
	if err := requireScopes(db, selection.ScopeIDs); err != nil {
		return selection, err
	}

	selection.TagIDs = common.UniqueIDs(in.GetTagIds())
	if _, err := common.ResolveActiveTags(db, selection.TagIDs); err != nil {
		return selection, err
	}

	selection.IncludeDrafts = in.GetIncludeDrafts()

	return selection, nil
}

// checkRepository checks where a git export goes: the host, repository and branches.
func checkRepository(in *pb.GitExportConfig) (models.GitExportConfig, error) {
	var config models.GitExportConfig
	if in == nil {
		return config, errors.InvalidInput(errors.ErrInvalidInput, "git export settings are required")
	}

	config.APIURL = strings.TrimRight(strings.TrimSpace(in.ApiUrl), "/")
	if config.APIURL != "" {
		parsed, err := url.Parse(config.APIURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return config, errors.InvalidInput(errors.ErrInvalidInput, "api url must be an http or https URL")
		}
	}

	config.Repository = strings.TrimSpace(in.Repository)
	if !repositoryPattern.MatchString(config.Repository) {
		return config, errors.InvalidInput(errors.ErrInvalidInput, `repository must look like "owner/name"`)
	}

	config.BaseBranch = strings.TrimSpace(in.BaseBranch)
	config.Branch = strings.TrimSpace(in.Branch)

	for _, branch := range []string{config.BaseBranch, config.Branch} {
		if !validBranch(branch) {
			return config, errors.InvalidInput(errors.ErrInvalidInput, "branch names may hold letters, digits and . _ / -")
		}
	}

	if config.BaseBranch == config.Branch {
		return config, errors.InvalidInput(errors.ErrInvalidInput, "the pull request branch must differ from the base branch")
	}

	return config, nil
}

// checkBucket checks where an S3 export goes: the storage, bucket, key and manifest.
func checkBucket(in *pb.S3ExportConfig) (models.S3ExportConfig, error) {
	var config models.S3ExportConfig
	if in == nil {
		return config, errors.InvalidInput(errors.ErrInvalidInput, "S3 export settings are required")
	}

	config.Endpoint = strings.TrimRight(strings.TrimSpace(in.Endpoint), "/")
	if config.Endpoint != "" {
		parsed, err := url.Parse(config.Endpoint)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
			parsed.RawQuery != "" || parsed.Fragment != "" {
			return config, errors.InvalidInput(errors.ErrInvalidInput, "endpoint must be an http or https URL")
		}
	}

	config.Region = strings.TrimSpace(in.Region)
	if config.Region != "" && !regionPattern.MatchString(config.Region) {
		return config, errors.InvalidInput(errors.ErrInvalidInput, "region may hold letters, digits, _ and -")
	}

	config.Bucket = strings.TrimSpace(in.Bucket)
	if !bucketPattern.MatchString(config.Bucket) {
		return config, errors.InvalidInput(errors.ErrInvalidInput, "bucket names may hold letters, digits and . _ -")
	}

	config.AccessKeyID = strings.TrimSpace(in.AccessKeyId)
	if config.AccessKeyID == "" {
		return config, errors.InvalidInput(errors.ErrInvalidInput, "access key id is required")
	}

	config.PathStyle = in.PathStyle

	config.ManifestPath = strings.TrimSpace(in.ManifestPath)
	if strings.Contains(config.ManifestPath, models.LocalePlaceholder) || strings.Contains(config.ManifestPath, models.HashPlaceholder) {
		return config, errors.InvalidInput(errors.ErrInvalidInput, "the manifest path can't hold {locale} or {hash}")
	}

	if err := validateKey(config.ManifestPath); err != nil {
		return config, err
	}

	if in.HashLength != 0 && (in.HashLength < minHashLength || in.HashLength > models.FullHashLength) {
		return config, errors.InvalidInput(errors.ErrInvalidInput, "hash length must be between 8 and 64")
	}

	config.HashLength = int(in.HashLength)

	return config, nil
}

// validBranch accepts the usual branch names, and none git would reject.
func validBranch(branch string) bool {
	return branch != "" && len(branch) <= maxBranchLength && branchPattern.MatchString(branch) &&
		!strings.Contains(branch, "..") && !strings.Contains(branch, "//") &&
		!strings.HasPrefix(branch, "/") && !strings.HasSuffix(branch, "/") &&
		!strings.HasSuffix(branch, ".lock") && !strings.HasPrefix(branch, "-")
}

// validateFilePath accepts a clean relative path that names each locale's file.
func validateFilePath(filePath string) error {
	if !strings.Contains(filePath, models.LocalePlaceholder) {
		return errors.InvalidInput(errors.ErrInvalidInput, "each file path needs {locale}, e.g. lang/{locale}.json")
	}

	if strings.HasPrefix(filePath, "/") || strings.Contains(filePath, `\`) || path.Clean(filePath) != filePath ||
		slices.Contains(strings.Split(filePath, "/"), "..") || strings.HasPrefix(filePath, ".git/") {
		return errors.InvalidInput(errors.ErrInvalidInput, "file paths must be relative to the repository, without .. or empty parts")
	}

	return nil
}

// validateKeyTemplate accepts a clean key that names each locale's file by its hash.
func validateKeyTemplate(key string) error {
	if !strings.Contains(key, models.LocalePlaceholder) || !strings.Contains(key, models.HashPlaceholder) {
		return errors.InvalidInput(errors.ErrInvalidInput, "each file path needs {locale} and {hash}, e.g. i18n/{locale}.{hash}.json")
	}

	return validateKey(key)
}

// validateKey accepts a key that reads like a relative path, so that the bucket can
// be browsed and served as folders.
func validateKey(key string) error {
	if key == "" || len(key) > maxKeyLength || strings.HasPrefix(key, "/") || strings.Contains(key, `\`) ||
		path.Clean(key) != key || slices.ContainsFunc(strings.Split(key, "/"), func(part string) bool { return part == "." || part == ".." }) {
		return errors.InvalidInput(errors.ErrInvalidInput, "paths in the bucket must be relative, without . or .. or empty parts")
	}

	return nil
}

func requireLocales(db *gorm.DB, ids []string) error {
	_, err := common.ResolveByIDs[models.Locale](db, ids, errors.ErrLocaleNotFound, "one or more languages do not exist")

	return err
}

func requireScopes(db *gorm.DB, ids []string) error {
	_, err := common.ResolveByIDs[models.Scope](db, ids, errors.ErrScopeNotFound, "one or more scopes do not exist")

	return err
}
