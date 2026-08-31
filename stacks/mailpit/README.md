# mailpit

[Mailpit](https://mailpit.axllent.org/) is an SMTP server that accepts every
message your app sends and shows it in a web inbox instead of delivering it.
Point your development mailer at it and nothing can ever reach a real person.

```
spinup up mailpit             # SMTP on 1025, inbox on http://localhost:8085
spinup open mailpit           # the inbox, in your browser
spinup url mailpit            # smtp://localhost:1025
```

## Ports

| Service | Host port | Container | Env var             |
|---------|-----------|-----------|---------------------|
| SMTP    | `1025`    | 1025      | `MAILPIT_SMTP_PORT` |
| Inbox   | `8085`    | 8025      | `MAILPIT_UI_PORT`   |

## Credentials

None. The inbox has no login, and the SMTP server accepts any username and
password your app cares to send — including over an unencrypted connection,
which is what `MAILPIT_ACCEPT_ANY_AUTH` and `MAILPIT_ALLOW_INSECURE_AUTH` are
for. Many frameworks refuse to send without an auth mechanism, and a mail
catcher that rejects them is a mail catcher you spend an afternoon on.

## Pointing an app at it

```
SMTP_HOST=localhost
SMTP_PORT=1025
SMTP_USER=anything
SMTP_PASS=anything
SMTP_ENCRYPTION=none
```

Django, Rails, Laravel, Nodemailer and every SMTP library work with those; the
username and password are ignored. From another container, the host is
`host.docker.internal` rather than `localhost`.

## Storage

Messages are kept in `mailpit-data` (`MP_DATABASE=/data/mailpit.db`), so
`spinup down mailpit` keeps the inbox and `spinup destroy mailpit` empties it.
Without that database Mailpit holds everything in memory and forgets it on
every restart, which is the default the image ships with.

`MAILPIT_MAX_MESSAGES` caps the inbox at 5000 messages, dropping the oldest
first. Set it to `0` to keep everything.

## Notes

- The web inbox is part of the same container as the SMTP server, so this stack
  has no `gui` profile: there is nothing separate to start.
- Mailpit can also speak POP3 and can relay to a real server. Neither is on
  here; see the Mailpit docs if you need them.
