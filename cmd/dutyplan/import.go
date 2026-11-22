package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// importCmd reads an import (a JSON file, or - for standard input) and puts it into an organisation. It prints what it made
// as one line of JSON; with --dry-run nothing is stored. Exit 0 when done, 1 when refused, 2 for wrong arguments.
func importCmd(args []string) int {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	dbPath := fs.String("db", "dutyplan.db", "SQLite database file")
	org := fs.String("org", "", "the organisation's address (slug), e.g. my-choir")
	dry := fs.Bool("dry-run", false, "check everything, store nothing")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: dutyplan import --org <slug> [--db file] [--dry-run] <file.json | ->")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil || *org == "" || fs.NArg() != 1 {
		if err == nil {
			fs.Usage()
		}
		return 2
	}
	var r io.Reader = os.Stdin
	if name := fs.Arg(0); name != "-" {
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "import:", err)
			return 1
		}
		defer f.Close()
		r = f
	}
	var in store.Import
	dec := json.NewDecoder(io.LimitReader(r, 4<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		fmt.Fprintln(os.Stderr, "import: not a valid import:", err)
		return 1
	}
	st, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "import:", err)
		return 1
	}
	defer st.Close()
	res, err := st.Import(context.Background(), *org, in, *dry)
	if err != nil {
		fmt.Fprintln(os.Stderr, "import: refused, nothing stored:", err)
		return 1
	}
	out, _ := json.Marshal(map[string]any{"dry_run": *dry, "result": res})
	fmt.Println(string(out))
	return 0
}
