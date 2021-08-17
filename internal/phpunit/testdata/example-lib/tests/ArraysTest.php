<?php

use PHPUnit\Framework\TestCase;
use ExampleLib\Arrays;

class ArraysTest extends TestCase {
    public function testIsList() {
        $this->assertSame(Arrays::isList([]), true);
        $this->assertSame(Arrays::isList([1]), true);
        $this->assertSame(Arrays::isList(['x' => 1]), false);
    }

    public function testIsAssoc() {
        $this->assertSame(Arrays::isAssoc([]), false);
        $this->assertSame(Arrays::isAssoc([1]), false);
        $this->assertSame(Arrays::isAssoc(['x' => 1]), true);
    }

    public function testFlatten() {
        $this->assertSame([1, 2, 3], Arrays::flatten([[1], [2], [3]]));
    }

    public function testCountStringKeys() {
        $this->assertEquals(0, Arrays::countStringKeys([3 => 2]));
        $this->assertEquals(1, Arrays::countStringKeys(['x' => 4, 3 => 2]));
        $this->assertEquals(2, Arrays::countStringKeys(['x' => 4, 'y' => 2]));
    }
}
