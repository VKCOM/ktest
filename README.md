![](docs/readme_header.png)

## Overview

`ktest` is a tool that makes [kphp](https://github.com/VKCOM/kphp/) programs easier to test.

* `ktest phpunit` can run [PHPUnit](https://github.com/sebastianbergmann/phpunit) tests using KPHP
* `ktest bench` run benchmarks using KPHP
* `ktest bench-php` run benchmarks using PHP
* `ktest bench-vs-php` run benchmarks using both KPHP and PHP, compare the results
* `ktest benchstat` compute and compare statistics about benchmark results (see [benchstat](https://godoc.org/golang.org/x/perf/cmd/benchstat))
* `ktest env` print ktest-related env variables

## Installation

Download the `ktest` binary from the [latest release](https://github.com/VKCOM/ktest/releases) or build it from sources.

You may need to set `KPHP_ROOT` environment variable to point to your [KPHP repository](https://github.com/VKCOM/kphp/) folder.

## Example - phpunit

Imagine that we have an ordinary `PHPUnit` test:

```php
<?php

use PHPUnit\Framework\TestCase;
use ExampleLib\Strings;

class StringsTest extends TestCase {
    public function testContains() {
        $this->assertSame(Strings::contains('foo', 'bar'), false);
        $this->assertTrue(Strings::contains('foo', 'foo'));
    }

    public function testHasPrefix() {
        $this->assertSame(Strings::hasPrefix('Hello World', 'Hello'), true);
        $this->assertFalse(Strings::hasPrefix('Hello World', 'ello'));
    }
}
```

It comes without a surprise that you can run it with `phpunit` tool:

```bash
$ ./vendor/bin/phpunit tests

......                                                              6 / 6 (100%)

Time: 70 ms, Memory: 4.00 MB

OK (6 tests, 14 assertions)
```

When you're using `phpunit`, tests are executed as PHP, not KPHP.

`ktest` makes it possible to run your phpunit-compatible tests with KPHP:

```bash
$ ktest phpunit tests

.... 4 / 6 (66%) OK
.. 6 / 6 (100%) OK

Time: 10.74657386s

OK (6 tests, 14 assertions)
```

> Note that running KPHP tests is slower: a separate binary is compiled per every Test class.

All you need is `ktest` utility and installed [kphpunit](https://github.com/VKCOM/kphpunit) package:

```bash
$ composer require --dev vkcom/kphpunit
```

Now let's do something more exciting.

Take a look at this `Integers::getFirst` method:

```php
<?php

namespace Foo\Bar;

class Integers {
    /** @param int[] $xs */
    public static function getFirst(array $xs) {
        return $xs[0];
    }
}
```

It's intended to return the first int array item, or `null`, if index 0 is unset.

We can write a test for this method:

```php
<?php

use PHPUnit\Framework\TestCase;
use Foo\Bar\Integers;

class IntegersTest extends TestCase {
    public function testGetFirst() {
        $this->assertSame(Integers::getFirst([]), null);
        $this->assertSame(Integers::getFirst([1]), 1);
    }
}
```

All tests are passing:

```
.                                                                   1 / 1 (100%)

Time: 36 ms, Memory: 4.00 MB

OK (1 test, 2 assertions)
```

But if you try to run it with `ktest`, you'll see how that code would behave in KPHP:

```
F 1 / 1 (100%) FAIL

Time: 4.59874429s

There was 1 failure:

1) IntegersTest::testGetFirst
Failed asserting that null is identical to 0.

IntegersTest.php:8

FAILURES!
Tests: 1, Assertions: 1, Failures: 1.
```

Accessing unset array index can yield a "zero value" instead of null.

Running with `ktest` makes it easier to ensure that your code behaves identically in both PHP and KPHP.

## Example - bench

There are 2 main ways to do benchmarking with `bench` subcommand:

1. Run different benchmarks and see how they relate
2. Run benchmarks by the same name and compare samples with [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat?utm_source=godoc)

Let's assume that you have a function that concatenates 3 strings. You can write a benchmark for it:

```php
<?php

// file "BenchmarkConcat3.php"

class BenchmarkConcat3 {
    private static $strings = [
        'foo',
        'bar',
        'baz',
    ];

    public function benchmarkConcat() {
        return self::$strings[0] . self::$strings[1] . self::$strings[2];
    }
}
```

This benchmark can be executed with a `bench` subcommand:

```bash
$ ktest bench BenchmarkConcat3.php
class BenchmarkConcat3
BenchmarkConcat3::benchmarkConcat	106500	372.0 ns/op
ok BenchmarkConcat3 147.153797ms
```

Suppose that somebody proposed to re-write this function with `ob_start()` claiming that it would make things faster.

First, we need to collect samples of the current implementation. We need at least 5 rounds, but usually the more - the better (don't get too crazy though, 10 is good enough in most cases).

```bash
$ ktest bench -count 5 Concat3Benchmark.php | tee old.txt
```

Now we have old implementation results, it's time to roll the a implementation:

```php
<?php

class Concat3Benchmark {
    private static $strings = [
        'foo',
        'bar',
        'baz',
    ];

    public function benchmarkConcat() {
        ob_start();
        echo self::$strings[0];
        echo self::$strings[1];
        echo self::$strings[2];
        return ob_get_clean();
    }
}
```

Now we need to collect the new implementation results:

```bash
$ ktest bench -count 5 Concat3Benchmark.php | tee new.txt
```

When you have 2 sets of samples, it's possible to compare them with benchstat:

```
$ ktest benchstat old.txt new.txt
name    old time/op  new time/op  delta
Concat   372ns ± 2%   546ns ± 6%  +46.91%  (p=0.008 n=5+5)
```

As we can see, the new implementation is, in fact, almost 2 times slower!

## TODO

* Mocks

## Limitations

* Assert functions can't be used for objects (class instances)
* No custom comparators for assert functions
