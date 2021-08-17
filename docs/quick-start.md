## Quick start guide

1. Install [kphp](https://vkcom.github.io/kphp/kphp-basics/installation.html)

`ktest` needs to find your kphp2cpp binary; you have several options here:

* Put `kphp2cpp` somewhere under your `$PATH`
* Set `$KPHP_ROOT` env variable to the `kphp` git repository folder (you'll need to compile it)
* If `kphp` is cloned in `~/kphp`, you don't need to set `$KPHP_ROOT`

2. Write some [PHPUnit tests](https://phpunit.readthedocs.io/en/9.5/writing-tests-for-phpunit.html)

3. Run tests with `ktest phpunit testsdir`

TODO: improve this documentation.
