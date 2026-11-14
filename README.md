# cron-utils

![cron-utils - cron in, English out](assets/hero.png)

**Your agent emits `0 9 * * 1-5`. Nobody on your team can read it.**

Agents, schedulers and infra-as-code emit cron expressions; humans get handed strings like `30 2 */3 * 1-5` and guess. The English descriptors for Go (lnquy/cron, jsuar/go-cron-descriptor) were last updated in 2020, and lnquy/cron has an [open issue](https://github.com/lnquy/cron/issues) reporting a panic on invalid input. For occurrence computation, gorhill/cronexpr is archived. Feeding LLM output into a parser that panics is not a parse error, it's an outage.

cron-utils validates, explains and previews cron schedules in one import. Bad input gets a typed error that says which field is wrong and why, so an agent can feed it straight back to the model.

```go
s, err := cronutils.Parse("0 9 * * FRI#3")
if err != nil {
    // *cronutils.ParseError: field, offending text, plain message
    return err
}
fmt.Println(s.Describe())
// At 09:00 AM, on the third Friday of the month

from := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
fmt.Println(s.NextN(from, 3))
// [2026-11-20 09:00:00 +0000 UTC 2026-12-18 09:00:00 +0000 UTC 2027-01-15 09:00:00 +0000 UTC]
```

There's also a small CLI for humans and demos:

```console
$ cron-utils "0 9 * * 1-5"
At 09:00 AM, Monday through Friday
```

## Install

Library:

```sh
go get github.com/ofershap/cron-utils
```

CLI:

```sh
go install github.com/ofershap/cron-utils/cmd/cron-utils@latest
```

Standard library only at runtime. (Test-only dependencies compare fire times against gorhill/cronexpr and robfig/cron in CI; they are not imported by the library.)

## What it does

- **Describes cron in English**, cronstrue-style: `0 9 * * 1-5` becomes "At 09:00 AM, Monday through Friday".
- **Validates with typed errors**: `ParseError` names the field, quotes the offending text and says what's wrong. Parse and Describe are designed never to panic; observed evidence: `go fuzz` ran 1.7M generated inputs with zero panics, plus a hostile-input table test. That's test evidence, not a proof.
- **Previews fire times**: `Next`, `Prev` and `NextN` compute occurrences in the caller's location, with day-skipping instead of minute-by-minute iteration. Measured: `0 0 29 2 *` from three years out resolves in about 34µs (BenchmarkNextFeb29, 2.6GHz Xeon; see bench_test.go).
- **Speaks the dialects agents emit**: 5-field Vixie, 6-field with leading seconds, `@daily` and friends, month and weekday names, `?`, and the Quartz specials `L`, `LW`, `15W`, `FRIL`, `FRI#3`.
- **Vixie-correct**: when both day-of-month and day-of-week are restricted, a day matching either fires, the way cron itself behaves. A day field counts as restricted unless it's `*`, `?` or a `*` with step 1 - the same rule as robfig/cron.
- **Differentially tested, per engine** (differential_test.go): `Next` matches robfig/cron v3 on 300 generated standard 5-field schedules and gorhill/cronexpr on 300 generated 6-field schedules including the Quartz L/LW/nW/xL/x#n specials - each over leap dates, impossible schedules (Feb 31) and strict Next boundaries. `Prev` and the Sunday-as-7 dialect are covered by pinned golden tests, not by the engines. Where cronexpr and robfig disagree with each other (a `*/1` day field beside a restricted one), cron-utils follows robfig/cron and the differential scope excludes the case.

## What it doesn't do

- **Not a scheduler.** It computes fire times; it doesn't run your jobs. For running jobs see robfig/cron.
- **No `@reboot`.** It fires at daemon startup, not on a schedule; Parse rejects it with a clear error.
- **English descriptions only**, for now. The descriptor is one function, so locales are a possible later addition.
- **7-field Quartz years are rejected** with a clear error rather than silently mis-parsed.
- **Bounded horizon**: `Next` and `Prev` search six years out and return `ok=false` beyond that; `MustNext` panics in that case (the one intentional panic in the package).
- **Day-of-week numbering**: 0-6 and 7 are all accepted, 0 and 7 both Sunday (Vixie style); `SUN`-`SAT` names are case-insensitive.
- **DST policy is pinned, not clever**: ordinary wall-clock times never drift across transitions. A schedule whose wall time doesn't exist (spring-forward gap) fires at the first existing wall-clock time at or after it; a schedule in the fall fold fires once, at its first occurrence. This is a chosen dialect policy, not universal Vixie compatibility - other crons skip gap times or delay by the offset. Pinned, including a half-hour zone, in dst_test.go.

## Errors an agent can use

```go
_, err := cronutils.Parse("0 25 * * *")
// cron: hours field "25": 25 is outside 0-23
```

The error tells the model exactly what to fix. Contrast with a panic, which tells it nothing.

<details><summary><b>How do I convert a cron expression to human-readable text in Go?</b></summary>
<code>cronutils.Describe("0 9 * * 1-5")</code> returns "At 09:00 AM, Monday through Friday". It handles 5-field and 6-field (seconds) expressions, @macros, month and weekday names, and the Quartz specials L, W and #.</details>

<details><summary><b>How do I compute the next run time of a cron expression in Go?</b></summary>
Parse the expression with <code>cronutils.Parse</code>, then call <code>Next(t)</code>, <code>Prev(t)</code> or <code>NextN(t, n)</code>. Times are computed in the input time's location, and sparse schedules like <code>0 0 29 2 *</code> (Feb 29) resolve in tens of microseconds via day-skipping (see bench_test.go).</details>

<details><summary><b>Is cron-utils safe to run on LLM-generated cron expressions?</b></summary>
Yes. Malformed input returns a typed <code>*ParseError</code> naming the field and problem, designed to be fed back to a model. Parse and Describe are designed never to panic; observed evidence is a 1.7M-execution <code>go fuzz</code> run with zero panics plus a hostile-input table test, not a proof.</details>

<details><summary><b>What's the difference between cron-utils and lnquy/cron or gorhill/cronexpr?</b></summary>
Those libraries stalled in 2020; lnquy/cron has an open issue reporting a panic on invalid input and gorhill/cronexpr is archived. cron-utils combines description and occurrence computation in one package (stdlib-only at runtime), supports Quartz specials, and is differentially tested against gorhill/cronexpr and robfig/cron v3 for fire-time correctness.</details>

## Contributing

Issues and PRs welcome - see [CONTRIBUTING.md](CONTRIBUTING.md). Please report security issues privately per [SECURITY.md](SECURITY.md).

---

More projects: [gitshow](https://github.com/ofershap/gitshow) · [real-browser-mcp](https://github.com/ofershap/real-browser-mcp) · [cursor-usage-tracker](https://github.com/ofershap/cursor-usage-tracker) · [mcp-server-markdown](https://github.com/ofershap/mcp-server-markdown) · [readme-builder](https://github.com/ofershap/readme-builder) · [grok-bot-skills](https://github.com/ofershap/grok-bot-skills) · [mashov-parent-brief](https://github.com/ofershap/mashov-parent-brief) · [gailwind](https://github.com/ofershap/gailwind) · [lather](https://github.com/ofershap/lather) · [json5](https://github.com/ofershap/json5) · [poppy-go](https://github.com/ofershap/poppy-go)

---

## Author

[![Made by ofershap](https://gitshow.dev/api/card/ofershap)](https://gitshow.dev/ofershap)

[![LinkedIn](https://img.shields.io/badge/LinkedIn-Connect-0A66C2?style=flat&logo=linkedin&logoColor=white)](https://linkedin.com/in/ofershap)
[![GitHub](https://img.shields.io/badge/GitHub-Follow-181717?style=flat&logo=github&logoColor=white)](https://github.com/ofershap)
[![Blog](https://img.shields.io/badge/Blog-ofershap.github.io-FF5722?style=flat&logo=rss&logoColor=white)](https://ofershap.github.io/)

---

<sub>README built with [README Builder](https://ofershap.github.io/readme-builder/)</sub>

## License

[MIT](LICENSE) &copy; [Ofer Shapira](https://github.com/ofershap)
