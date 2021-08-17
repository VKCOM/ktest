<?php

namespace ExampleLib;

class Strings {
    public static function join($parts, $sep): string {
        return implode($sep, $parts);
    }

    public static function contains($s, $substr): bool {
        return '' === $substr || false !== strpos($s, $substr);
    }

    public static function hasPrefix(string $s, string $prefix): bool {
        return 0 === strncmp($s, $prefix, strlen($prefix));
    }
}
