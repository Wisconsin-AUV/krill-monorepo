# Krill

Labeling tool for building and validating WAUV's RoboSub YOLO datasets. This app aims to help humans be as efficient as possible.  See the [design doc](docs/design-doc.md).

## Quick start

```sh
cp .env.example .env
just setup
just dev      # Postgres :5432, MinIO :9000 (console :9001)
just api      # :8080
just web      # :5173
```

Full stack on the GPU box. Set the passwords in `.env` first:

```sh
just up
```

Build the worker for CUDA Nvidia with `just build-worker-cuda`.

The GPU worker authenticates with `KRILL_WORKER_TOKEN`, which must be set for both the API and the worker, or with a worker token created in the admin panel. It runs SAM 2.1 by default. To use SAM 3, request access to [facebook/sam3](https://huggingface.co/facebook/sam3), set `HF_TOKEN` on the worker, and pick SAM 3 under Settings in the admin panel.

The API image bundles the web app, so the full stack is served from `:8080`. Browsers load frames straight from MinIO on `:9000`, so `KRILL_S3_PUBLIC_ENDPOINT` must be an address they can reach. When the app is served over HTTPS, MinIO needs its own HTTPS hostname too, for example a second Cloudflare Tunnel route to `:9000`.

## Admin panel

Admins open the admin panel from the shield in the top bar. It shows the running version and what changed in newer releases, checks the database, storage, GPU worker, email, and Slack, and charts CPU, memory, and GPU use for the API and each worker.

Its Database page backs the database up to MinIO under `backups/`, by hand or daily at the hour `KRILL_BACKUP_SCHEDULE` sets in the API's time zone (UTC in Docker), keeping the newest `KRILL_BACKUP_KEEP` scheduled ones. Backups need `pg_dump` at least as new as the database, which the API image has. To restore one, download it and run `pg_restore --clean --if-exists -d "$KRILL_DATABASE_URL" krill-....dump` with the API stopped.

Its Settings page edits every `KRILL_` variable except the ones the API needs to start or to be reached: the database, storage, listen address, public URL, and web directory. Saved settings apply without a restart. A variable set in the environment overrides the saved value and locks it in the panel, and an empty variable counts as unset.

The SAM model setting applies to every worker between tasks. A worker started with its own `KRILL_SAM_MODEL` keeps that model, and the Compute page shows it as pinned.

To rotate a worker token, create a new one under Worker tokens, set it as the worker's `KRILL_WORKER_TOKEN` and restart the worker, then revoke the old one once the Compute page shows the worker on the new token. Revoked tokens stop working on the next request. The API's own `KRILL_WORKER_TOKEN` stays valid until it is cleared.

Deleted videos and clips wait in the panel's Trash until they are restored or purged. `KRILL_TRASH_RETENTION_DAYS` (30 by default) purges them after that many days, and 0 keeps them until someone empties the trash.

## Accounts

The first account to register becomes an admin. Later sign-ups are labelers, and admins promote them under Users in the admin panel. Password sign-up needs an `@wisc.edu` email, which `KRILL_SIGNUP_EMAIL_DOMAIN` changes. Set `KRILL_ALLOW_SIGNUP=false` to turn off open sign-up.

`KRILL_EMAIL_VERIFICATION` sets how sign-up checks emails: `off` accepts any, `domain` (the default) needs the sign-up domain, and `verify` also has people enter a code emailed to them. Codes are sent with [Resend](https://resend.com), so `verify` needs `KRILL_RESEND_API_KEY` and `KRILL_EMAIL_FROM` at a domain verified in Resend. A key also turns on password resets and sign-in codes. On localhost without a key, emails are written to the API log instead. Edit the email templates in `emails/`, preview them with `just emails`, and rebuild them with `just gen`.

Admins can also let people in from Invites in the admin panel, with an invite link or by pre-approving their email. Either works while sign-up is closed and outside the sign-up domain, and gives the account the role the admin picked. With `verify`, the new account still confirms its email with a code. A pre-approved email trusts whoever signs up with it first, so without `verify` prefer invite links, or Slack, which only matches emails Slack has verified. A pre-approval is good for one account, and an invite for as many as its uses.

Roles and what each can do are defined in [`api/internal/auth/access.go`](api/internal/auth/access.go).

Create a Slack app with the redirect URL `$KRILL_PUBLIC_URL/auth/slack/callback` and the `openid`, `profile`, and `email` scopes, then set `KRILL_SLACK_CLIENT_ID`, `KRILL_SLACK_CLIENT_SECRET`, and `KRILL_SLACK_TEAM_ID`. Slack requires an HTTPS redirect URL.

## Labeling workflow

1. **Labels**: create label types, or add the starter set. Attributes such as size or color are set once per track.
2. **Videos**: upload footage. Frames are extracted and split into clips of about 15 seconds.
3. **Clips**: pick a type with 1 to 9 and drag to draw. Step with J and K, press C to copy boxes from the previous frame, and press Space when every object in the frame has a box (E if there are none). Shift+click an object to track it through the clip with SAM, and press R to retrack a selected box from the current frame. Tracked boxes are proposals until Space accepts them. Press ? for all shortcuts.
4. **Exports**: preview and download a YOLO dataset split by video.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md).


## License

Copyright © 2026 Wisconsin Autonomous Underwater Vehicles, Camden Rush, and contributors.

Code released under the [MIT License](./LICENSE.md).
