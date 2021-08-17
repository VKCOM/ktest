<?php

use PHPUnit\Framework\TestCase;
use Foo\Bar\Integers;

class IntegersTest extends TestCase {
    public function testGetFirst() {
        $this->assertSame(Integers::getFirst([]), null);
        $this->assertSame(Integers::getFirst([1]), 1);
    }
}
