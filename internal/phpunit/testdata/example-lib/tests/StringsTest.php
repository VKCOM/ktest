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

    public function testJoin() {
        $this->assertEquals(Strings::join(['a', 'b'], ''), 'ab');
        $this->assertEquals(Strings::join(['a', 'b'], '-'), 'a-b');
        $this->assertEquals(Strings::join(['a'], ''), 'a');
        $this->assertEquals(Strings::join(['a'], '-'), 'a');
        $this->assertEquals(Strings::join([], ''), '');
    }
}
