<?php

class ConcatBenchmark {
    private static $strings = [
        'foo',
        'bar',
        'baz',
    ];

    public function benchmarkConcat3() {
        return self::$strings[0] . self::$strings[1] . self::$strings[2];
    }

    public function benchmarkImplode() {
        return implode('', self::$strings);
    }

    public function benchmarkOutputBuffer() {
        ob_start();
        echo self::$strings[0];
        echo self::$strings[1];
        echo self::$strings[2];
        return ob_get_clean();
    }
}
