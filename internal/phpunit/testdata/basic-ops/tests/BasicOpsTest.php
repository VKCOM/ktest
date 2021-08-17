<?php

use PHPUnit\Framework\TestCase;

class BasicOpsTest extends TestCase {
    public function testAssertTrueOk1() { $this->assertTrue(true); }
    public function testAssertTrueFail1() { $this->assertTrue(false); }
    public function testAssertTrueFail2() { $this->assertTrue(0); }
    public function testAssertTrueFail3() { $this->assertTrue(1); }

    public function testAssertFalseOk1() { $this->assertFalse(false); }
    public function testAssertFalseFail1() { $this->assertFalse(true); }
    public function testAssertFalseFail2() { $this->assertFalse(0); }
    public function testAssertFalseFail3() { $this->assertFalse(1); }

    public function testAssertSameOk1() { $this->assertSame(10, 10); }
    public function testAssertSameFail1() { $this->assertSame(true, false); }
    public function testAssertSameFail2() { $this->assertSame(0, false); }
    public function testAssertSameFail3() { $this->assertSame('0', 0); }

    public function testAssertNotSameOk1() { $this->assertNotSame('0', 0); }
    public function testAssertNotSameOk2() { $this->assertNotSame(0, 1); }
    public function testAssertNotSameFail1() { $this->assertNotSame(1, 1); }
    public function testAssertNotSameFail2() { $this->assertNotSame('1', '1'); }

    public function testAssertEqualsOk1() { $this->assertEquals(1, 1); }
    public function testAssertEqualsOk2() { $this->assertEquals('1', 1); }
    public function testAssertEqualsFail1() { $this->assertEquals(1, 2); }
    public function testAssertEqualsFail2() { $this->assertEquals(false, true); }

    public function testAssertNotEqualsOk1() { $this->assertNotEquals(1, 2); }
    public function testAssertNotEqualsOk2() { $this->assertNotEquals('foo', false); }
    public function testAssertNotEqualsFail1() { $this->assertNotEquals(false, 0); }
    public function testAssertNotEqualsFail2() { $this->assertNotEquals('1', 1); }
}
