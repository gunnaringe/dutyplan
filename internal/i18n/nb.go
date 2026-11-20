package i18n

var nb = map[string]string{
	// Why a place is empty (explain)
	"explain.none":              "Ingen kan ta denne",
	"explain.some":              "%d kunne stilt, men trengs andre steder eller kan ikke kombineres med noe de allerede har",
	"explain.selection":         "utvalget er %s",
	"explain.selection_empty":   "utvalget er tomt",
	"explain.selection_outside": " (%d andre står utenfor)",
	"explain.event_never":       "passer ikke: %s",
	"explain.task_never":        "har sagt «ikke» til oppgaven: %s",
	"explain.not_proposable":    "kan ikke foreslås: %s",
	"explain.busy":              "opptatt med noe annet på hendelsen: %s",
	"explain.not_in_task":       "%d har ikke valgt oppgaven",
	"explain.silent":            "Planleggeren sa ingenting om denne plassen.",

	// Warnings about manual cells (from the solver, with names)
	"warn.manual.not_in_task":       "%s er satt på %s %s for hånd, selv om hen ikke har valgt oppgaven.",
	"warn.manual.task_never":        "%s er satt på %s %s for hånd, selv om hen har sagt «ikke» til oppgaven.",
	"warn.manual.event_never":       "%s er satt på %s %s for hånd, selv om det ikke passer.",
	"warn.manual.outside_selection": "%s er satt på %s %s for hånd, selv om hen står utenfor utvalget.",
	"warn.manual.busy":              "%s er satt på %s %s for hånd, selv om hen er opptatt med %s.",
	"warn.manual.other":             "%s er satt på %s %s for hånd, selv om «%s».",

	// Compromises (a «poor» answer was used)
	"compromise":            "%s fikk %s %s, selv om det er «dårlig» for %s.",
	"compromise.what.event": "hendelsen",
	"compromise.what.task":  "oppgaven",
	"compromise.what.both":  "hendelsen og oppgaven",

	// Notes made when the run starts: things the planner was not told about
}
