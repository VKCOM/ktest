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
