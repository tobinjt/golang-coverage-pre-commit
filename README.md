# golang-coverage-pre-commit

Pre-commit check that Golang code has sufficient test coverage.

Coverage will be generated by running:

```shell
go test --coverprofile="${filename}" --covermode=set
go tool cover --func="${filename}"
```

The output from the second command will be parsed to check whether it meets the
coverage requirements you define (see Configuration below).

## Configuration

A YAML config file named `.golang-coverage-pre-commit.yaml` is _required_; if
this file doesn't exist an example config (see below) will be printed for you to
save and modify.

None of the fields are required; an empty config is equivalent to a config
containing only `default: 0`.

### Example config

```yaml
comment:
  Comment is not interpreted or used; it is provided as a structured way of
  adding comments to a config, so that automated editing is easier.
default: 80
functions:
  - comment: Low coverage is acceptable for main()
    regex: ^main$
    coverage: 50
  - comment:
      All the fooOrDie() functions should be fully tested because they panic()
      on failure
    regex: ^.*OrDie$
    coverage: 50
filenames:
  - comment: "TODO: improve test coverage for parse_json.go"
    regex: ^parse_json.go$
    coverage: 73
  - comment: Full coverage for other parsers
    regex: ^parse.*.go$
    coverage: 100
```

### Order of evaluation

Each line of coverage output is independently evaluated:

- The function name is matched against each `function` regex, in the order
  they're listed in the config, with the first match winning.
- If the function name wasn't matched, the filename (with the line number and
  the module name from `go.mod` removed) is matched against each `filename`
  regex, in the order they're listed in the config, with the first match
  winning. Note that filenames use regex matching, _not_ globs.
- If neither the function name or the filename was matched, `default` is used.

## FAQ

**How can I pass different arguments to `go test`?**

You can't, they're hard-coded. If you need this please open an issue so we can
discuss it.

**Can I include one config in another?**

There's no facility for this, but hopefully it's relatively easy to write some
code to merge two config files. This is why comments are structured data rather
than `//` comments.
