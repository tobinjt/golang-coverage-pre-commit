# golang-coverage-pre-commit

A tool to check that Golang code has sufficient test coverage. This tool can be
used via <https://pre-commit.com> or standalone by running
`golang-coverage-pre-commit`.

Coverage will be generated by running:

```shell
go test --coverprofile="${filename}" --covermode=set
go tool cover --func="${filename}"
```

The output from the second command will be parsed to check whether it meets the
coverage requirements you define (see [Configuration](#configuration) below),
and an error message will be output for any functions not meeting your
requirements.

## Configuration

A YAML config file named `.golang-coverage-pre-commit.yaml` is **_required_**.
None of the fields are required inside the file; an empty config is equivalent
to a config containing only `default_coverage: 0`. You can generate an example
config (shown below) by running `golang-coverage-pre-commit --example_config`.

### Example config

```yaml
comment:
  Comment is not interpreted or used; it is provided as a structured way of
  adding comments to a config, so that automated editing is easier.
default_coverage: 80
functions:
  - comment: Low coverage is acceptable for main()
    regex: ^main$
    coverage: 50
  - comment:
      All the fooOrDie() functions should be fully tested because they panic()
      on failure
    regex: ^.*OrDie$
    coverage: 100
filenames:
  - comment: "TODO: improve test coverage for parse_json.go"
    regex: ^parse_json.go$
    coverage: 73
  - comment: Full coverage for other parsers
    regex: ^parse.*.go$
    coverage: 100
```

### Fields in the config file

**_Top-level fields_**

- `comment`: unused by `golang-coverage-pre-commit`, it exists to support
  structured comments that survive de-serialisation and re-serialisation, e.g.
  when combining config snippets.
- `default_coverage`: this is the default required coverage level that is used
  when a coverage line is not matched by a more specific rule (see [Order of
  evaluation](#order-of-evaluation) below).
- `functions`: a list of rules matching against function names.
- `filenames`: a list of rules matching against filenames.

**_functions and filenames fields_**

The `functions` and `filenames` rules have exactly the same fields.

- `comment`: unused by `golang-coverage-pre-commit`, it exists to support
  structured comments that survive de-serialisation and re-serialisation, e.g.
  when combining config snippets.
- `regex`: the regular expression that the function name or filename is matched
  against.
- `coverage`: the required coverage level for lines matched by this rule.

### Order of evaluation

Each line of coverage output (effectively, each function in your code) is
independently evaluated:

- The function name is matched against each `function` regex, in the order
  they're listed in the config, with the first match winning.
- If the function name wasn't matched, the filename (with the line number and
  the module name from `go.mod` removed) is matched against each `filename`
  regex, in the order they're listed in the config, with the first match
  winning. Note that filenames use regex matching, _not_ globs.
- If neither the function name or the filename was matched, `default_coverage`
  is used.

The coverage in the line is compared to the coverage required by the matching
rule, and if the coverage is lower than the requirement an error will be output.

### Bootstrapping a config

You can easily bootstrap a config that requires high coverage for new code and
the current coverage level for existing code to prevent a reduction in coverage.

```shell
default_coverage=100  # Change if you'd prefer to set the bar lower.
tmpfile="$(mktemp -t .golang-coverage-pre-commit.yaml.XXXXXXXX)"
echo "default_coverage: ${default_coverage}" > .golang-coverage-pre-commit.yaml

cat > "${tmpfile}" <<END_OF_HEADER
comment:
    Bootstrapped config to maintain current coverage and require high coverage
    for new functions.
default_coverage: ${default_coverage}
END_OF_HEADER

(golang-coverage-pre-commit 2>&1) \
  | awk -F '\t' '{
        print "  - regex: ^" $2 "$";
        sub("%.*", "", $3);
        print "    coverage: " $3}' \
  >> "${tmpfile}"
mv "${tmpfile}" .golang-coverage-pre-commit.yaml
```

## FAQ

**How can I tell which lines of code have not been tested?**

Run `golang-coverage-pre-commit --browser` - it will open the coverage report in
your browser. If you're developing remotely this will not work, but you can
find the coverage report in `${TMPDIR}/cover<RANDOM>/coverage.html` so maybe you
can copy it to your local machine and open it from there?

**How can I debug rule matching?**

Run `golang-coverage-pre-commit --debug_matching` - each coverage line and the
rule that matches it will be printed. That tells you that the earlier rules in
your config did not match that line and the later rules in your config were not
reached.

**How can I pass different arguments to `go test`?**

You can't, they're hard-coded. If you need this please open an issue so we can
discuss it.

**Can I include one config in another?**

There's no facility for this, but hopefully it's relatively easy to write some
code to merge two config files. This is why comments are structured data rather
than `//` comments.

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for details.

## License

Apache 2.0; see [`LICENSE`](LICENSE) for details.

## Disclaimer

This project is not an official Google project. It is not supported by
Google and Google specifically disclaims all warranties as to its quality,
merchantability, or fitness for a particular purpose.
