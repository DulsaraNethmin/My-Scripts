# jobzkraper

Jobzkraper scrapes job postings — LinkedIn, Greenhouse, Lever and Ashby
boards, and Indeed — into one SQLite database on a schedule, scores them
against a profile derived from *your* CV, and serves them back as a queue you
review over its CLI, with a morning digest. The image is
`ghcr.io/dulsaranethmin/jobzkraper`, and it carries everything: both of its
Python runtimes, all five source adapters, the scheduler.

This is the catalog's first **worker** stack: nothing listens on any port.
You talk to it through its own CLI, and everything after `--` goes to that
CLI:

```
spin up jobzkraper                       # first boot waits for setup, and says so
spin cli jobzkraper -- init              # the setup interview: searches, skills, titles
spin restart jobzkraper                  # the scheduler takes it from there

spin cli jobzkraper -- queue             # what came in, best first
spin cli jobzkraper -- mark 17 reviewed  # move a posting along
spin cli jobzkraper -- stats             # rows per source, per status
spin cli jobzkraper -- doctor            # every misconfiguration, named
spin cli jobzkraper -- scrape            # one run now, without waiting for 06:00
```

## First-time setup

The scraper is useless without knowing what to look for, so `up` deliberately
does not invent a config: until one exists the container idles, prints these
same instructions to `spin logs jobzkraper`, and reports healthy — waiting
for you *is* its job at that point.

1. `spin cli jobzkraper -- init` — an interview: keywords, location, sources,
   ATS boards to watch, your skills and target titles. It writes `config.yaml`
   (the searches) and `candidate.yaml` (you) into the config volume. No
   terminal handy? `-- init --defaults` writes a generic example to edit later
   (re-run with `-- init --force` to redo the interview).
2. Optional, for the CV-tailoring command:
   `docker cp your-cv.docx spinup-jobzkraper-jobzkraper-1:/config/cv.docx`
   (`.docx`, `.md` or `.txt` — not PDF).
3. `spin restart jobzkraper` — from here it scrapes at 06:00 and 18:00 and
   writes a digest at 07:00, local wall clock, with deliberate jitter so runs
   do not fire at the exact same second every day.

## Environment

Set these with `spin env jobzkraper --edit`.

| Variable | Default | What it does |
|---|---|---|
| `TZ` | `UTC` | Whose clock 06:00 means. Set it, or the schedule is UTC. |
| `JOBSCRAPER_SCRAPE_AT` | `06:00,18:00` | Scrape times, `HH:MM`, comma-separated |
| `JOBSCRAPER_DIGEST_AT` | `07:00` | Digest time |
| `NORDVPN_SOCKS_USER` / `_PASS` | — | SOCKS5 credentials, only if your config proxies a source |
| `JOBSCRAPER_HOME_IP` | — | Sources told to refuse a VPN exit verify they leave from here |
| `JOBSCRAPER_DIGEST_WEBHOOK` | — | POST the digest here; empty keeps it on disk only |

## Where your data lives

Two named volumes, and the difference matters:

- `spinup-jobzkraper_jobzkraper-config` — your searches, your candidate
  profile, your CV. Deliberately a volume rather than files in the stack
  directory: `spin reset jobzkraper` deletes the stack directory, and your
  configuration survives it.
- `spinup-jobzkraper_jobzkraper-data` — the database, response cache, digest
  and application artifacts.

`spin destroy jobzkraper` deletes **both**, config included. It asks first.

Anything downstream that consumes the queue goes through
`docker exec` (or `spin cli jobzkraper -- queue --json`), never by mounting
the data volume into another container: a second process needs write
permission on the database even to read it (WAL), and a consumer built
against a different version would silently read a different contract.

## Where it runs matters

The LinkedIn source works best from a **residential IP** — a home connection.
Datacenter ranges are pre-flagged, so on a VPS or in the cloud expect LinkedIn
to deny more and the other sources (which are official APIs and JobSpy) to
carry the load. That is a property of the addresses, not a knob in this stack.

## Notes

- First boot in an unconfigured state is *healthy by design* — the container
  is waiting for `init` and says so in its logs. After setup, health means
  `jobscraper doctor --quiet` passes.
- The interview needs a terminal. Piped or scripted, `spin cli` drops the
  TTY, so use `-- init --defaults` there.
- The image is pinned to a release tag. New Jobzkraper releases arrive as
  spinup updates, not by `:latest` drifting under you overnight.
