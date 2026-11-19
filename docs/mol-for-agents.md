# $mol for coding agents

One file to read before writing $mol. Rules first, one minimal example each,
then the mistake the rule prevents. [frontend-mol.md](frontend-mol.md) is how
this kit builds and serves the SPA; [mol-interface-design.md](mol-interface-design.md)
is what a screen should look like. This page is the language and the framework.

Standing of what is written here:

- `mol/...` paths are in the mam workspace (`MAM_WORKSPACE`, default `../mam`).
  Each rule that names one was read there.
- Every `$my_*` example on this page was built with `mam` and its tests ran.
  "Measured" means observed in headless Chromium on that build.
- `assets/ui/...` paths are this kit. A "kit convention" is a choice of this
  kit, not of the framework.

## 0. The model in ten lines

1. A component is a **directory**. Its global name is its path: `my/user/card/` ⇄ `$my_user_card`.
2. There are **no imports**. Writing `$my_user_card` anywhere makes the builder include that directory.
3. `card.view.tree` declares the class: properties, children, bindings. It compiles to a TS class.
4. `card.view.ts` is a subclass with the same name in `namespace $.$$` that overrides properties. Logic lives there.
5. Every property is a **method**: `title()` reads, `title(next)` writes. No fields, no props object, no state hook.
6. `@$mol_mem` caches a method and tracks what it read. When a source changes, dependants recompute. Nothing subscribes by hand.
7. Rendering is a memoised method too. A component re-renders because it read something that changed.
8. Async reads look synchronous: a pending value **throws a Promise**, the framework catches it and re-runs the method when it resolves.
9. Styling hooks are **attributes**, not classes: `<my_user_card my_user_card mol_view>`. The tag is the class name too.
10. State that must survive a reload lives in the URL (`$mol_state_arg`) or local storage (`$mol_state_local`).

## 1. Naming is the path

| Rule | Example |
|---|---|
| FQN `$a_b_c` ⇄ directory `a/b/c/`; files inside are `c.<kind>` | `my/user/card/card.view.tree` declares `$my_user_card` |
| The first segment is the package: a top-level directory of the workspace | this kit's `assets/ui` is linked as `<workspace>/starterkit`, so `assets/ui/users/show/` is `$starterkit_users_show` |
| A name with no directory of its own resolves to the nearest existing parent | `$starterkit_gql_make_map` lives in `assets/ui/gql/gql.ts` |
| Sub-component properties are `Capitalised`, value properties `lower_snake` | `Row*`, `Email_field` return views; `row_email*`, `email` return values |
| Anything whose file name does not start with a letter or digit is skipped by the builder (`mol/build/build.node.ts`, `mods`) | `-/`, `-view.tree/`, `-css/` are build output; `.gitignore` has `-*` |
| File tags pick the bundle | `x.web.ts` browser only, `x.node.ts` node only, `x.test.ts` test bundle only, `x.view.tree.locale=ru.json` a translation |

Mistakes:

- `import`/`export`/`require` of a neighbour. Nothing is imported; write the FQN.
- A class named differently from its directory. The builder finds code by name → path, so the bundle misses it and the browser throws `$x is not defined`.
- **A dollar-word in a string.** The builder scans every `.ts` for `$word` (`mol/build/build.node.ts`, `dependors['ts']`). A GraphQL `$limit` in a hand-written string reads as a package named `limit`, and the build fails with `Root package "limit" not found`. Write `\u0024limit`: TypeScript reads it as the same character, the scanner does not. The kit's codegen already does this in every `*.graphql.ts`.

## 2. mam: the builder

mam is one shared process for the whole workspace, not a per-project bundler.

| Do | How |
|---|---|
| Link the kit in | `make ui-link` → `<workspace>/starterkit` → `assets/ui` |
| Build once | `make ui` = `node node_modules/.bin/mam starterkit/app` in the workspace |
| Dev server | `make mol-dev` → `http://127.0.0.1:9080/`; it builds what a URL asks for; reuse a running one |
| Open the app in dev | `/<pkg>/<module>/-/test.html`. A request for a module directory that holds an `index.html` redirects there (`mol/build/server/server.node.ts`, `expressIndexRequest`) |

An app mounts where an element names it: `<div mol_view_root="$my_app"></div>`
plus `<script src="web.js">` (`mol/view/view/view.tsx`, `roots`). A module
without an `index.html` still gets a `-/test.html` that mounts
`$<pkg>_<module>`. This kit's production shell is `assets/index.html`.

What a build writes into `<module>/-/`:

| File | What |
|---|---|
| `web.js`, `web.css` | the browser bundle; styles are also inside `web.js` |
| `test.html` | the module's `index.html`, or a generated page mounting `$<pkg>_<module>`, plus `web.test.js` and the dev reload client. The app **and** its tests in a browser |
| `index.html` | a plain copy of the module's `index.html`, when it has one |
| `web.test.js`, `node.test.js` | the test bundles |
| `web.d.ts`, `web.locale=en.json`, `web.deps.json` | types, the extracted `@` strings, the dependency graph |

Rules:

- A build type-checks the whole dependency closure, then runs `node.test.js`. A tree syntax error, a type error or one failed test makes `mam` exit non-zero (measured: 135).
- **Judge a build by its exit code, never by `-/web.js` existing.** The bundle is written before the tests run, so a failed build can leave a fresh one behind.
- Type errors in a tree are reported against generated files: `x/-view.tree/x.view.tree.d.ts(4,12): error TS2304`. Fix the `.view.tree` next to it.
- In dev, `Refused to execute script … MIME type ('text/html')` means the build failed: the body served as `web.js` is the error text. Read it.
- An external package is one line in a `.meta.tree` of the directory that will hold it: `pack mol git \https://github.com/hyoo-ru/mam_mol.git`. mam clones it on first use and runs `git pull` on later builds unless `MAM_PULL_DISABLED=1`.
- Stale index: a file added to a module while the dev server runs can be missing from the bundle although type-checking passes; the browser then throws `ReferenceError: $x is not defined`. Check with `grep -c '$x' <module>/-/web.js` against a working neighbour; clear with `rm -rf <dir>/-view.tree` and rebuild.
- A full build asks the npm registry for the versions of `<pkg>_<module>` (`npm view …`, seen in the build log). The module's name leaves the machine.

Mistakes: `npm run build` inside the repository; a second dev server (`EADDRINUSE`); `ln -sfn` onto an existing directory, which puts the link inside it and builds an empty package.

## 3. view.tree

Indentation is **tabs**. One space of indent is `Wrong nodes separator`. A node
is `name value`, children one tab deeper.

```tree
$my_syntax $mol_view
	- a remark: this node and its children are ignored
	label \Plain string, anything up to the end of the line
	greeting @ \Shown to a person, so it is localised
	limit 10
	enabled true
	icon null
	tags /string
		\a
		\b
	options *
		yes @ \Yes
		no \No
	attr *
		^
		my_syntax_kind <= kind \plain
	style *
		width <= width_px 120
	event *
		^
		mouseenter? <=> hover? null
	sub /
		<= Head $mol_view
			sub <= head /
				<= label
		^ extra /
		<= Page $mol_page
			Title => Page_title
			title <= greeting
```

| Syntax | Meaning | Generated TS |
|---|---|---|
| `$my_syntax $mol_view` | declare a class and its base | `class $my_syntax extends $mol_view` |
| `label \text` | string; `\` takes the rest of the line raw | `label() { return "text" }` |
| `limit 10`, `enabled true`, `icon null` | number, boolean, null: no prefix | `limit() { return 10 }` |
| `@ \text` | localised string; key is `<class>_<property path>` | `this.$.$mol_locale.text('$my_syntax_greeting')` |
| `/` | list; `/string` types the items | `tags(): readonly string[]` |
| `*` | dictionary | `options() { return { yes: …, no: … } }` |
| `^` in `*` or `/` | splice the base class's value in | `{ ...super.attr(), … }` |
| `^ extra /` inside a list | declare `extra` and splice its items | `[ this.Head(), ...this.extra(), this.Page() ]` |
| `<= name default` | read binding: declare `name` on **this** class with that default and use it here | `kind() { return "plain" }` |
| `<= Head $mol_view` | declare a memoised child instance and place it | `@$mol_mem Head() { return new $mol_view }` |
| nested lines under a child | override that child's properties from the owner | `obj.sub = () => this.head()` |
| `name? <=> other? default` | two-way: the child's `name` **is** the owner's `other` | `obj.name = next => this.other(next)` |
| `Title => Page_title` | expose the child's `Title` on the owner | `Page_title() { return this.Page().Title() }` |
| `name?` | the property takes a `next` argument (writable, memoised) | `@$mol_mem name(next?)` |
| `Name*`, `name*` | the property takes a key (§5) | `@$mol_mem_key Name(id)` |
| `- …` | remark | nothing |

`$mol_view` gives every component these slots (`mol/view/view/view.tsx`):

| Property | Becomes | Note |
|---|---|---|
| `sub /` | child nodes: views, strings, numbers, `null` | `$mol_list` rows must be views |
| `attr *` | DOM attributes | `null`/`false` removes; start with `^` |
| `style *` | inline style; a number is px | only for a value computed per instance (§11) |
| `event *` | DOM listeners, bound once | each handler runs in a fiber, so it may read async data |
| `field *` | DOM properties (`value`, `scrollTop`) | |
| `plugins /` | `$mol_plugin`s acting on this element (§10) | |

Mistakes:

- `title Hello` without `\` → `error TS2304: Cannot find name 'Hello'`.
- A dictionary without `^` (`attr *` then one key) drops the base attributes, such as `mol_theme`.
- **A misspelt name inside a list builds green.** `sub /` items are untyped, so `<= labell` for a declared `label` compiles and the browser throws `TypeError: this.labell is not a function`. In a typed slot (`title <= labell`) the same typo fails the build with `TS2339`. Re-read every name under `/`, and open the screen.
- Reusing one child instance in two places (`<= Close` twice). A view has one DOM node; declare two children.
- A string in `$mol_list rows`. Wrap it: `<= Note $mol_view sub / <= note \`.
- Logic in the tree. A tree has no conditions and no loops; compute the list or the value in `view.ts`.
- Binding types are checked: wiring a `number` property into a `string` slot fails the build in the generated `.d.ts`.

## 4. view.ts: the override file

```tree
$my_counter $mol_view
	count? 0
	sub /
		<= Value $mol_view
			sub /
				<= count_text \0
		<= Inc $mol_button_minor
			title @ \Add one
			enabled <= inc_enabled true
			click? <=> inc? null
```

```ts
namespace $.$$ {
	export class $my_counter extends $.$my_counter {
		override count_text() {
			return String(this.count())
		}

		override inc_enabled() {
			return this.count() < 3
		}

		override inc(next?: Event) {
			if (next === undefined) return null

			this.count(this.count() + 1)

			return null
		}
	}
}
```

- The file is `namespace $.$$ { export class $x extends $.$x { … } }`. `$` and `$$` are one namespace at runtime; `$.$$` is how the subclass takes the name over.
- **Write `override` on every member the tree or a base class declared** (`title()` of `$mol_page` counts). A misspelt name then fails the build (`TS4117: … not declared in the base class`). Without it the typo is a new method nobody calls and the screen shows the tree's default.
- A member without `override` is the component's own helper (`row(id)`, `user()`).
- An override replaces the generated method, decorator included. A tree `name?` is memoised; your override is not unless you write `@$mol_mem` on it again.
- A click handler is a channel: the button calls it with the event. Kit convention: `if (next === undefined) return null`, do the work, `return null`.
- Reach globals through `this.$`: `this.$.$mol_state_arg`, `this.$.$mol_fetch`. Tests swap `$` (§12).
- No constructor, no fields with state, no `this.x = …`. State is a `@$mol_mem` method with a `next`.

## 5. Keyed properties: `Name*`

```tree
$my_tasks $mol_list
	rows <= task_rows /
	Empty $mol_status
		message @ \No tasks yet
	Task_row* $mol_row
		minimal_height 40
		sub /
			<= Task_done* $mol_check_box
				title <= task_title* \
				checked? <=> task_done*? false
```

```ts
namespace $.$$ {
	export class $my_tasks extends $.$my_tasks {
		@$mol_mem
		titles() {
			return new Map([['t1', 'Write the tree'], ['t2', 'Override in view.ts']])
		}

		@$mol_mem
		override task_rows() {
			return [...this.titles().keys()].map(id => this.Task_row(id))
		}

		override task_title(id: string) {
			return this.titles().get(id) ?? ''
		}

		@$mol_mem_key
		override task_done(id: string, next?: boolean) {
			return next ?? false
		}
	}
}
```

- `Task_row*` is a factory memoised per key: `this.Task_row('t1')` is the same instance every time. The key flows down to every `name*` inside it.
- The tree declares the row **shape** once. `view.ts` returns the **list**: ids mapped through the factory.
- Per-row values are `name(id)`; per-row state is `@$mol_mem_key name(id, next?)`.
- Use a stable id as the key. An index is right only for an append-only list (`assets/ui/auditlog/auditlog.view.ts`).

Mistakes: writing one child per row in the tree; `new $my_row()` in `view.ts` (a new instance every render, state lost); a `*` child whose inner properties forgot the `*`, so every row shows the same value.

## 6. Reactivity

| Decorator | On | Meaning |
|---|---|---|
| `@$mol_mem` | `name(next?)` | cached; recomputed when something it read changed; `name(v)` stores `v` |
| `@$mol_mem_key` | `name(key, next?)` | the same, one cache per key |
| `@$mol_action` | `name(...)` | a command: runs once even when the fiber around it restarts |
| none | `name()` | a plain method: recomputed on every call, still tracked by whoever calls it |

```ts
@$mol_mem
reason(next?: string): string {
	return next ?? this.ban_reason()
}
```

That is the whole pattern for a draft over server data: the server value until
the person types, then theirs (`assets/ui/users/show/show.view.ts`).

- Dependencies are discovered by execution. No dependency arrays, no selectors, no `subscribe`, no `watch`, no event bus.
- Put `@$mol_mem` on what is expensive or must keep identity: a request, a built list of views, a created object. A one-line derivation needs none.
- A method that performs a request without `@$mol_mem` hits the network on every re-render.
- Setting a value from inside a memoised getter is a design error. Derive instead: a second `@$mol_mem` that reads the first.
- Polling is reading a clock: `this.$.$mol_state_time.now(5000)` inside a `@$mol_mem` makes it recompute every 5 s (`mol/state/time/time.ts`). No `setInterval`.
- An object returned from `@$mol_mem` is owned by it: when the cache is dropped, its `destructor()` runs. Close sockets and timers there.
- Shared logic is a `$mol_object` subclass with memoised methods, not a view (`assets/ui/paging/paging.ts`).

## 7. Suspense: async reads as plain values

```ts
namespace $.$$ {
	export class $my_profile extends $.$my_profile {
		async load_async() {
			return { name: 'Ada' }
		}

		@$mol_mem
		data() {
			return $mol_wire_sync(this).load_async()
		}

		override name() {
			return this.data().name
		}
	}
}
```

- `$mol_fetch.json(url, init)` is already synchronous (`mol/fetch/fetch.ts`). `$mol_wire_sync(obj).method()` makes any promise-returning method so.
- While the promise is pending, the call **throws it**. Every caller up to the rendering view unwinds. When it resolves, the methods run again and this time get the value.
- The only `async` method a component may hold is the source itself (`load_async` above), and it is called only through `$mol_wire_sync`. Everything that reads it is synchronous: no `await`, no `.then`, no loading flag, no `try/catch` that swallows everything. A `catch` must rethrow promises: `if ($mol_promise_like(error)) $mol_fail_hidden(error)` (`mol/status/status.view.ts`).
- The view that could not render gets the attribute `mol_view_error="Promise"` and a shimmer (`mol/view/view/view.css`). A real error puts its name there (`mol_view_error="Error"`), red stripes, and the message as the view's text. The failure shows where it happened, the rest of the page works. So `throw new Error('User not found')` in a data method **is** the not-found state.
- **Code before a suspending call runs again after it** (measured: a handler body ran twice, its `@$mol_action` once, the request once). Keep everything before the request idempotent, or put the side effect in an `@$mol_action`.
- Event handlers from `event *` and `click?` already run in a fiber. A mutation is a synchronous call inside the handler (`assets/ui/users/show/show.view.ts`, `ban`).
- From code that is not a fiber (a test, a raw callback) call through `$mol_wire_async(obj).method()`, which returns a promise.
- A pause is `this.$.$mol_wait_timeout(ms)` inside a fiber. `setTimeout` for data is always wrong.

Layout traps, all from suspense:

| What happens | Why | Do |
|---|---|---|
| The page shimmers on every poll | Measured: on a refetch the old content stays in the DOM, and the view still gets `mol_view_error="Promise"` until the answer comes | `[mol_view][mol_view_error="Promise"]:not(:empty) { background: none; animation: none }` in the app's stylesheet: shimmer only where there is nothing yet |
| A list jumps while loading | A view whose `sub()` throws reports `minimal_height()` 24 (`view.tsx`, the catch branch), and `$mol_list` sizes unseen rows from that | set `minimal_height` on each row; leave a view with nothing to say out of `body()`/`sub()` instead of rendering it empty |
| One slow field blanks the whole page | The suspension unwinds to the first view that reads it. `title <= slow` on a page suspends the page | read slow data in the smallest view that shows it |

Measure a suspected jump with `getBoundingClientRect().top` before, during and after a delayed response. Not by eye.

## 8. Lists

- `$mol_list` renders only rows near the viewport and pads the rest from each row's `minimal_height()` (`mol/list/list.view.ts`, `view_window`). Rows that report 0 defeat it: every row renders.
- Give each row a fixed minimum: `minimal_height 24` on a `$mol_row` of labelers (kit convention), 40 for a row holding a control.
- `Empty` is what the list shows with no rows. Give it a sentence: `Empty $mol_status message @ \…` in a list subclass, `Empty <= List_empty $mol_status` on a child list.
- Never slice or window rows yourself, never add a scroll listener. Hand the list every row view; it asks only the visible ones to render.
- A page of server data is `$starterkit_paging` (`assets/ui/paging/paging.ts`): the screen supplies `fetch`, the "Show more" button calls `more()`.
- A row is a `$mol_row` of `$mol_labeler` cells, each with its own `title`. No `<table>`, no `$mol_expander` for a list.

## 9. Pages, the book and the URL

```tree
$my_app $mol_book2_catalog
	param \section
	menu_title @ \My app
	plugins /
		<= Theme $mol_theme_auto
	menu_tools /
		<= Lights $mol_lights_toggle
	spreads *
		tasks <= Tasks_page $mol_page
			title @ \Tasks
			tools /
				<= Spread_close
			body /
				<= Profile $my_profile
				<= Counter $my_counter
				<= Tasks $my_tasks
		signup <= Signup $my_signup
```

Measured on this app: the menu links are `#!` (current) and `#!section=signup`;
opening the second sets the hash and the document title to `Sign up | My app`.

| Piece | Rule |
|---|---|
| `$mol_page` | `title`, `tools /` (head, right), `body /` (the only part that scrolls), `foot /` |
| `$mol_book2_catalog` | first page is a menu; `spreads *` maps a URL value to a page; `param` names the URL argument; `Spread_default` shows when it is absent; `Spread_close` is a link that removes it |
| Nesting | a spread that is itself a catalog adds its pages to the same strip: sections → list → item (`assets/ui/app`, `assets/ui/users/catalog`) |
| List → item | override `spreads()` to map row ids to `Show_page(id)`; `menu_link_content*` draws the row (`assets/ui/users/catalog/catalog.view.ts`) |
| URL format | `#!key=value/key=value` (`mol/state/arg/arg.web.ts`) |
| Read | `this.$.$mol_state_arg.value('user')` → `string \| null`, reactive |
| Write from code | `.value('user', 'key12')` replaces the address, no history entry; `.go({ user: 'key12' })` navigates, with one; `null` removes the key |
| Link | `$mol_link` with `arg * user \key12`: a real `<a href>`. It marks itself `mol_link_current`, and a current link's `href` removes its arguments, so a second click closes |
| Order is depth | a link that sets `section` keeps the arguments left of it and drops those from it on (`dict_cut`) |

- A menu entry's text is the spread's `title()`, or `menu_title()` when the spread is a catalog (`mol/book2/catalog/catalog.view.ts`, `spread_title`). A title that reads a request makes the menu entry wait for it too.
- There is no router, no route table and no history API in your code. A screen is open because an argument is in the URL.
- A dialog, a confirm step, a filter, a tab: each is a URL argument, so it can be linked and survives a reload.
- Kit convention: numeric ids become spread keys with a `key` prefix (`$starterkit_gql_make_map`, `mapKeys`), which keeps the server's order.
- Persisted non-URL state: `this.$.$mol_state_local.value('key', next)`.

Mistakes: `location.hash = …`, `history.pushState`, `window.onhashchange`; a boolean `opened` state for a page; a hamburger or a breakpoint (the book already scrolls page by page on a narrow screen).

## 10. Forms

```tree
$my_signup $mol_page
	title @ \Sign up
	email_invalid @ \An e-mail address has an @
	body /
		<= Form $mol_form
			body /
				<= Email_field $mol_form_field
					name @ \Email
					bid <= email_bid \
					control <= Email $mol_string
						hint \you@example.org
						value? <=> email? \
			submit_title @ \Create account
			save? <=> save? null
```

```ts
namespace $.$$ {
	export class $my_signup extends $.$my_signup {
		override email_bid() {
			return this.email().includes('@') ? '' : this.email_invalid()
		}

		create(email: string) {
			this.$.$mol_fetch.json('/api/signup', { method: 'POST', body: JSON.stringify({ email }) })
		}

		override save(next?: Event) {
			if (next === undefined) return null

			this.create(this.email())
			this.$.$mol_state_arg.go({ section: 'tasks' })

			return null
		}
	}
}
```

| Control | Binding |
|---|---|
| `$mol_string`, `$mol_textarea` | `value? <=> name? \` |
| `$mol_number` | `value? <=> amount? +NaN` |
| `$mol_check_box` | `checked? <=> agreed? false`, `title` |
| `$mol_switch` | `value? <=> mode? \`, `options *` of id → label |
| `$mol_select` | `value? <=> pick? \`, `dictionary *` of id → label |
| `$mol_button_minor`, `$mol_button_major` | `click? <=> act? null`, `enabled <= …`, `title` |

- The input's value **is** the owner's property (`<=>`). There is no `onChange`, no form state object, no reading the DOM.
- `$mol_form_field`: `name` (label), `bid` (the complaint; empty string means valid), `control`.
- `$mol_form` (`mol/form/form.view.ts`): submit is refused while any field has a `bid`; `save?` is your action; a thrown error becomes the result line beside the button; success shows `message_done`. Ctrl+Enter submits.
- A button whose `click` throws shows the error on itself (`mol/button/button.view.ts`). Do not catch to show a toast.
- Server refusals are data, not exceptions: the mutation returns a union, the screen stores the error in a `@$mol_mem` and feeds each field's `bid` from it (`assets/ui/gql/fielderrors.ts`, `assets/ui/users/show/show.view.ts`).
- Validation is a derived property (`email_bid()`), recomputed as the person types. Never validate inside the handler and set flags.

## 11. Styling and themes

Every view's element carries attributes for its class, each ancestor class and
its place in each owner (measured on `$my_app` above). A keyed child gets the
same names without the key: `Task_row*` is `[my_tasks_task_row]`.

```html
<mol_view id="$my_app.Root(0).Counter().Value()"
	my_counter_value my_app_counter_value mol_view>0</mol_view>
```

`x.view.css` beside the tree, attribute selectors only:

```css
[my_counter] {
	gap: var(--mol_gap_block);
	align-items: center;
}

[my_counter_value] {
	padding: var(--mol_gap_text);
	color: var(--mol_theme_shade);
}
```

Or typed, in `x.view.css.ts` (`mol/style/readme.md`):

```ts
namespace $.$$ {
	const { rem } = $mol_style_unit

	$mol_style_define($my_syntax, {
		gap: $mol_gap.block,
		Head: {
			padding: $mol_gap.text,
			color: $mol_theme.shade,
			minWidth: rem(10),
			':hover': {
				color: $mol_theme.focus,
			},
		},
		'@': {
			my_syntax_kind: {
				loud: {
					font: {
						weight: 'bold',
					},
				},
			},
		},
		'@media': {
			'(max-width: 40rem)': {
				flex: {
					direction: 'column',
				},
			},
		},
	})
}
```

It emits `[my_syntax_head]`, `[my_syntax_head]:hover`,
`[my_syntax]:where([my_syntax_kind="loud"])` and the media block. A child is
addressed by its property name (`Head`), an attribute state by `'@'`.

Rules:

- No `class`, no `id` selectors, no descendant chains (`[a] [b] [c]`). One attribute per rule; the per-owner attribute (`my_app_counter_value`) already says "this one, here".
- A state is an attribute you declare in `attr *` (`my_syntax_kind <= kind \plain`) and select on. Not a class toggled from code.
- Colours and gaps are tokens only: `--mol_theme_back card field hover text shade line focus control current special`, `--mol_gap_page block text round space` (`mol/theme/theme.css`, `mol/gap/gap.css`): `shade` is secondary text, `block` separates blocks, `text` pads text, `space` is the small gap, `round` the radius. In `css.ts`: `$mol_theme.text`, `$mol_gap.block`. A literal colour breaks the dark theme.
- `style *` in a tree is for a value computed per instance (a meter's width). Everything static goes in the stylesheet.
- Every `[mol_view]` is `display: flex; flex-shrink: 0; box-sizing: border-box` (`mol/view/view/view.css`). A view of text children needs no wrapper; a column needs `flex-direction: column`.
- The CSS file is found the same way as code: by the names in it. `[my_counter]` in any stylesheet pulls `my/counter/` into the bundle.

Themes:

| Want | Do |
|---|---|
| Follow the system's light/dark | `plugins / <= Theme $mol_theme_auto` on the root (`assets/ui/app/app.view.tree`). The root then carries `mol_theme="$mol_theme_light"` or `…_dark` |
| A manual switch | `$mol_lights_toggle` anywhere; it drives `$mol_lights()` (`mol/lights/lights.ts`), stored locally |
| An accented region | `theme \$mol_theme_special` (or `_base`, `_accent`, `_current`) on any view; `$mol_button_major` does this |
| Your own palette | redefine the `--mol_theme_*` tokens under `[my_app][mol_theme="$mol_theme_light"]` and `…_dark`; change `--mol_theme_hue` for a tint. Never restyle stock components one by one |

A plugin is a view with no element of its own; it acts on its owner's
(`mol/plugin/plugin.ts`). Use one to add attributes, events or tokens to a
root:

```tree
$my_tone $mol_plugin
	tone \neutral
	attr *
		^
		my_tone <= tone
```

```css
[my_tone="bad"] {
	color: var(--mol_theme_special);
}
```

Attach it in the tree of the component it should act on:
`plugins / <= Tone $my_tone tone \bad`. `$mol_theme_auto`, `$mol_nav`
(keyboard navigation) and `$mol_hotkey` are plugins.

**A plugin acts on the element of the component whose tree declares it**, not
of the child it is listed under. Measured: `<= Syntax $my_syntax plugins /
<= Tone $my_tone` written in `$my_app`'s tree put `my_tone` on the `$my_app`
element and nothing on `Syntax`. To give a child a plugin, declare it in the
child's own tree.

Localisation:

- A string a person reads is `@ \Text` in the tree. English is extracted to `-/web.locale=en.json`; a translation is a file beside the tree, `x.view.tree.locale=ru.json`, keyed as the extraction shows (`"$my_counter_Inc_title": "…"`). The language is `$mol_locale.lang()`.
- A string that `view.ts` needs is still declared in the tree and read as a property: `email_invalid @ \…` → `this.email_invalid()` (§10). No literals for people in `view.ts`.
- There is no interpolation syntax. Compose in `view.ts` from localised pieces.
- **In tests a localised string reads as its key** (`$my_counter_Inc_title`): the test `$` has an empty dictionary (`mol/locale/locale.test.ts`). Compare with the property, `view.email_bid() === view.email_invalid()`, never with the English literal.
- This kit's own screens still declare their strings with a plain `\`; they are not extracted.

## 12. GraphQL, as this kit does it

1. Write one operation per file next to the component: `assets/ui/users/show/ban.graphql`. No operation name; it is derived from the path.
2. `make codegen` writes `ban.graphql.ts` with a typed, synchronous function named by the path: `$starterkit_users_show_ban(variables, opts?)`.
3. Read inside `@$mol_mem`; mutate inside a handler.

```ts
@$mol_mem
user() {
	const user = $starterkit_users_show_data({ id: this.user_id() }).admin.user
	if (!user) throw new Error('User not found')

	return user
}

ban(next?: null) {
	if (next === undefined) return null

	const result = $starterkit_users_show_ban({
		input: { userId: this.user_id(), reason: this.reason().trim() },
	}).admin.banUser

	this.message(result.__typename === 'ErrorPayload' ? result.message : 'Banned')

	return null
}
```

- **Every mutation refetches every query on the page**: queries read a shared marker, a mutation bumps it (`assets/ui/gql/gql.ts`). Do not refetch, do not update a cache, do not pass the new row around. `{ revalidate: false }` opts one call out.
- Select `__typename` on a union and branch on it. A domain refusal is `ErrorPayload`, rendered; only infrastructure failure throws.
- Index rows by id with `$starterkit_gql_make_map(nodes)`; derive row types from the function's return type, never restate them.
- Dates arrive formatted (`createdAt { formatted }`). No date library, no formatting on the client.
- Never call `fetch`, never edit a `*.graphql.ts`, never put two operations in one file.

## 13. Tests

```ts
namespace $.$$ {
	$mol_test({
		'counter stops at three'($) {
			const counter = $my_counter.make({ $ })

			counter.inc(new Event('click'))
			counter.inc(new Event('click'))
			counter.inc(new Event('click'))

			$mol_assert_equal(counter.count_text(), '3')
			$mol_assert_equal(counter.inc_enabled(), false)
		},

		'a stubbed property replaces the request'($) {
			const profile = $my_profile.make({ $, data: () => ({ name: 'Grace' }) })

			$mol_assert_equal(profile.name(), 'Grace')
		},

		async 'an async source reads as a plain value inside a fiber'($) {
			const profile = $my_profile.make({ $ })

			$mol_assert_equal(await $mol_wire_async(profile).name(), 'Ada')
		},
	})
}
```

- A test is `x.test.ts` beside the component. It ships only in the test bundles.
- A component is tested as an object: `make({ $, prop: () => value })` builds one with properties replaced, then call methods and compare. No DOM, no rendering, no selectors.
- Each test gets its own `$`; pass it to `make`. In it `fetch` and `XMLHttpRequest` throw `… is forbidden in tests` (`mol/test/test.test.ts`). Stub the data property instead.
- An `async` test fails after 1000 ms (same file).
- Children are objects too: `page.Form().submit_allowed()`. A member that is not in the tree (`data`) can be replaced through `make` the same way.
- Never assert on an `@` string as a literal (§11): it reads as its key under test.
- Run: `mam <pkg>/<module>` builds and runs `node.test.js`; a failure is a non-zero exit. `-/test.html` runs the same tests in the browser console on every reload.
- What these tests cannot see (layout, the book, a real request, a misspelt name in a list) is checked in a browser: by hand at `-/test.html`, and by this kit's Playwright suite in `e2e/`. The generated attributes are stable selectors: `[starterkit_users_show_ban]`.

## 14. Do not

| Do not | Because | Instead |
|---|---|---|
| `document.querySelector`, `innerHTML`, `classList`, `addEventListener` | the next render overwrites it, and it reads nothing reactive | a property in the tree; `attr *`, `event *`, `field *` |
| `setTimeout`/`setInterval` to wait for data or to poll | a fiber already waits; a clock is a dependency | read the value; `$mol_state_time.now(ms)`; `$mol_wait_timeout(ms)` |
| `await`/`.then` in anything a view reads | breaks the fiber; the value never becomes reactive | synchronous call; one `async` source method behind `$mol_wire_sync` |
| `isLoading`, `error` state set from handlers | suspense and `mol_view_error` are that state | let it throw |
| CSS classes, BEM strings, inline static `style *` | attributes are generated; inline style cannot be themed | attribute selectors in `x.view.css` |
| Literal colours, pixel gaps | breaks themes and rhythm | `--mol_theme_*`, `--mol_gap_*` |
| `import`, npm UI packages, a bundler config, React/Vue/JSX for a screen | there is no module system to import through | an FQN; a stock `$mol_*` component |
| A store, a context, an event bus, manual subscriptions | `@$mol_mem` on the owner is the store | a memoised method; pass it down with `<=` |
| A router, `history`, `location.hash` | the URL is `$mol_state_arg` | `$mol_link arg *`, `.value()`, `.go()` |
| `new $my_row()` in a method | a new instance per render | a `Row*` in the tree, or a `@$mol_mem`/`@$mol_mem_key` factory |
| Guessing a stock component's properties | names differ from other frameworks (`checked`, not `value`; `rows`, not `items`) | read `mol/<name>/<name>.view.tree` first; its `demo/` shows usage |
| Editing anything under `-/`, `-view.tree/`, `-css/`, or a `*.graphql.ts` | generated | edit the source beside it |

## 15. First-try checklist

1. Directory = name: `a/b/c/c.view.tree` declares `$a_b_c`, and nothing is imported.
2. Tabs only in `.view.tree`; strings start with `\`; person-facing strings are `@ \…`.
3. Every `<=` name under a `/` was re-read against its declaration (lists are untyped); `attr *`, `event *`, `field *` start with `^`.
4. Before using a stock component, its `mol/<name>/<name>.view.tree` was read for the real property names.
5. `view.ts` is `namespace $.$$ { export class $x extends $.$x {…} }`, with `override` on every member the tree declared.
6. Repeated things are `Name*` in the tree plus a list of `this.Name(id)` in `view.ts`; rows set `minimal_height`.
7. Requests and built lists are `@$mol_mem`; per-key ones `@$mol_mem_key`; nothing subscribes, nothing awaits.
8. No loading flags and no `try/catch` around data; a `catch` that exists rethrows promises.
9. Code before a request in a handler is idempotent; a side effect that must run once is an `@$mol_action`.
10. What is open is in the URL: `$mol_link arg *` or `$mol_state_arg`, never component state.
11. Mutations go through the generated function; nothing refetches by hand; refusals are read from `__typename`.
12. Styles are attribute selectors on generated names, tokens for every colour and gap, nothing static inline.
13. A component with logic has an `x.test.ts` using `make({ $ })`, and no assertion compares an `@` string with a literal.
14. `make ui` exits zero: types, then tests. Then `grep -c` the new symbol in `-/web.js`.
15. The screen was opened in a browser at `-/test.html`, with the console read, before it is called done.
