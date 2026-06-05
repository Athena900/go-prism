package evidence

import (
	"fmt"
	"sort"
	"strings"
)

const (
	maxSummaryFindings = 5
	maxSummaryActions  = 3
)

// NewMaintainerSummary creates a deterministic human-readable report summary.
func NewMaintainerSummary(items []Item, decision Status) *MaintainerSummary {
	if len(items) == 0 {
		return nil
	}

	findings := summaryFindings(items)
	actions := summaryActions(items)
	summary := &MaintainerSummary{
		Headline:    summaryHeadline(decision),
		KeyFindings: findings,
		NextActions: actions,
	}
	summary.EvidenceIDs = collectSummaryEvidenceIDs(summary)
	return summary
}

func summaryHeadline(decision Status) string {
	switch decision {
	case StatusBlock:
		return "Release review is blocked by deterministic evidence."
	case StatusWarn:
		return "Maintainer review is needed before release."
	case StatusUnknown:
		return "Some checks could not complete and need review."
	default:
		return "No blockers or warnings were found in deterministic evidence."
	}
}

func summaryFindings(items []Item) []SummaryFinding {
	selected := selectSummaryItems(items)
	findings := make([]SummaryFinding, 0, len(selected))
	for _, item := range selected {
		findings = append(findings, SummaryFinding{
			Text:        itemSummaryText(item),
			EvidenceIDs: []string{item.ID},
		})
	}
	return findings
}

func selectSummaryItems(items []Item) []Item {
	riskItems := orderedItemsByStatus(items, []Status{StatusBlock, StatusWarn, StatusUnknown}, false)
	if len(riskItems) > 0 {
		return firstN(riskItems, maxSummaryFindings)
	}

	nonMeta := orderedItemsByStatus(items, []Status{StatusPass, StatusInfo}, true)
	if len(nonMeta) > 0 {
		return firstN(nonMeta, maxSummaryFindings)
	}

	meta := orderedItemsByStatus(items, []Status{StatusPass, StatusInfo}, false)
	return firstN(meta, maxSummaryFindings)
}

func orderedItemsByStatus(items []Item, statuses []Status, skipMeta bool) []Item {
	out := make([]Item, 0, len(items))
	for _, status := range statuses {
		group := make([]Item, 0)
		for _, item := range items {
			if item.Status != status || strings.TrimSpace(item.ID) == "" {
				continue
			}
			if skipMeta && item.Category == CategoryMeta {
				continue
			}
			group = append(group, item)
		}
		sort.SliceStable(group, func(i, j int) bool {
			return group[i].ID < group[j].ID
		})
		out = append(out, group...)
	}
	return out
}

func firstN(items []Item, max int) []Item {
	if len(items) <= max {
		return items
	}
	return items[:max]
}

func itemSummaryText(item Item) string {
	title := strings.TrimSpace(item.Title)
	summary := strings.TrimSpace(item.Summary)
	switch {
	case title == "":
		return summary
	case summary == "":
		return title
	default:
		return fmt.Sprintf("%s: %s", title, summary)
	}
}

func summaryActions(items []Item) []SummaryFinding {
	selected := orderedItemsByStatus(items, []Status{StatusBlock, StatusWarn, StatusUnknown}, false)
	if len(selected) == 0 {
		return nil
	}

	actions := make([]SummaryFinding, 0, maxSummaryActions)
	for _, item := range selected {
		action := strings.TrimSpace(item.Recommendation)
		if action == "" {
			action = defaultSummaryAction(item.Status)
		}
		actions = append(actions, SummaryFinding{
			Text:        action,
			EvidenceIDs: []string{item.ID},
		})
		if len(actions) == maxSummaryActions {
			break
		}
	}
	return actions
}

func defaultSummaryAction(status Status) string {
	switch status {
	case StatusBlock:
		return "Resolve this blocker before release."
	case StatusWarn:
		return "Review this warning before release."
	case StatusUnknown:
		return "Re-run or inspect this check before relying on the report."
	default:
		return "Review this evidence item before release."
	}
}

func collectSummaryEvidenceIDs(summary *MaintainerSummary) []string {
	seen := map[string]bool{}
	ids := make([]string, 0)
	for _, finding := range summary.KeyFindings {
		for _, id := range finding.EvidenceIDs {
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, finding := range summary.NextActions {
		for _, id := range finding.EvidenceIDs {
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
