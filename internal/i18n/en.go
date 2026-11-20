package i18n

var en = map[string]string{
	// Why a place is empty (explain)
	"explain.none":              "Nobody can take this",
	"explain.some":              "%d could have taken it, but are needed elsewhere or cannot be combined with something they already have",
	"explain.selection":         "the selection is %s",
	"explain.selection_empty":   "the selection is empty",
	"explain.selection_outside": " (%d others are outside it)",
	"explain.event_never":       "cannot attend: %s",
	"explain.task_never":        "said «never» to the task: %s",
	"explain.not_proposable":    "cannot be proposed: %s",
	"explain.busy":              "busy with something else at the event: %s",
	"explain.not_in_task":       "%d have not chosen the task",
	"explain.silent":            "The planner said nothing about this place.",

	// Warnings about manual cells (from the solver, with names)
	"warn.manual.not_in_task":       "%s was set on %s %s by hand, even though they have not chosen the task.",
	"warn.manual.task_never":        "%s was set on %s %s by hand, even though they said «never» to the task.",
	"warn.manual.event_never":       "%s was set on %s %s by hand, even though it does not suit them.",
	"warn.manual.outside_selection": "%s was set on %s %s by hand, even though they are outside the selection.",
	"warn.manual.busy":              "%s was set on %s %s by hand, even though they are busy with %s.",
	"warn.manual.other":             "%s was set on %s %s by hand, even though «%s».",

	// Compromises (a «poor» answer was used)
	"compromise":            "%s got %s %s, even though it is «poor» for %s.",
	"compromise.what.event": "the event",
	"compromise.what.task":  "the task",
	"compromise.what.both":  "the event and the task",

	// Notes made when the run starts: things the planner was not told about
}
