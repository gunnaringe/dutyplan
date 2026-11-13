# Pictures of the documentation

Made by `mise run app:shots` (`scripts/shots.js`) from the real app: it starts the server on a temporary database, builds the
demo organisation *Maple Hall* through the API (tasks, day types, a group, events with times, answers, a locked place, a
selection, two planner runs, one pinned), and takes the pictures in headless Chromium. Run it again, and commit the result,
when the page changes visibly. It needs the Python packages for the planner (`mise run setup`) and `playwright-core` with a
Chromium, like `mise run app:ui`.

Every name, date and text in them is invented. The people use the placeholder names of `scripts/seed-demo.sh`; keep it that
way (see *Testing and data* in `CLAUDE.md`). `pngquant` is used to make the files smaller when it is installed.
