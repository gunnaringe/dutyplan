# Security

## Reporting a problem

Please report security issues privately, through
[GitHub's private vulnerability reporting](https://github.com/gunnaringe/dutyplan/security/advisories/new),
not as a public issue.

## What to know when you run it

- **There is no login.** Anyone who can reach the server can read and change everything in it: every organisation, plan,
  person and answer. Run it on a trusted network, or behind a proxy that handles login. See *No login: where to run it* in
  [README.md](README.md).
- It listens on `127.0.0.1:8080` by default. `--addr 0.0.0.0:8080` (and the container image, which does that) makes it
  reachable from other machines.
- The page sends a strict Content-Security-Policy and no third-party resources. That keeps other pages out; it is not
  authentication.
- All data is one SQLite file. Protect it and its backups like the data in it: names, and when people can come.
