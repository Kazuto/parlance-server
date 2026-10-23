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
)

var (
	repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	branchPattern     = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
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
		SourceLocaleID:   in.SourceLocaleId,
		TargetLocaleIDs:  targets,
		ScopeIDs:         scopes,
		ApplyTerminology: in.ApplyTerminology,
		MaxPairs:         int(in.MaxPairs),
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

	if len(in.Files) == 0 {
		return config, errors.InvalidInput(errors.ErrInvalidInput, "at least one file is required")
	}

	if len(in.Files) > maxExportFiles {
		return config, errors.InvalidInput(errors.ErrInvalidInput, "too many files")
	}

	for _, file := range in.Files {
		format := converter.ExportFormatFromProto(file.Format)
		if format == "" {
			return config, errors.InvalidInput(errors.ErrInvalidInput, "each file needs a format")
		}

		filePath := strings.TrimSpace(file.Path)
		if err := validateFilePath(filePath); err != nil {
			return config, err
		}

		if slices.ContainsFunc(config.Files, func(other models.GitExportFile) bool { return other.Path == filePath }) {
			return config, errors.InvalidInput(errors.ErrInvalidInput, "two files have the same path")
		}

		config.Files = append(config.Files, models.GitExportFile{Format: format, Path: filePath})
	}

	config.LocaleIDs = common.UniqueIDs(in.LocaleIds)
	if err := requireLocales(db, config.LocaleIDs); err != nil {
		return config, err
	}

	config.ScopeIDs = common.UniqueIDs(in.ScopeIds)
	if err := requireScopes(db, config.ScopeIDs); err != nil {
		return config, err
	}

	config.IncludeDrafts = in.IncludeDrafts

	return config, nil
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

func requireLocales(db *gorm.DB, ids []string) error {
	_, err := common.ResolveByIDs[models.Locale](db, ids, errors.ErrLocaleNotFound, "one or more languages do not exist")

	return err
}

func requireScopes(db *gorm.DB, ids []string) error {
	_, err := common.ResolveByIDs[models.Scope](db, ids, errors.ErrScopeNotFound, "one or more scopes do not exist")

	return err
}
