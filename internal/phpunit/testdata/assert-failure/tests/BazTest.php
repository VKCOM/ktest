<?php

use PHPUnit\Framework\TestCase;
use Foo\Bar\Baz;

class BazTest extends TestCase {
    public function testEquals1() {
        $baz = new Baz();
        $this->assertEquals($baz->add1(2), 'foo');
    }

    public function testTrue() {
        $baz = new Baz();
        $this->assertTrue($baz->add1(10));
    }

    public function testFalse() {
        $this->assertFalse(['a', 'b']);
    }
}
