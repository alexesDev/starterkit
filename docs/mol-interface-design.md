# Designing an interface on $mol

What a screen, a menu and an action should look like in an app built on this
kit, and why. [frontend-mol.md](frontend-mol.md) is the build manual: files,
codegen, the mam workspace. This page is about the person in front of the page.

Every statement carries its standing:

| Standing | Meaning |
|---|---|
| `from the framework's source` | Read in the mam workspace, file named. |
| `author's statement` | Dmitry Karlovsky (nin-jin) wrote it, source named. |
| `house rule` | Decided in the project this kit was cut from. Follow it. |
| `observed in this kit` | A screen in `assets/ui` does it. Not a rule by itself. |
| `inferred` | A conclusion from the above. Nobody decided it. |

Nothing in this guide was checked in a browser while writing it. Where a claim
is about what a person sees, it is derived from CSS and code, and says so.

The kit ships two sections, Users and Audit log. It has no create page, no
update page, no confirmation page, no counter and no dashboard. Where a rule
needs one of those, the rule describes the pattern and says the kit does not
have it yet.

## A worked example

A page where a person decides on waiting items: letters an assistant drafted
that go out only after a yes. Nine of them wait. The person opens the page and
cannot find how to decide.

| What the person meets | The cause in the design |
|---|---|
| The page opens on a section that is empty, while the nine items wait in another; the menu shows no counts | The default spread is the first section of the menu, and menu entries are bare titles |
| The empty section shows nothing at all | The list declares no empty text |
| They find the item in a journal of everything, which shows it with no action and no hint where to act | A read-only page for an item that waits |
| Rows in the waiting list show a state and a time, nothing else | Nine rows read the same |
| On the item page the approving button is in the title bar and the rejecting one at the very bottom, under the whole text, in a form with a reason field | One of two opposite actions lives at the end of the body |
| A second section with the same purpose has both buttons on top and no reason at all | One purpose, two page designs |
| The text renders as one line on one page and keeps its line breaks on another | A bare view on one, a text component on the other |
| The notification that led here said "Send / Do not send"; the page says "Release / Reject" | Two names for one action |

Each row is a rule below. The remark that started this guide was that $mol
gives a peculiar experience through its catalog, and that the way to design
for it should be written down from what had already been built.

## The model

All of this section is `from the framework's source` unless marked. Paths are
relative to the mam workspace (`MAM_WORKSPACE`, see
[frontend-mol.md](frontend-mol.md#building)).

### A book of pages

The app is a `$mol_book2`: a horizontal strip of pages, left to right
(`mol/book2/book2.view.css`: `display: flex; flex-flow: row nowrap`). The book
is itself a horizontal scroller with `scroll-snap-type: x mandatory`. Each
page keeps its own width (`flex-shrink: 0`) and is never wider than the screen
(`max-width: 100%`). Karlovsky's own definition: «буклет - это горизонтальный
список из вертикальных списков», a booklet is a horizontal list of vertical
lists (`author's statement`,
[habr 537388](https://habr.com/ru/articles/537388/)).

When the set of pages changes, the book scrolls smoothly so that the newest
changed page's right edge meets the book's right edge
(`mol/book2/book2.view.ts`, `sub()`).

| Screen | What the person sees | Standing |
|---|---|---|
| Wide | Menu, list and detail side by side. Leftover width goes to `placeholders`. | source |
| Narrow | The same strip. No page is removed or collapsed. Each is at most one screen wide, the rightmost is in view, the earlier ones are to the left, reached by a horizontal swipe that snaps page by page. | `inferred` from the CSS above; not checked on a phone |
| Narrow, first load | The book scrolls to the last page. With a default section set, a phone opens on that section, and the menu is off screen to the left. | `inferred` from `sub()`; not checked on a phone |

There is no breakpoint and no hamburger. The page count and their widths decide
everything.

### A catalog

`$mol_book2_catalog` is a book whose first page is a menu
(`mol/book2/catalog/catalog.view.tree`, `catalog.view.ts`).

| Property | What it does |
|---|---|
| `param` | Name of the URL argument that holds the open spread's id. |
| `spreads` | Dictionary id → view. Every id is a menu entry unless filtered out. |
| `Spread_default` | Shown when `param` is absent. By default `spreads['']`; `null` means the menu stands alone. |
| `menu_title` | Title of the menu page. A nested catalog's `menu_title` is also its entry text in the parent menu (`spread_title`). |
| `menu_tools` | Controls in the menu page's head. The kit's base adds `actions` and the parent's close icon here (`assets/ui/catalog/catalog.view.ts`). |
| `menu_foot` | Foot of the menu page. The kit holds the signed-in user, the build and Sign out there (`assets/ui/app/app.view.tree`). |
| `menu_link_content*` | What one menu entry shows. Default is the spread's title. The kit replaces it with a row of labelers. |
| `Spread_close` | A link with a close icon whose `arg` is `spread_close_arg`. |
| `spread_close_arg` | `{ [param]: null }`. Following it removes the argument, so the spread and everything right of it leave the book. |
| `menu_filter_enabled` | A search box appears in the menu on its own once there are 10 or more spreads. It matches `spread_title`, the page's title, not the row cells. |

A spread may itself be a catalog. Its pages are flattened into the parent's
strip (`pages_deep`), which gives the usual three columns: sections, the list
of one section, one item. `mol/book2/catalog/demo` nests two levels this way,
and each inner menu puts the outer `Spread_close` in its `menu_tools`. The kit
does the same: `$starterkit_app` holds `$starterkit_users_catalog`.

### The URL is the state

`$mol_state_arg` keeps the arguments in the hash: `#!key=value/key=value`
(`mol/state/arg/arg.web.ts`). A catalog reads its `param` from there. So the
address `#!nav=users/user=key12` is the whole state of the kit's panel:
section `users`, item `key12`. Opening that address opens those pages. That is
a deep link, and there is no router to configure.

Three behaviours a designer must know:

| Behaviour | Source |
|---|---|
| Order means depth. A link that sets `nav` keeps the arguments left of `nav` and drops everything from `nav` on. Switching section closes the open item. | `$mol_state_arg.link` → `dict_cut` |
| A link whose arguments all match the current URL is "current" (`mol_link_current`). Its `href` becomes the link with those arguments removed. Clicking the open row a second time closes it. | `mol/link/link.view.ts`: `uri_toggle`, `uri_off` |
| A `$mol_link` is a real `<a href>`, so the browser's Back returns to the previous address. Setting an argument from code (`$mol_state_arg.value(key, next)`) replaces the address instead. | `arg.web.ts`: `href()` uses `history.replaceState`; the Back half is `inferred` |

How a person goes back: swipe to an earlier page (narrow), click the close
link, click the open row again, or the browser's Back.

### A page

`$mol_page` (`mol/page/page.view.tree`, `page.view.css.ts`):

| Part | What it is | Scrolls away? |
|---|---|---|
| `title` | An `h1` in the head. The catalog also uses it as the menu entry text. | no |
| `tools` | Right side of the head. Wraps when crowded. | no, `Head` is `flex: none` |
| `body` | A `$mol_scroll`. The only part that scrolls. | yes |
| `foot` | A bar under the body. Hidden when empty. | no, `Foot` is `flex: none` |

`head` is `Title` plus `Tools`; replace it only to change that pair.

### Lists render lazily

`$mol_list` draws only the rows near the viewport and sizes the rest from each
row's `minimal_height` (`mol/list/list.view.ts`). The window grows by adding
each row's minimum until the viewport is covered, so rows that report 0 never
fill it and the list renders all of them (`inferred` from the loop in
`view_window`). Both lists in the kit set `minimal_height 24` on their rows.
`$mol_status`, `$mol_link` and the two halves of `$mol_labeler` carry their own
minimum.

`minimal_height()` asks `sub()` and each child for its minimum inside a `try`.
When that throws, including while a child waits on a request, it answers 24
(`mol/view/view/view.tsx`, the catch in `minimal_height`). Separately, a view
whose render waits on a request gets `mol_view_error="Promise"` and a shimmer.
Karlovsky: «Если произошла ошибка — показываем её пользователю там, где она
произошла», an error is shown to the person where it happened
(`author's statement`, [habr 491120](https://habr.com/ru/post/491120/)).

### Tokens

Colours come from `$mol_theme`, spacing from `$mol_gap`. Use the tokens, never
a literal colour or pixel gap.

| Token set | Names | File |
|---|---|---|
| `$mol_theme` | `back`, `card`, `field`, `hover`, `text`, `shade`, `line`, `focus`, `control`, `current`, `special`, `image`, `spirit`, `hue`, `hue_spread` | `mol/theme/theme.ts`, `theme.css` |
| `mol_theme` scopes | `$mol_theme_dark`, `_light`, `_base`, `_current`, `_special`, `_accent` | `mol/theme/theme.css` |
| `$mol_gap` | `page` 3rem, `block` .75rem, `text` .5rem .75rem, `emoji` .5rem, `round` .25rem, `space` .25rem, `blur` .5rem | `mol/gap/gap.ts`, `gap.css` |

The kit mounts `$mol_theme_auto` as a plugin on the root
(`assets/ui/app/app.view.tree`), which follows the system's light or dark
preference.

The default look is plain on purpose: «лучший дизайн — тот, что ты не
замечаешь», the best design is the one you do not notice (`author's
statement`, [habr 1019030](https://habr.com/ru/articles/1019030/)). Controls
should be finger-sized, about 40 CSS pixels (`author's statement`,
[habr 527574](https://habr.com/ru/articles/527574/)).

## What the author and the community say

The framework's documentation site has no page on designing screens. Its
section «Пользовательский опыт» holds one article, about a rich text editor.
Two pages bear on design, and both are the author's statements, not rules of
this kit.

From «Общие принципы» (Dmitry Karlovsky, 2025-01-21):

| Principle | In his words | What it asks of a screen |
|---|---|---|
| Человек ленив | «правильный путь должен быть легче неправильного» | The action a person came for is the nearest control, not the farthest |
| Человек не идиот | «Если делать продукт для идиотов - только идиоты им и смогут пользоваться» | No screen is spent on guarding against misuse nobody attempts |
| Собственный опыт полезнее чужого | «ненавязчиво разбросать повсюду хлебные крошки» | A count, a link and a status line where the person already looks, in place of a tour |
| Отказ от не необходимого | «Если от чего-то допустимо отказаться в некоторых случаях, то от этого следует отказаться вообще» | The same as rule 15 |

From «Дизайны»: the stock widgets are «намеренно аскетичным, чтобы их легко
можно было вписать в любое оформление, просто изменив CSS-константы», with a
full change of look left to the style cascade. A restyle therefore starts
from the theme and gap constants, not from markup.

From the public chat «$mol: Разработка» (t.me/mam_mol), 2026-06-15 to
2026-11-15, 4,555 messages read; the chat holds about 180,000 and the rest was
not read:

- The catalog's menu is a section of the book, not a side menu. Asked on
  2026-10-20 why the close cross closes everything, the author answered «это
  не сайд меню, это раздел».
- Spacing goes through the `$mol_gap` variables, not margins (a participant,
  2026-10-08), and the author pads a block on every side so that blocks stay
  separated when they wrap.
- Adaptivity is a question of order: «при уменьшении размера чем мы готовы
  жертвовать и в какой последовательности прятать» (a participant,
  2026-10-20). The author objected that hiding what is needed is not
  adaptivity. Not settled there.
- Horizontal scroll of the book is a property of the layout: on 2026-08-18,
  about a designer's objection to it, the author said «дизайнеры спорят с
  физической реальностью».
- Popups anchored to a control keep failing on small screens (2026-09-30,
  2026-11-01); opening them full-screen was proposed and not adopted.
- A custom theme is a selector on `[mol_theme="…"]` (2026-07-06), and the
  root must carry a theme (2026-11-01).
- The book's page markers are styled through `[mol_book2] > ::scroll-marker`
  (2026-10-17).
- A known defect: a catalog's list does not refresh when a spread id is
  pushed asynchronously (2026-10-14).

## Rules

### 1. The first screen says what waits and where to go

When an app has work that waits for a person → the default spread is a page
that names that work and links to it. It is not the first section of the menu.

- Why it matters on a phone: the default spread is the screen a phone opens on
  (see the model). An empty one is a blank first screen.
- In this kit: `assets/ui/app/app.view.ts` opens Users for an admin and the
  bare menu for anyone else. The kit has no waiting work, so it has no such
  page yet.
- Standing: `house rule`, decided for one page that had waiting work. As a
  rule for every app it is `inferred`.

### 2. Waiting work is counted in the menu

When a section can hold items that wait for a decision → its menu entry shows
how many.

- In this kit: no counter yet. Menu entries are the section titles.
- Standing: `house rule`, decided for one page. As a rule for every app it is
  `inferred`. Where the number is drawn and whether a zero is shown is open.

### 3. A row carries enough to choose from

When a person picks one item out of a list → the row shows what tells that item
from its neighbours: who or what it concerns, a recognisable piece of its
content, its state, when.

- In this kit: `assets/ui/users/catalog/catalog.view.tree` (id, email, status,
  last sign-in).
- Standing: `inferred` from the worked example.

### 4. A row is a `$mol_row` of labelers

When building a list → each row is a `$mol_row` of labelers, each cell with its
own title, so the row reads as a card. No `$mol_expander` for a list, no HTML
table. Column widths go in the labeler's stylesheet. Every row sets
`minimal_height`.

- In this kit: `assets/ui/users/catalog/catalog.view.tree`,
  `assets/ui/auditlog/auditlog.view.tree`, and
  [frontend-mol.md](frontend-mol.md#rows-are-labelers-not-strings). The kit's
  labelers have no stylesheet, so no column widths yet.
- Standing: `house rule`.

### 5. A detail page: title names the thing, fields stack vertically

When showing one item → `title` is the item's own name, because the same text
is its menu entry and the search target. The body is a vertical list of plain
`$mol_labeler` fields. No `$mol_row` on a show page.

- In this kit: `assets/ui/users/show/show.view.tree`: `title <= email`,
  `Fields $mol_list`.
- Standing: `house rule` for the vertical fields. The title half is `inferred`
  from `spread_title`.

### 6. Every decision on an item sits in the tools bar

When a page lets a person act on the item → all the action controls are in
`tools`. Both halves of a pair (send / do not send) sit side by side. Nothing a
person must press lives at the end of `body`.

- Why: `tools` is in the head, which does not scroll
  (`from the framework's source`). A control at the end of `body` is below
  everything the body holds.
- In this kit: `assets/ui/users/show/show.view.tree` keeps Make Admin, Remove
  Admin, Ban and Unban in `tools`.
- Standing: `house rule`, decided for one page. As a rule for every app it is
  `inferred`, with the kit's user page as `observed in this kit`.

### 7. An action that needs a reason opens a further page

When an action needs input before it can run (a reason) → the control in
`tools` is a link that opens one more page of the book. That page states what
is about to happen, asks for what it needs, and holds the final button and a
way back.

- The same shape serves a confirmation: a Delete link opens a page that names
  the item, says what is lost, and offers Cancel and Delete. On success it
  clears every URL argument it used.
- In this kit: no further page yet. `assets/ui/users/show/show.view.tree`
  keeps Ban in `tools` and its Reason field in the body under the fields,
  which is the shape this rule replaces.
- Standing: `house rule` for an action with a reason, decided for one page; as
  a rule for every app `inferred`. Which destructive actions get a
  confirmation page, and where the final button sits, are open questions.

### 8. A result or a refusal is shown as data, beside what caused it

When a mutation answers → its outcome is text on the page. A domain refusal is
an `ErrorPayload`, rendered, never thrown. A field's refusal goes under that
field (`bid`). A page-level outcome goes in a `$mol_status` line.

A refusal only the server can know is not guessed in the client: the button
stays live and the answer is shown. State the page already holds may switch a
button off. A control that can never act for this item is rule 15's case.

- In this kit: `assets/ui/users/show/show.view.ts`. The Reason field's refusal
  arrives through `reason_bid`; every mutation's answer lands in `message`.
  `ban_enabled`, `unban_enabled` and `make_admin_enabled` follow state the
  page holds. Remove Admin stays clickable on your own account, and the
  server's refusal is what you read.
- Standing: `observed in this kit` for the first paragraph, with
  [frontend-mol.md](frontend-mol.md#errors-are-data) and the `ErrorPayload`
  rule in [rules.md](rules.md). The second paragraph is `inferred` from that
  one screen.

### 9. One thing reached from two sections looks and acts the same

When the same item opens from two places → it is the same page design in both,
with the same actions in the same place. A read-only view of an item that
waits for a decision is not a dead end. How it stops being one, by carrying
the buttons or by linking to the section that acts, is an open question.

- In this kit: no item opens from two places yet.
- Standing: `inferred` from the worked example.

### 10. Long text keeps its shape and never buries a control

When a field holds text a person wrote → render it with `$mol_text`, which
breaks it into paragraphs. A `$mol_view` with a string in `sub` renders one
line. Machine output goes in a `pre`. A long list pages with an explicit
control. A truncated log says it is truncated.

- In this kit: `assets/ui/auditlog/auditlog.view.tree` pages with `Show more`
  under its list, and `assets/ui/users/catalog` under the menu links, both
  through the shared `$starterkit_paging` fed by `endCursor`.
  The kit shows no long text yet.
- Standing: `house rule` for the line breaks, decided for one page;
  `observed in this kit` for the paging; the rest `inferred`.

### 11. Empty, loading and missing each say what they are

| When | Do | In this kit | Standing |
|---|---|---|---|
| A list has no rows | Set `Menu_links_empty` / `Empty` to a sentence that says what would fill it | not yet: neither list declares one | `inferred` |
| A view has nothing to say | Leave it out of `body()` when its text is empty. Do not render it empty. | not yet | `house rule`, from a measured layout jump: two empty views pushed a polling page down 48px on every poll |
| Data is reloading over data already shown | No loading state. Keep the shimmer for views with no content yet: `[mol_view][mol_view_error="Promise"]:not(:empty) { background: none; animation: none }` | not yet: the kit has no such stylesheet | `house rule` |
| A value was not measured | Say "not measured" | not yet | `house rule` for health figures read from a metrics store; for any other value `inferred` |
| A deep link names an item that is gone | Say so on the page | `assets/ui/users/show/show.view.ts` throws «User not found», which $mol shows where the view would be | `observed in this kit` |
| A date is null | Render `-` | `assets/ui/labeler/moment/moment.view.ts` | `observed in this kit` |

A timer refetch does not suspend the view: $mol serves the previous value while
it recomputes. Measure a suspected jump with `getBoundingClientRect().top`
before, during and after a delayed response, not by eye.

### 12. An entity gets four screens: catalog, create, update, show

When adding an entity → build `catalog`, `create`, `update`, `show`, copied
from a neighbour.

- Create is a page of its own, opened by an add link in the catalog's
  `menu_tools`. On success it navigates to the item it made.
- Update is a page of its own, opened from the show page's tools, with a
  Cancel link back to the item.
- Fields are `$mol_form_field`: `name`, `bid` for the refusal, a `hint` on the
  control for what may stay empty.
- Create, update and other non-item pages share the catalog's `param` with the
  items, so they are hidden from the menu by overriding `spread_ids_filtered`.
- In this kit: `assets/ui/users/catalog` and `assets/ui/users/show` only.
  Users arrive by signing in, so the kit has no create or update page yet.
  The base catalog already carries `actions` for the add link
  (`assets/ui/catalog/catalog.view.tree`).
- Standing: `house rule` for the four screens. The bullet details are the
  origin project's practice; whether a show page may double as the form is an
  open question.

### 13. A deep link is a contract

When anything outside the page links into it (a button in a chat message, an
email, an API field) → treat the argument names and values as a published
interface.

- Shape, the framework's part: `#!<param>=<value>/<param>=<value>`, separated
  by `/`, in order of depth; a link that sets one argument drops those after
  it.
- Shape, the kit's part: `#!nav=<section>/<param>=key<id>`. An item's value is
  `key` plus its id. That prefix is the kit's convention (`mapKeys` in
  `assets/ui/users/catalog/catalog.view.ts`, the `key` handling in
  `assets/ui/gql/gql.ts`); it keeps item ids apart from reserved words such as
  `add` that share the argument.
- The server builds the link, and a test asserts that it opens a card. A link
  built with `;` for `/`, or a bare id for `key<id>`, opens the app and no
  item, and a test that only compares strings will not notice.
- Must stay stable once a message carries it: each catalog's `param`, the
  section ids in `spreads`, the `key` prefix.
- An argument must not outlive its page ([rules.md](rules.md), "Nothing exists
  that does nothing").
- Standing: `from the framework's source` for the argument syntax and the cut;
  `observed in this kit` for `nav` and the `key` prefix; `house rule` for the
  tested link. That it is a contract is `inferred`.

### 14. One name per thing, and the page speaks like the message that led to it

When naming a section, a button or a state → use one name everywhere, and make
it the words the person already read in the notification that sent them here.
Use the human word; keep the internal name small and grey beside it if it is
needed at all.

- In this kit: sentences a person reads outside the page live in
  `internal/texts` ([texts.md](texts.md)). The page's labels are literals in
  `*.view.tree`. Nothing ties the two together yet.
- Standing: `house rule` for one name per thing. Matching the notification is
  `inferred` from the worked example. The language of labels is an open
  question.

### 15. A screen contains nothing that does nothing

When a control cannot act for this item → leave it out. No button that is
never enabled, no field nobody reads, no section that is always empty, no link
to a page that says nothing.

- Standing: `house rule`, [rules.md](rules.md), "Nothing exists that does
  nothing".

## Words the checklists use

| Word | What it is on screen | How to test it |
|---|---|---|
| Page | One column of the strip, with a title bar on top | Count the columns |
| Menu | The leftmost page: the list of sections, or of items | It is there when nothing is open |
| Spread | The page a menu entry opens, to the right of the menu | Click an entry; a column appears |
| Default spread, first screen | The spread shown at the bare address, before any click | Open the address with nothing after `#` |
| Further page | One more column opened from an item page, for a reason or a confirmation | A link in the item's title bar adds a column |
| Tools | The controls at the right of a page's title bar | They stay put when the page is scrolled |
| Body, foot | The scrolling middle of a page; the fixed bar under it | Scroll: the body moves, the foot does not |
| Row | One menu entry of a list | It is one click target |
| Labeler | A small grey title with a value under it; one cell of a row, or one field of an item | Every value on screen has such a title |
| Status line | One line of text where an action's result or refusal appears | Press the button and watch it change |
| `minimal_height` | A number in the row's code that lets a long list draw only what is visible | Open the row's `.view.tree` and look for it |
| Deep link | An address that opens a given item directly | Paste it in a new tab: the item is open |
| Counter | A number beside a menu entry saying how many items wait | Read it without opening the section |

## Checklist: designing a new screen

1. Who lands here, and from where: the menu, or a link in a message? Write the
   deep link first (rule 13).
2. What is the one thing they came to do? Is it reachable without scrolling
   (rule 6)?
3. Is this the first screen? Does it say what waits (rules 1, 2)?
4. For a list: could you pick the right row out of nine similar ones (rule 3)?
   Rows are labelers with `minimal_height` (rule 4).
5. For an item: title is its name; fields vertical; actions in `tools`
   (rules 5, 6).
6. For each action: does it need a reason? Then a further page (rule 7). Where
   does its result appear (rule 8)?
7. Is the same item shown anywhere else? Make the two agree (rule 9).
8. What is the longest content this can hold? Where does it go (rule 10)?
9. Write the empty, loading, missing and refused states in words (rule 11).
10. Read every label aloud next to the notification that leads here (rule 14).
11. Count the pages open at the deepest point. Each one is a swipe on a phone.
12. Delete what does nothing (rule 15).

## Checklist: reviewing a screen by clicking through it

Tests prove the contract; only a hand proves the surface. A screen is not done
when its suite is green. It is done when a person has clicked through it on a
running stand (`make up`, `make dev`) and it did what they expected.

1. Open the app at its bare address, signed in as the person it is for. Is the
   first screen the one rule 1 asks for? Repeat at phone width.
2. Arrive the way a person does: follow the real link in the real message.
   Does it open the item, not just the app?
3. With waiting work seeded: can you say from the menu alone how many items
   wait and where?
4. Do the task start to finish without being told where anything is. Note
   every place you hesitated.
5. Use the longest realistic content. Are the controls still on screen?
6. Empty every list. Does each say what it is?
7. Make the server refuse: a wrong value, a stale item. Is the refusal on the
   page, next to its cause?
8. Reload on the deepest page. Does the same state come back? Press the
   browser's Back. Click the open row again.
9. Watch a polling page for ten seconds. Does anything jump?
10. Press every control. One that does nothing is a defect.
11. Take a screenshot per claim, one line each: "task: X — this shows X done".
    Wide and phone width. A reader who has not seen the work judges whether
    each image shows what its line says.
12. In the report, say in words what a person did, from which account and
    device.

## Open design questions

Forks the sources do not settle. None is chosen here; decide per project.

| # | Question | One way | The other way |
|---|---|---|---|
| 1 | Several sections that all mean "waiting on you". Merge them? | One inbox: one place to look, one page design, one counter. Kinds of item with different actions share a list. | Separate sections: each list is uniform. The person must know which kind they have; a first screen and counters may be enough. |
| 2 | Language of labels on a page whose reader got a notification in another language | English on the panel, as [rules.md](rules.md) says of its UI copy. The page then never matches the notification's words. | Follow the reader's language through `internal/texts`. The SPA needs localisation. |
| 3 | The way back from an item page | A close icon (the framework's `Spread_close`) | A text link, or nothing beyond the swipe and the open row, as the kit's user page has now |
| 4 | On the further page of rule 7, where is the final button | In `foot` with Cancel | In the form's own buttons under the reason field, or in `tools` |
| 5 | Which one-click destructive actions get a confirmation page | All that cannot be undone | Only those that destroy data; an action with an inverse stays one click |
| 6 | An exception to the four screens of rule 12 | No exception: every entity gets an update page | The show page may be the form when every field is editable |
| 7 | The first screen of an app with no waiting work | A dashboard anyway | The main list, as the kit has now |
| 8 | A waiting item opened from a journal | The journal page carries the same buttons | The journal stays read-only and links to the section that acts |
| 9 | The counter | In the menu entry's text, shown only when above zero | A separate cell, zero shown, so an empty section is visibly empty |
| 10 | Status line position | First in body, always in view | Last, next to the form it reports on, as the kit's user page has now |

## Sources

Framework: the `mol/` directory of a mam workspace, checked out 2026-11-16.
Read: `book2` (tree, ts, css, readme, demo), `book2/catalog` (tree, ts, css,
readme, demo), `page`, `list`, `scroll`, `link`, `state/arg/arg.web.ts`,
`text/text` (head of the tree), `status`, `labeler`, `row`, `form`,
`form/field`, `theme` (`theme.ts`, `theme.css`), `gap`, `view/view/view.tsx`
(`minimal_height`, `minimal_width`, error marking).

The author, Dmitry Karlovsky:

- [$mol: 4 года спустя](https://habr.com/ru/post/491120/) and
  [Автоматическая виртуализация рендеринга произвольной вёрстки](https://habr.com/ru/articles/537388/)
  — first 60% of each read verbatim
- [Мифы о веб-фреймворке $mol](https://habr.com/ru/articles/1019030/),
  [Mathematics Driven Design: размеры и отступы](https://habr.com/ru/articles/527574/),
  [Продвинутый CSS-in-TS](https://habr.com/ru/articles/523646/),
  [Простенькие заметки на $mol](https://habr.com/ru/articles/732934/)
- [Большой каталог на почитать про $mol](https://habr.com/ru/articles/1047754/),
  as a list of links
- The documentation site [mol.hyoo.ru](https://mol.hyoo.ru): «Общие принципы»,
  «Дизайны»
- The public chat «$mol: Разработка», t.me/mam_mol

None of the articles names `$mol_book2`, `$mol_book2_catalog`, `$mol_gap` or
`minimal_height`. Everything this guide says about those comes from the source.

This kit: every `*.view.tree` and `*.view.ts` under `assets/ui`,
[frontend-mol.md](frontend-mol.md), [rules.md](rules.md), [texts.md](texts.md).

## Not read

- The $mol documentation site beyond a crawl of 64 pages in a real browser on
  2026-11-16 (a plain fetch gets an empty shell). The site renders lazily, so
  a long page was captured only as far as one tall viewport shows. Read in
  full: «Общие принципы», «Дизайны». Not read to the end: «Композиция
  компонентов», «Каскадные стили компонент», «Mathematics Driven Design».
- The chat «$mol: Разработка» before 2026-06-15.
- The last 40% of habr 491120 and 537388: they came back as a summary only and
  are not quoted.
- Karlovsky's «Идеальный UI-фреймворк» (276747), «Хоп, Хлоп, и MVP готов»
  (820871), «Принцип минимизации злобы» (786640), «UX редактирования богатого
  текста» (1095498); pages 2 to 7 of his article list.
- No page was opened in a browser, at any width.
- Not traced: what a row's `minimal_width` changes on screen.
