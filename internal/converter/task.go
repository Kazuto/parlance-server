package converter

import (
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
	"github.com/kazuto/parlance-server/internal/models"
)

// Model values and their proto enums
var (
	taskTypes = map[string]pb.TaskType{
		models.TaskTypeAITranslate: pb.TaskType_TASK_TYPE_AI_TRANSLATE,
		models.TaskTypeGitExport:   pb.TaskType_TASK_TYPE_GIT_EXPORT,
	}
	taskEvents = map[string]pb.TaskEvent{
		models.EventTranslationsApproved: pb.TaskEvent_TASK_EVENT_TRANSLATIONS_APPROVED,
	}
	taskTriggers = map[string]pb.TaskTrigger{
		models.TriggerManual:   pb.TaskTrigger_TASK_TRIGGER_MANUAL,
		models.TriggerSchedule: pb.TaskTrigger_TASK_TRIGGER_SCHEDULE,
		models.TriggerEvent:    pb.TaskTrigger_TASK_TRIGGER_EVENT,
	}
	taskRunStatuses = map[string]pb.TaskRunStatus{
		models.RunQueued:    pb.TaskRunStatus_TASK_RUN_STATUS_QUEUED,
		models.RunRunning:   pb.TaskRunStatus_TASK_RUN_STATUS_RUNNING,
		models.RunSucceeded: pb.TaskRunStatus_TASK_RUN_STATUS_SUCCEEDED,
		models.RunFailed:    pb.TaskRunStatus_TASK_RUN_STATUS_FAILED,
		models.RunCanceled:  pb.TaskRunStatus_TASK_RUN_STATUS_CANCELED,
	}
	exportFormats = map[string]pb.ExportFormat{
		"json": pb.ExportFormat_EXPORT_FORMAT_JSON,
		"php":  pb.ExportFormat_EXPORT_FORMAT_PHP,
		"po":   pb.ExportFormat_EXPORT_FORMAT_PO,
	}
)

// lookup returns the key that maps to value, or "" for none.
func lookup[V comparable](values map[string]V, value V) string {
	for key, candidate := range values {
		if candidate == value {
			return key
		}
	}

	return ""
}

// TaskTypeFromProto returns the model's task type, or "" for none.
func TaskTypeFromProto(value pb.TaskType) string { return lookup(taskTypes, value) }

// TaskEventFromProto returns the model's event name, or "" for none.
func TaskEventFromProto(value pb.TaskEvent) string { return lookup(taskEvents, value) }

// ExportFormatFromProto returns the format's file extension, or "" for none.
func ExportFormatFromProto(value pb.ExportFormat) string { return lookup(exportFormats, value) }

// TaskToProto converts a Task with its newest run, if any. The token stays out:
// only whether one is set is sent.
func TaskToProto(task *models.Task, lastRun *models.TaskRun) *pb.Task {
	result := &pb.Task{
		Id:                task.ID,
		Name:              task.Name,
		Type:              taskTypes[task.Type],
		Enabled:           task.Enabled,
		Schedule:          task.Schedule,
		NextRunAt:         optionalTime(task.NextRunAt),
		EventDelayMinutes: int32(task.EventDelayMinutes),
		HasToken:          task.Secret != "",
		CreatedAt:         task.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         task.UpdatedAt.Format(time.RFC3339),
	}

	for _, event := range task.Events {
		if value, ok := taskEvents[event]; ok {
			result.Events = append(result.Events, value)
		}
	}

	switch task.Type {
	case models.TaskTypeAITranslate:
		if config, err := task.AITranslate(); err == nil {
			result.AiTranslate = &pb.AiTranslateConfig{
				SourceLocaleId:   config.SourceLocaleID,
				TargetLocaleIds:  config.TargetLocaleIDs,
				ScopeIds:         config.ScopeIDs,
				ApplyTerminology: config.ApplyTerminology,
				MaxPairs:         int32(config.MaxPairs),
			}
		}
	case models.TaskTypeGitExport:
		if config, err := task.GitExport(); err == nil {
			result.GitExport = GitExportConfigToProto(config)
		}
	}

	if lastRun != nil {
		result.LastRun = TaskRunToProto(lastRun, false)
	}

	return result
}

// GitExportConfigToProto converts a git export's settings.
func GitExportConfigToProto(config models.GitExportConfig) *pb.GitExportConfig {
	result := &pb.GitExportConfig{
		ApiUrl:        config.APIURL,
		Repository:    config.Repository,
		BaseBranch:    config.BaseBranch,
		Branch:        config.Branch,
		LocaleIds:     config.LocaleIDs,
		ScopeIds:      config.ScopeIDs,
		IncludeDrafts: config.IncludeDrafts,
	}

	for _, file := range config.Files {
		result.Files = append(result.Files, &pb.GitExportFile{Format: exportFormats[file.Format], Path: file.Path})
	}

	return result
}

// TaskRunToProto converts a TaskRun, with its log only when asked, as logs are long.
func TaskRunToProto(run *models.TaskRun, withLog bool) *pb.TaskRun {
	summary := run.RunSummary()

	result := &pb.TaskRun{
		Id:              run.ID,
		TaskId:          run.TaskID,
		Trigger:         taskTriggers[run.Trigger],
		TriggeredBy:     run.TriggeredBy,
		TriggeredByUser: UserToProto(run.TriggeredByUser),
		Status:          taskRunStatuses[run.Status],
		RunAfter:        run.RunAfter.Format(time.RFC3339),
		CancelRequested: run.CancelRequested,
		StartedAt:       optionalTime(run.StartedAt),
		FinishedAt:      optionalTime(run.FinishedAt),
		Error:           run.Error,
		CreatedAt:       run.CreatedAt.Format(time.RFC3339),
		Summary: &pb.TaskRunSummary{
			Translated:     int32(summary.Translated),
			Failed:         int32(summary.Failed),
			Remaining:      int32(summary.Remaining),
			PullRequestUrl: summary.PullRequestURL,
			ChangedFiles:   summary.ChangedFiles,
			Outcome:        summary.Outcome,
		},
	}

	if withLog {
		result.Log = run.Log
	}

	return result
}

// optionalTime formats an optional time as RFC 3339, or returns nil without one.
func optionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}

	formatted := value.UTC().Format(time.RFC3339)

	return &formatted
}
