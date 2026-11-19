# UI kit

The look of this panel and the widgets stock $mol lacks come from
[airkit](https://github.com/alexesDev/airkit), a separate mam package:
the «Air» theme (`$airkit_air`), status pills, stat tiles, column titles once
above a list, a confirm page, a secret shown once, a stacked chart and more.
Its README lists every widget, its properties and states, the patterns
(dashboard, list → item, confirm page, create-once secret) and the tokens; its
demo shows them: <https://alexesdev.github.io/airkit/>.

## How this project wires it

| Where | What |
|---|---|
| the mam workspace's `.meta.tree` | `pack airkit git \https://github.com/alexesDev/airkit.git` — mam clones the package on the first build that names an `$airkit_…` module. `make ui-link` adds the line if it is missing. |
| `Dockerfile`, the `ui` stage | the same pack line; mam clones airkit's `main` during the build. `MAM_PULL_DISABLED=1` only keeps the pinned mam workspace from moving. |
| `assets/ui/app/app.view.tree` | `<= Air $airkit_air` next to `$mol_theme_auto` in the root's plugins |
| `assets/ui/users/catalog`, `assets/ui/auditlog` | `$airkit_columns` above the list (sticky in the audit log), `$airkit_pill` for a user's status; column widths in each page's `view.css` as `--airkit_columns` |

## airkit follows main

Every build takes airkit's current `main`, here and in the image. So a
breaking change in airkit reaches every project on its next build: airkit
changes go through pull requests and keep their API — a property, a token or
a module name that consumers use is not renamed or removed in place. A change
to a widget goes to airkit as its own PR; nothing in this repository pins it.
