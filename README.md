# RegreSQL, Regression Testing your SQL queries

The `regresql` tool implement a regression testing facility for SQL queries,
and supports the PostgreSQL database system. A regression test allows to
ensure known results when the code is edited. To enable that we need:

  - some code to test, here SQL queries, each in its own file,
  - a known result set for each SQL query,
  - a regression driver that runs queries again and check their result
    against the known expected result set.
    
The RegreSQL tool is that regression driver. It helps with creating the
expected result set for each query and then running query files again to
check that the results are still the same.

Of course, for the results the be comparable the queries need to be run
against a known PostgreSQL database content.

## Basic usage

Basic usage or regresql:

  - `regresql init [ -C dir ]`
  
    Creates the regresql main directories and runs all SQL queries found in
    your target code base (defaults to current directory).
    
    The -C option changes current directory to *dir* before running the
    command.
  
  - `regresql update [ -C dir ]`
  
    Updates the *expected* files from the queries, considering that the
    output is valid.
  
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

## Test Suites

By default a Test Suite is a source directory.

## File organisation

RegreSQL needs the following files and directories to run:

  - `./regresql` where to register needed files
  
  - `./regresql/pg`
  
    The PostgreSQL connection string where to connect to for running the
    regression tests.
    
    TODO: support several connection strings for different parts of an
    application, with subpath. (`regresql/pg/path/to/pg`)
  
  - `./regresql/expected/path/to/query.out`
  
    For each file *query.sql* found in your source tree, RegreSQL creates a
    subpath in `regresql/expected` directory and stores in *query.out* the
    expected result set of the query,
    
  - `./regresql/out/path/to/query.out`
  
    The result of running the query in *query.sql* is stored in *query.out*
    in the `regresql/out` directory subpath for it, so that it is possible
    to compare this result to the expected one in `regresql/expected`.

## History

This tool is inspired by the PostgreSQL regression testing framework. It's
been written in the process of
the [Mastering PostgreSQL](http://masteringpostgresql.com/) book as an
example of an SQL framework for unit testing and regression testing.

## License

The RegreSQL utility is released
under [The PostgreSQL License](https://www.postgresql.org/about/licence/).
