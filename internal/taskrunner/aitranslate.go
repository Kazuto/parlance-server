package taskrunner

import (
	"context"
	stderrors "errors"
	"strings"
	"sync"
	"time"

	"github.com/kazuto/parlance-server/internal/models"
	"github.com/kazuto/parlance-server/internal/server/common"
	"github.com/kazuto/parlance-server/internal/server/localization"
	"github.com/kazuto/parlance-server/internal/translate"
	"github.com/kazuto/parlance-server/internal/webhook"
	"gorm.io/gorm"
)

// OutcomeNothingToTranslate ends a run without languages to translate into.
const OutcomeNothingToTranslate = "nothing_to_translate"

// defaultMaxPairs caps the translations one run makes unless the task says otherwise.
const defaultMaxPairs = 200

// maxParallelTranslations limits the requests a run sends at once; the translation
// servers' own limits apply on top.
const maxParallelTranslations = 4

// missingPair is an entry without a translation in a locale, with its source text,
// or with a machine draft to translate again, which ExistingID names.
type missingPair struct {
	EntryID     string
	Key         string
	Description string
	Source      string
	LocaleID    string
	ExistingID  string
}

// aiTranslate fills missing translations with machine translations, saved as drafts
// without an author. Existing translations, drafts included, are kept, except that
// the task may translate again unedited machine drafts of models no longer served.
func (r *Runner) aiTranslate(ctx context.Context, task *models.Task, runLog *runLog) (models.TaskRunSummary, error) {
	var summary models.TaskRunSummary

	config, err := task.AITranslate()
	if err != nil {
		return summary, err
	}

	if r.translators == nil {
		return summary, errNoTranslator
	}

	translator, err := r.translators.Translator(ctx)
	if err != nil {
		return summary, err
	}

	if translator == nil {
		return summary, errNoTranslator
	}

	db := r.db.WithContext(ctx)

	style, err := localization.Style(ctx, db)
	if err != nil {
		return summary, err
	}

	source, err := r.sourceLocale(ctx, config.SourceLocaleID)
	if err != nil {
		return summary, err
	}

	targetQuery := db.Where("id <> ?", source.ID).Order("code")
	if len(config.TargetLocaleIDs) > 0 {
		targetQuery = targetQuery.Where("id IN ?", config.TargetLocaleIDs)
	}

	var targets []models.Locale
	if err := targetQuery.Find(&targets).Error; err != nil {
		return summary, err
	}

	if len(targets) == 0 {
		summary.Outcome = OutcomeNothingToTranslate

		runLog.Printf("No languages to translate into")

		return summary, nil
	}

	locales := make(map[string]models.Locale, len(targets))
	targetIDs := make([]string, len(targets))

	for i, target := range targets {
		locales[target.ID] = target
		targetIDs[i] = target.ID
	}

	// Drafts by other models are found by the models the servers serve now.
	var currentModels []string
	if config.RedoMachineDrafts {
		currentModels = r.currentModels(ctx, translator, runLog)
	}

	missing := common.FilterEntriesByScopes(db.Table("entries"), config.ScopeIDs).
		Joins("JOIN localizations source ON source.entry_id = entries.id AND source.locale_id = ? AND source.deleted_at IS NULL AND TRIM(source.translation) <> ''", source.ID).
		Joins("CROSS JOIN locales").
		Joins("LEFT JOIN localizations target ON target.entry_id = entries.id AND target.locale_id = locales.id AND target.deleted_at IS NULL").
		Where("entries.deleted_at IS NULL AND locales.deleted_at IS NULL AND locales.id IN ?", targetIDs)

	if len(currentModels) > 0 {
		missing = missing.Where("target.id IS NULL OR (target.origin = ? AND target.status = ? AND target.review_comment = '' AND target.machine_model NOT IN ?)",
			models.OriginMachine, models.StatusDraft, currentModels)
	} else {
		missing = missing.Where("target.id IS NULL")
	}

	missing = missing.Session(&gorm.Session{}) // counted, then listed

	var total int64
	if err := missing.Count(&total).Error; err != nil {
		return summary, err
	}

	maxPairs := config.MaxPairs
	if maxPairs <= 0 {
		maxPairs = defaultMaxPairs
	}

	var pairs []missingPair
	if err := missing.
		Select("entries.id AS entry_id, entries.key, entries.description, source.translation AS source, locales.id AS locale_id, COALESCE(target.id::text, '') AS existing_id").
		// Missing translations come first; redoing drafts can wait for later runs.
		Order("target.id IS NOT NULL, entries.key, locales.code").
		Limit(maxPairs).
		Scan(&pairs).Error; err != nil {
		return summary, err
	}

	runLog.Printf("%d translations to write from %s, translating %d", total, source.Code, len(pairs))

	var glossary *localization.Glossary

	if config.ApplyTerminology {
		if glossary, err = localization.LoadGlossary(ctx, db, source.Code, targetIDs); err != nil {
			return summary, err
		}
	}

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		limit   = make(chan struct{}, maxParallelTranslations)
		drafted []string
	)

	for _, pair := range pairs {
		if ctx.Err() != nil {
			break
		}

		limit <- struct{}{}

		wg.Go(func() {
			defer func() { <-limit }()

			locale := locales[pair.LocaleID]
			id := r.translatePair(ctx, translator, glossary, style, pair, locale, runLog)

			mu.Lock()
			defer mu.Unlock()

			if id != "" {
				summary.Translated++
				if pair.ExistingID != "" {
					summary.Redone++
				}

				drafted = append(drafted, id)
			} else {
				summary.Failed++
			}
		})
	}

	wg.Wait()

	r.emitDrafted(task, drafted, runLog)

	summary.Remaining = int(total) - summary.Translated

	if ctx.Err() != nil {
		return summary, ctx.Err()
	}

	if summary.Translated == 0 && summary.Failed > 0 {
		return summary, errAllFailed
	}

	return summary, nil
}

// Why a machine translation run can't do its work
var (
	errNoTranslator = stderrors.New("machine translation is not configured: add a translation server in the settings")
	errAllFailed    = stderrors.New("every translation failed, see the log")
)

// emitDrafted sends the run's drafts to webhooks as one event. The drafts are saved
// already, also when the run was cut short, so this uses its own context.
func (r *Runner) emitDrafted(task *models.Task, drafted []string, runLog *runLog) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := webhook.EmitTranslations(r.db.WithContext(ctx), models.WebhookEventTranslationsAwaitingApproval, drafted, nil, task); err != nil {
		runLog.Printf("The drafts could not be sent to webhooks: %v", err)
	}
}

// translatePair translates one entry into one locale and saves it as a draft,
// returning its id, or "" when it didn't. A result a reviewer would have to fix
// first, e.g. one that lost a placeholder, is not saved.
func (r *Runner) translatePair(
	ctx context.Context,
	translator translate.Translator,
	glossary *localization.Glossary,
	style string,
	pair missingPair,
	locale models.Locale,
	runLog *runLog,
) string {
	var terms []translate.Term

	if glossary != nil {
		found, err := glossary.Terms(pair.Source)
		if err != nil {
			runLog.Printf("%s → %s: glossary not applied, %v", pair.Key, locale.Code, err)
		}

		terms = found[locale.ID]
	}

	source := localization.Source{Text: pair.Source, Context: pair.Description, Style: style}

	suggestion := localization.SuggestOne(ctx, translator, source, locale, terms)
	if suggestion.ErrorCode != "" {
		runLog.Printf("%s → %s: %s", pair.Key, locale.Code, suggestion.ErrorCode)

		return ""
	}

	if len(suggestion.Warnings) > 0 {
		runLog.Printf("%s → %s: left out, %s", pair.Key, locale.Code, suggestion.Warnings[0].Code)

		return ""
	}

	if pair.ExistingID != "" {
		return r.redoDraft(ctx, pair, locale, suggestion.Translation, suggestion.Model, runLog)
	}

	draft := models.Localization{
		EntryID:     pair.EntryID,
		LocaleID:    locale.ID,
		Translation: suggestion.Translation,
		Review:      models.Review{Status: models.StatusDraft},
	}
	draft.WrittenBy(suggestion.Model)

	// Someone may have translated it in the meantime; theirs stays.
	if err := r.db.WithContext(ctx).Create(&draft).Error; err != nil {
		runLog.Printf("%s → %s: not saved, %v", pair.Key, locale.Code, err)

		return ""
	}

	return draft.ID
}

// redoDraft replaces a machine draft with the new translation, unless someone
// edited, approved or rejected it since it was picked. The old text stays in the
// history.
func (r *Runner) redoDraft(ctx context.Context, pair missingPair, locale models.Locale, text, model string, runLog *runLog) string {
	var draft models.Localization
	if err := r.db.WithContext(ctx).First(&draft, "id = ?", pair.ExistingID).Error; err != nil {
		runLog.Printf("%s → %s: not saved, %v", pair.Key, locale.Code, err)

		return ""
	}

	if !draft.IsMachine() || draft.Status != models.StatusDraft || draft.ReviewComment != "" {
		runLog.Printf("%s → %s: kept, it changed since the run started", pair.Key, locale.Code)

		return ""
	}

	draft.Translation = text
	draft.UpdatedBy = nil
	draft.WrittenBy(model)
	draft.ClearOutdated()

	if err := r.db.WithContext(ctx).Save(&draft).Error; err != nil {
		runLog.Printf("%s → %s: not saved, %v", pair.Key, locale.Code, err)

		return ""
	}

	return draft.ID
}

// currentModels returns the models the translation servers serve, or none when
// they can't tell, in which case no draft is translated again: any could look as
// if another model wrote it.
func (r *Runner) currentModels(ctx context.Context, translator translate.Translator, runLog *runLog) []string {
	lister, ok := translator.(translate.ModelLister)
	if !ok {
		runLog.Printf("The translation servers can't tell their models, so no drafts are translated again")

		return nil
	}

	served, err := lister.Models(ctx)
	if err != nil || len(served) == 0 {
		runLog.Printf("The translation servers' models could not be read, so no drafts are translated again: %v", err)

		return nil
	}

	runLog.Printf("Drafts by models other than %s are translated again", strings.Join(served, ", "))

	return served
}

// sourceLocale returns the locale to translate from: the given one, or the default.
func (r *Runner) sourceLocale(ctx context.Context, localeID string) (*models.Locale, error) {
	db := r.db.WithContext(ctx)

	if localeID == "" {
		locale, err := common.DefaultLocale(db)
		if err != nil {
			return nil, err
		}

		if locale == nil {
			return nil, stderrors.New("there is no default language to translate from")
		}

		return locale, nil
	}

	var locale models.Locale
	if err := db.First(&locale, "id = ?", localeID).Error; err != nil {
		return nil, stderrors.New("the language to translate from no longer exists")
	}

	return &locale, nil
}
