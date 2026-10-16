package activity

import (
	"time"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// change is one row from a history table.
type change struct {
	ID        string
	Kind      pb.ActivityKind
	Action    string // created, updated, deleted
	UserID    string
	LocaleID  string
	SubjectID string // entry or terminology id
	ChangedAt time.Time
}

// group is a run of related changes shown as one activity item.
type group struct {
	Newest   change
	Oldest   time.Time
	Created  int
	Updated  int
	Deleted  int
	Subjects map[string]bool
	Locales  map[string]bool
}

// maxGap is the longest pause between two changes that still counts as the same burst of work.
const maxGap = 5 * time.Minute

// groupChanges folds changes (newest first) into at most limit groups. Consecutive changes of
// the same kind by the same user join a group while they are at most maxGap apart, so an
// import or "save all" becomes one item instead of hundreds.
func groupChanges(changes []change, limit int) []*group {
	var groups []*group
	for _, c := range changes {
		if last := lastGroup(groups); last != nil &&
			last.Newest.Kind == c.Kind &&
			last.Newest.UserID == c.UserID &&
			last.Oldest.Sub(c.ChangedAt) <= maxGap {
			last.add(c)
			continue
		}
		if len(groups) == limit {
			break
		}
		g := &group{Newest: c, Oldest: c.ChangedAt, Subjects: map[string]bool{}, Locales: map[string]bool{}}
		g.add(c)
		groups = append(groups, g)
	}
	return groups
}

func lastGroup(groups []*group) *group {
	if len(groups) == 0 {
		return nil
	}
	return groups[len(groups)-1]
}

func (g *group) add(c change) {
	if c.ChangedAt.Before(g.Oldest) {
		g.Oldest = c.ChangedAt
	}
	g.Subjects[c.SubjectID] = true
	g.Locales[c.LocaleID] = true
	switch c.Action {
	case "created":
		g.Created++
	case "deleted":
		g.Deleted++
	default:
		g.Updated++
	}
}
