package taskrunner

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/kazuto/parlance-server/internal/forge"
	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/server/export"
)

// gitExport renders the task's files for each locale and proposes them as a pull
// request. Nothing is pushed while the repository already has them.
func (r *Runner) gitExport(ctx context.Context, task *models.Task, runLog *runLog) (models.TaskRunSummary, error) {
	var summary models.TaskRunSummary

	config, err := task.GitExport()
	if err != nil {
		return summary, err
	}

	if task.Secret == "" {
		return summary, stderrors.New("the task has no access token")
	}

	token, err := r.box.Decrypt(task.Secret)
	if err != nil {
		return summary, stderrors.New("the access token can't be read, e.g. because ENCRYPTION_KEY changed; save it again")
	}

	files, locales, err := r.renderFiles(ctx, config)
	if err != nil {
		return summary, err
	}

	runLog.Printf("Rendered %d files for %d languages", len(files), len(locales))

	result, err := r.NewHost(config.APIURL, token).Propose(ctx, config.Repository, forge.Proposal{
		BaseBranch:    config.BaseBranch,
		Branch:        config.Branch,
		CommitMessage: "Update translations from Parlance",
		Title:         "Update translations",
		Body:          pullRequestBody(task, config, locales),
		Files:         files,
	})
	if err != nil {
		return summary, err
	}

	summary.ChangedFiles = result.ChangedFiles
	summary.PullRequestURL = result.PullRequestURL
	summary.Outcome = result.Outcome

	switch result.Outcome {
	case forge.OutcomeNoChanges:
		runLog.Printf("The repository already has these translations")
	case forge.OutcomeUpToDate:
		runLog.Printf("The pull request branch is up to date")
	default:
		runLog.Printf("Changed %d files: %s", len(result.ChangedFiles), strings.Join(result.ChangedFiles, ", "))
	}

	if summary.PullRequestURL != "" {
		runLog.Printf("Pull request: %s", summary.PullRequestURL)
	}

	return summary, nil
}

// renderFiles renders each of the config's files for each of its locales.
func (r *Runner) renderFiles(ctx context.Context, config models.GitExportConfig) ([]forge.File, []models.Locale, error) {
	db := r.db.WithContext(ctx)

	query := db.Order("code")
	if len(config.LocaleIDs) > 0 {
		query = query.Where("id IN ?", config.LocaleIDs)
	}

	var locales []models.Locale
	if err := query.Find(&locales).Error; err != nil {
		return nil, nil, err
	}

	if len(locales) == 0 {
		return nil, nil, stderrors.New("no languages to export")
	}

	scopeIDs := common.UniqueIDs(config.ScopeIDs)
	files := make([]forge.File, 0, len(config.Files)*len(locales))

	for _, file := range config.Files {
		for _, locale := range locales {
			content, err := export.Render(db, file.Format, locale, scopeIDs, config.IncludeDrafts)
			if err != nil {
				return nil, nil, fmt.Errorf("render %s for %s: %w", file.Format, locale.Code, err)
			}

			files = append(files, forge.File{
				Path:    strings.ReplaceAll(file.Path, models.LocalePlaceholder, locale.Code),
				Content: content,
			})
		}
	}

	return files, locales, nil
}

// pullRequestBody says where the pull request comes from and what it holds.
func pullRequestBody(task *models.Task, config models.GitExportConfig, locales []models.Locale) string {
	codes := make([]string, len(locales))
	for i, locale := range locales {
		codes[i] = "`" + locale.Code + "`"
	}

	var body strings.Builder

	fmt.Fprintf(&body, "Translations exported by the Parlance task **%s**.\n\n", task.Name)
	fmt.Fprintf(&body, "- Languages: %s\n", strings.Join(codes, ", "))

	for _, file := range config.Files {
		fmt.Fprintf(&body, "- `%s` (%s)\n", file.Path, file.Format)
	}

	if config.IncludeDrafts {
		body.WriteString("\nIncludes drafts that still await review.\n")
	} else {
		body.WriteString("\nOnly approved translations are included.\n")
	}

	body.WriteString("\nThis branch is rebuilt on every export; changes pushed to it are replaced.\n")

	return body.String()
}
