# RegreSQL, Regression Testing your SQL queries

The `regresql` tool implements a regression testing facility for SQL queries,
and supports the PostgreSQL database system. A regression test allows to
ensure known results when the code is edited. To enable that we need:

  - some code to test, here SQL queries, each in its own file,
  - a known result set for each SQL query,
  - a regression driver that runs queries again and checks their result
    against the known expected result set.
    
The RegreSQL tool is that regression driver. It helps with creating the
expected result set for each query and then running query files again to
check that the results are still the same.

Of course, for the results to be comparable the queries need to be run
against a known PostgreSQL database content.

## Installing

The `regresql` tool is written in Go, so:

    go install github.com/dimitri/regresql
    
This command will compile and install the command in your `$GOPATH/bin`,
which defaults to `~/go/bin`. See <https://golang.org/doc/install> if you're
new to the Go language.

## Basic usage

Basic usage of regresql:

  - `regresql init [ -C dir ]`
  
    Creates the regresql main directories and runs all SQL queries found in
    your target code base (defaults to current directory).
    
    The -C option changes current directory to *dir* before running the
    command.
  
  - `regresql plan [ -C dir ]`
  
    Create query plan files for all queries. Run that command when you add
    new queries to your repository.
  
  - `regresql update [ -C dir ] [ --versioned-all | file ... ]`
  
    Updates the *expected* files from the queries, considering that the
    output is valid.
    
    When SQL file paths are given as arguments, those files produce
    version-specific expected output (e.g. `query.pg16.out`) while all
    other files are updated normally.  Use `--versioned-all` to write
    version-specific files for every query in the suite.  The two forms
    are mutually exclusive.
  
  - `regresql test [ -C dir ]`
  
    Runs all the SQL queries found in current directory.
    
    The -C option changes the current directory before running the tests.
    
  - `regresql list [ -C dir ]`
  
    List all SQL files found in current directory.

    The -C option changes the current directory before listing the files.

## SQL query files

RegreSQL finds every *.sql* file in your code repository and runs them
against PostgreSQL. It means you're supposed to maintain your queries as
separate query files, see the
excellent <https://github.com/krisajenkins/yesql> Clojure library to see how
that's done. The project links to many implementation in other languages,
including Python, PHP or Go.

SQL files might contain variables, and RegreSQL implements the same support
for them as `psql`, see the PostgreSQL documentation
about
[psql variables](https://www.postgresql.org/docs/current/static/app-psql.html#APP-PSQL-VARIABLES) and
their usage syntax and quoting rules: `:foo`, `:'foo'` and `:"foo"`.

## Default variable values with `\set`

SQL files may contain `\set` metacommands to supply default values for query
parameters, exactly as you would type them in an interactive `psql` session:

```sql
-- genre-topn.sql
\set n 10
SELECT name
  FROM genre
 ORDER BY name
 LIMIT :n;
```

RegreSQL strips `\set` lines before sending the query to PostgreSQL and stores
their values as defaults.  When a parameter is resolved at test time the lookup
order is:

1. The YAML plan binding for the current test case (explicit value wins).
2. The `\set` default from the SQL file.
3. Error — the parameter has no value.

**No plan file needed** when every query variable has a `\set` default.
RegreSQL synthesises a single test case automatically, so you can run
`regresql update` and `regresql test` straight away without touching any YAML.

**Running `regresql plan`** on a file that uses `\set` pre-fills the generated
YAML with those default values, so the plan is immediately runnable.  You can
then add extra test cases or override individual values by editing the YAML.

### Token parsing

Value tokens follow psql quoting rules and are concatenated without any
separator:

| Token form | Stored value |
|---|---|
| `'text'` | outer quotes stripped; psql escapes expanded (`''`→`'`, `\n`→newline, `\t`→tab, `\NNN`→octal byte, `\xHH`→hex byte, `\.`→literal char) |
| `"text"` | kept verbatim including the surrounding double-quotes |
| `word` | taken as-is |
| *(nothing)* | empty string |

Example: `\set x a 'b' c` stores `abc` in `x`.

## Test Suites

By default a Test Suite is a source directory.

## File organisation

RegreSQL needs the following files and directories to run:

  - `./regresql` where to register needed files
  
  - `./regresql/regress.yaml`
  
    Configuration file for the directory in which it's installed. It
    contains the PostgreSQL connection string where to connect to for
    running the regression tests and the top level directory where to find
    the SQL files to test against.
    
    Two optional fields further control which files are processed:
    
    - `root` — restrict SQL file discovery to a subdirectory, e.g. `src/sql`.
    - `exclude` — a list of glob patterns (relative to the project root) for
      SQL files that should be ignored entirely by all commands.
    
    Example:
    
    ```yaml
    pguri: postgres:///mydb?sslmode=disable
    root: src/sql
    exclude:
      - src/sql/version.sql
      - src/sql/temp-*.sql
    ```
  
  - `./regresql/plans/path/to/query.yaml`
  
    For each file *query.sql* found in your source tree, RegreSQL creates a
    subpath in `./regresql/plans` with a *query.yaml* file. This YAML file
    contains query plans: that's a list of SQL parameters values to use when
    testing.
  
  - `./regresql/expected/path/to/query.out`
  
    For each file *query.sql* found in your source tree, RegreSQL creates a
    subpath in `./regresql/expected` directory and stores in *query.out* the
    expected result set of the query.
    
    When a query's output differs across PostgreSQL major versions (e.g.
    `SELECT version()`), version-specific expected files can be placed
    alongside the generic one:
    
    ```
    regresql/expected/src/sql/version.out        # generic fallback
    regresql/expected/src/sql/version.pg14.out   # PostgreSQL 14.x
    regresql/expected/src/sql/version.pg16.out   # PostgreSQL 16.x
    ```
    
    During `regresql test` the version-specific file is used when it exists;
    otherwise the generic file is used as a fallback.
    
  - `./regresql/out/path/to/query.sql`
  
    The result of running the query in *query.sql* is stored in *query.out*
    in the `regresql/out` directory subpath for it, so that it is possible
    to compare this result to the expected one in `regresql/expected`.
    
## Excluding queries

Some SQL files in a project may not be suitable for regression testing (data
seed scripts, migration helpers, etc.).  Add an `exclude` list to
`regresql/regress.yaml` to skip them entirely — they will be invisible to
`list`, `plan`, `update`, and `test`:

```yaml
exclude:
  - src/sql/seed-data.sql
  - src/sql/migrations/*.sql
```

Patterns follow Go's [`filepath.Match`](https://pkg.go.dev/path/filepath#Match)
glob syntax (single `*` wildcard).  Paths are relative to the project root.

## Version-specific expected files

Queries whose output changes between PostgreSQL major versions — such as
`SELECT version();` — can have separate expected files for each version.
The file naming convention inserts `.pg{major}` before `.out`:

| Query type | Generic | Version-specific |
|---|---|---|
| No parameters | `query.out` | `query.pg16.out` |
| With parameters | `query.top-3.out` | `query.top-3.pg16.out` |

During `regresql test` the version-specific file is used when it exists;
otherwise the generic file is the fallback.

To create version-specific expected files, run `regresql update` against each
PostgreSQL version you want to support, naming the files explicitly:

```bash
# update only version.sql with a version-specific expected file
regresql update src/sql/version.sql

# update every query in the suite with version-specific expected files
regresql update --versioned-all
```

Plain `regresql update` (no arguments, no flag) continues to write generic
`.out` files as before, so existing workflows are unaffected.

## Example

In a small local application the command `regresql list` returns the
following SQL source files:

```
$ regresql list
.
  src/sql/
    album-by-artist.sql
    album-tracks.sql
    artist.sql
    genre-topn.sql
    genre-tracks.sql
```

After having done the following commands:

```
$ regresql init postgres:///chinook?sslmode=disable
...

$ regresql update
...
```

Now we have to edit the YAML plan files to add bindings, here's an example
for a query using a single parameter, `:name`:

```
$ cat src/sql/album-by-artist.sql
-- name: list-albums-by-artist
-- List the album titles and duration of a given artist
  select album.title as album,
         sum(milliseconds) * interval '1 ms' as duration
    from album
         join artist using(artistid)
         left join track using(albumid)
   where artist.name = :name
group by album
order by album;

$ cat regresql/plans/src/sql/album-by-artist.yaml 
"1":
  name: "Red Hot Chili Peppers"
```

And we can now run the tests:

```
$ regresql test
Connecting to 'postgres:///chinook?sslmode=disable'… ✓
TAP version 13
ok 1 - src/sql/album-by-artist.1.out
ok 2 - src/sql/album-tracks.1.out
ok 3 - src/sql/artist.1.out
ok 4 - src/sql/genre-topn.top-3.out
ok 5 - src/sql/genre-topn.top-1.out
ok 6 - src/sql/genre-tracks.out
```

We can see the following files have been created by the RegreSQL tool: 

```
$ tree regresql/
regresql/
├── expected
│   └── src
│       └── sql
│           ├── album-by-artist.1.out
│           ├── album-tracks.1.out
│           ├── artist.1.out
│           ├── genre-topn.1.out
│           ├── genre-topn.top-1.out
│           ├── genre-topn.top-3.out
│           └── genre-tracks.out
├── out
│   └── src
│       └── sql
│           ├── album-by-artist.1.out
│           ├── album-tracks.1.out
│           ├── artist.1.out
│           ├── genre-topn.1.out
│           ├── genre-topn.top\ 1.out
│           ├── genre-topn.top\ 3.out
│           ├── genre-topn.top-1.out
│           ├── genre-topn.top-3.out
│           └── genre-tracks.out
├── plans
│   └── src
│       └── sql
│           ├── album-by-artist.yaml
│           ├── album-tracks.yaml
│           ├── artist.yaml
│           └── genre-topn.yaml
└── regress.yaml

9 directories, 21 files
```

## History

This tool is inspired by the PostgreSQL regression testing framework. It's
been written in the process of
the [Mastering PostgreSQL](http://masteringpostgresql.com/) book as an
example of an SQL framework for unit testing and regression testing.

## License

The RegreSQL utility is released
under [The PostgreSQL License](https://www.postgresql.org/about/licence/).
