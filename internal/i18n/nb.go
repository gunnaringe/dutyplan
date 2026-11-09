package i18n

var nb = map[string]string{
	// Why a place is empty (explain)
	"explain.none":                    "Ingen kan ta denne",
	"explain.some":                    "%d kunne stilt, men trengs andre steder eller kan ikke kombineres med noe de allerede har",
	"explain.selection":               "utvalget er %s",
	"explain.selection_empty":         "utvalget er tomt",
	"explain.selection_outside":       " (%d andre står utenfor)",
	"explain.event_never":             "passer ikke: %s",
	"explain.task_never":              "har sagt «ikke» til oppgaven: %s",
	"explain.not_proposable":          "kan ikke foreslås: %s",
	"explain.busy":                    "opptatt med noe annet på hendelsen: %s",
	"explain.not_in_task":             "%d har ikke valgt oppgaven",
	"explain.group.not_proposable":    "%s kan ikke foreslås",
	"explain.group.not_proposable_by": "%s kan ikke foreslås fordi %s ikke kan foreslås",
	"explain.group.too_many_places":   "%s trenger flere plasser enn det er igjen",
	"explain.group.outside_selection": "%s står utenfor utvalget",
	"explain.group.task_never":        "%s: %s har sagt «ikke» til oppgaven",
	"explain.group.event_never":       "%s: %s passer ikke",
	"explain.group.busy":              "%s: %s er opptatt med noe annet",
	"explain.silent":                  "Planleggeren sa ingenting om denne plassen.",

	// Warnings about manual cells (from the solver, with names)
	"warn.manual.not_in_task":       "%s er satt på %s %s for hånd, selv om hen ikke har valgt oppgaven.",
	"warn.manual.task_never":        "%s er satt på %s %s for hånd, selv om hen har sagt «ikke» til oppgaven.",
	"warn.manual.event_never":       "%s er satt på %s %s for hånd, selv om det ikke passer.",
	"warn.manual.outside_selection": "%s er satt på %s %s for hånd, selv om hen står utenfor utvalget.",
	"warn.manual.busy":              "%s er satt på %s %s for hånd, selv om hen er opptatt med %s.",
	"warn.manual.other":             "%s er satt på %s %s for hånd, selv om «%s».",
	"warn.manual.group_member":      "%s er satt på %s %s for hånd, men %s har sagt at det ikke passer.",
	"warn.manual.locked_with_group": "%s er satt på %s %s for hånd samtidig som %s står på %s; de to kan ikke kombineres.",

	// Compromises (a «poor» answer was used)
	"compromise":            "%s fikk %s %s, selv om det er «dårlig» for %s.",
	"compromise.what.event": "hendelsen",
	"compromise.what.task":  "oppgaven",
	"compromise.what.both":  "hendelsen og oppgaven",
	"compromise.member_of":  " (som medlem av %s)",

	// Notes made when the run starts: things the planner was not told about
	"note.inactive_person_cell":  "Ruten for %s på %s er ikke med fordi personen ikke er aktiv.",
	"note.group_cell_no_members": "Ruten for gruppen «%s» på %s er ikke med fordi gruppen ikke har aktive medlemmer.",
	"note.group_no_members":      "Gruppen «%s» har ingen aktive medlemmer og er ikke med i planleggingen.",
	"note.empty_selection":       "Utvalget for %s har ingen aktive personer eller grupper, så ingen kan velges.",
}
