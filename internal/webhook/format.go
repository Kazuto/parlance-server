package webhook

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/kazuto/parlance-server/internal/models"
)

const (
	// maxListed caps the translations or entries a chat message lists.
	maxListed = 10
	// maxDigestKeys caps the keys a digest lists per kind of event, and
	// maxDigestNames the people and tasks.
	maxDigestKeys  = 5
	maxDigestNames = 3
	// maxListedText is how much of a translation a chat message shows.
	maxListedText = 80
	// discordMaxContent is the longest message Discord takes.
	discordMaxContent = 2000
)

// discordMessage is a message as Discord's webhooks take it. Mentions are off, so a
// translation reading "@everyone" pings nobody.
type discordMessage struct {
	Content         string `json:"content"`
	Username        string `json:"username"`
	AllowedMentions struct {
		Parse []string `json:"parse"`
	} `json:"allowed_mentions"`
}

// Body returns the request body that posts a delivery's payload in the format: the
// event as JSON, or a chat message describing it.
func Body(format, payload string) ([]byte, error) {
	if format != models.WebhookFormatSlack && format != models.WebhookFormatDiscord {
		return []byte(payload), nil
	}

	text, err := message(payload)
	if err != nil {
		return nil, err
	}

	if format == models.WebhookFormatSlack {
		return json.Marshal(map[string]string{"text": escapeSlack(text)})
	}

	body := discordMessage{Content: truncate(text, discordMaxContent), Username: "Parlance"}
	body.AllowedMentions.Parse = []string{}

	return json.Marshal(body)
}

// message describes an event in a few lines of chat text.
func message(payload string) (string, error) {
	var event struct {
		Event string          `json:"event"`
		Data  json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return "", fmt.Errorf("read the event: %w", err)
	}

	decode := func(data any) error {
		if err := json.Unmarshal(event.Data, data); err != nil {
			return fmt.Errorf("read the %s event: %w", event.Event, err)
		}

		return nil
	}

	switch event.Event {
	case models.WebhookEventTranslationsApproved, models.WebhookEventTranslationsRejected, models.WebhookEventTranslationsAwaitingApproval:
		var data TranslationsData
		if err := decode(&data); err != nil {
			return "", err
		}

		return translationsMessage(event.Event, data), nil
	case models.WebhookEventEntriesCreated:
		var data EntriesData
		if err := decode(&data); err != nil {
			return "", err
		}

		return entriesMessage(data), nil
	case models.WebhookEventTaskRunSucceeded, models.WebhookEventTaskRunFailed:
		var data TaskRunData
		if err := decode(&data); err != nil {
			return "", err
		}

		return taskRunMessage(data), nil
	case models.WebhookEventPing:
		var data PingData
		if err := decode(&data); err != nil {
			return "", err
		}

		return fmt.Sprintf("This is a test message from the Parlance webhook “%s”.", data.Webhook.Name), nil
	case models.WebhookEventDigest:
		var data DigestData
		if err := decode(&data); err != nil {
			return "", err
		}

		return digestMessage(data), nil
	default:
		return "Parlance: " + event.Event, nil
	}
}

func translationsMessage(event string, data TranslationsData) string {
	var text strings.Builder

	what := countOf(data.Count, "translation", "translations")
	who := translationsBy(data)

	switch {
	case event == models.WebhookEventTranslationsAwaitingApproval && data.Task != nil:
		fmt.Fprintf(&text, "%s drafted %s for approval", who, what)
	case event == models.WebhookEventTranslationsAwaitingApproval && who != "":
		fmt.Fprintf(&text, "%s submitted %s for approval", who, what)
	case event == models.WebhookEventTranslationsAwaitingApproval && data.Count == 1:
		fmt.Fprintf(&text, "%s awaits approval", what)
	case event == models.WebhookEventTranslationsAwaitingApproval:
		fmt.Fprintf(&text, "%s await approval", what)
	case who != "":
		fmt.Fprintf(&text, "%s %s %s", who, reviewVerbs[event], what)
	default:
		fmt.Fprintf(&text, "%s %s", what, reviewVerbs[event])
	}

	if data.Comment != "" {
		fmt.Fprintf(&text, ": “%s”", oneLine(data.Comment))
	}

	for i, translation := range data.Translations {
		if i == maxListed {
			break
		}

		fmt.Fprintf(&text, "\n• %s (%s): %s", code(translation.Key), translation.Locale, truncate(oneLine(translation.Translation), maxListedText))
	}

	writeMore(&text, data.Count, len(data.Translations))

	return text.String()
}

// reviewVerbs say what a review did to translations.
var reviewVerbs = map[string]string{
	models.WebhookEventTranslationsApproved: "approved",
	models.WebhookEventTranslationsRejected: "rejected",
}

// translationsBy names who changed translations: the task, or the user.
func translationsBy(data TranslationsData) string {
	switch {
	case data.Task != nil:
		return fmt.Sprintf("Task “%s”", data.Task.Name)
	case data.User != nil:
		return data.User.Name
	default:
		return ""
	}
}

func entriesMessage(data EntriesData) string {
	var text strings.Builder

	what := countOf(data.Count, "entry", "entries")

	if data.User != nil {
		fmt.Fprintf(&text, "%s added %s", data.User.Name, what)
	} else {
		fmt.Fprintf(&text, "%s added", what)
	}

	for i, entry := range data.Entries {
		if i == maxListed {
			break
		}

		fmt.Fprintf(&text, "\n• %s", code(entry.Key))
	}

	writeMore(&text, data.Count, len(data.Entries))

	return text.String()
}

func taskRunMessage(data TaskRunData) string {
	if data.Run.Status == models.RunFailed {
		return fmt.Sprintf("Task “%s” failed: %s", data.Task.Name, oneLine(data.Run.Error))
	}

	text := fmt.Sprintf("Task “%s” succeeded", data.Task.Name)
	summary := data.Run.Summary

	if summary.Translated > 0 || summary.Failed > 0 {
		text += fmt.Sprintf(": %d translated, %d failed", summary.Translated, summary.Failed)
	}

	if summary.PullRequestURL != "" {
		text += "\nPull request: " + summary.PullRequestURL
	}

	return text
}

// digestGroup sums up the events of one kind in a digest.
type digestGroup struct {
	count int
	names []string
	keys  []string
	// more tells that there were keys beyond those listed.
	more bool
}

func (g *digestGroup) add(count int, name string, keys []string) {
	g.count += count

	if name != "" && !slices.Contains(g.names, name) {
		g.names = append(g.names, name)
	}

	for _, key := range keys {
		if slices.Contains(g.keys, key) {
			continue
		}

		if len(g.keys) == maxDigestKeys {
			g.more = true

			return
		}

		g.keys = append(g.keys, key)
	}

	g.more = g.more || count > len(keys)
}

// digestKinds are the kinds of events a digest sums up, in its order, with what
// their events did.
var digestKinds = []struct {
	event, noun, nouns, did string
}{
	{models.WebhookEventTranslationsAwaitingApproval, "translation", "translations", "submitted for approval"},
	{models.WebhookEventTranslationsApproved, "translation", "translations", "approved"},
	{models.WebhookEventTranslationsRejected, "translation", "translations", "rejected"},
	{models.WebhookEventEntriesCreated, "entry", "entries", "added"},
}

// digestMessage sums up a digest's events: one line per kind of change, and one per
// task run.
func digestMessage(data DigestData) string {
	groups := map[string]*digestGroup{}
	group := func(event string) *digestGroup {
		if groups[event] == nil {
			groups[event] = &digestGroup{}
		}

		return groups[event]
	}

	var runs []string

	for _, raw := range data.Events {
		var event struct {
			Event string          `json:"event"`
			Data  json.RawMessage `json:"data"`
		}

		if err := json.Unmarshal(raw, &event); err != nil {
			continue
		}

		switch event.Event {
		case models.WebhookEventTranslationsApproved, models.WebhookEventTranslationsRejected, models.WebhookEventTranslationsAwaitingApproval:
			var translations TranslationsData
			if json.Unmarshal(event.Data, &translations) != nil {
				continue
			}

			keys := make([]string, len(translations.Translations))
			for i, translation := range translations.Translations {
				keys[i] = translation.Key
			}

			group(event.Event).add(translations.Count, translationsBy(translations), keys)
		case models.WebhookEventEntriesCreated:
			var entries EntriesData
			if json.Unmarshal(event.Data, &entries) != nil {
				continue
			}

			keys := make([]string, len(entries.Entries))
			for i, entry := range entries.Entries {
				keys[i] = entry.Key
			}

			name := ""
			if entries.User != nil {
				name = entries.User.Name
			}

			group(event.Event).add(entries.Count, name, keys)
		case models.WebhookEventTaskRunSucceeded, models.WebhookEventTaskRunFailed:
			var run TaskRunData
			if json.Unmarshal(event.Data, &run) == nil {
				runs = append(runs, strings.ReplaceAll(taskRunMessage(run), "\n", " · "))
			}
		}
	}

	var text strings.Builder

	fmt.Fprintf(&text, "Parlance digest since %s", data.Since.UTC().Format("2 Jan 15:04 UTC"))

	for _, kind := range digestKinds {
		g := groups[kind.event]
		if g == nil {
			continue
		}

		fmt.Fprintf(&text, "\n• %s %s", countOf(g.count, kind.noun, kind.nouns), kind.did)

		if len(g.names) > 0 {
			fmt.Fprintf(&text, " (%s)", names(g.names))
		}

		if len(g.keys) > 0 {
			listed := make([]string, len(g.keys))
			for i, key := range g.keys {
				listed[i] = code(key)
			}

			fmt.Fprintf(&text, ": %s", strings.Join(listed, ", "))

			if g.more {
				text.WriteString(", …")
			}
		}
	}

	for i, run := range runs {
		if i == maxListed {
			fmt.Fprintf(&text, "\n• …and %s", countOf(len(runs)-i, "more task run", "more task runs"))

			break
		}

		text.WriteString("\n• " + run)
	}

	if data.Truncated {
		fmt.Fprintf(&text, "\n…and %s", countOf(data.Count-len(data.Events), "more event", "more events"))
	}

	return text.String()
}

// names lists people and tasks, the first few by name.
func names(list []string) string {
	if len(list) <= maxDigestNames {
		return strings.Join(list, ", ")
	}

	return fmt.Sprintf("%s +%d", strings.Join(list[:maxDigestNames], ", "), len(list)-maxDigestNames)
}

// writeMore says how many items the message left out.
func writeMore(text *strings.Builder, count, listed int) {
	if shown := min(listed, maxListed); count > shown {
		fmt.Fprintf(text, "\n…and %d more", count-shown)
	}
}

func countOf(count int, one, many string) string {
	if count == 1 {
		return "1 " + one
	}

	return fmt.Sprintf("%d %s", count, many)
}

// code shows a key as code; a backtick in it would end the span.
func code(key string) string {
	return "`" + strings.ReplaceAll(key, "`", "'") + "`"
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}

	return string(runes[:limit-1]) + "…"
}

// escapeSlack escapes the characters Slack reads as markup, so a translation can't
// mention a channel or fake a link.
func escapeSlack(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
}
