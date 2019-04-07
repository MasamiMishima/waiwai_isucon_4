#!/usr/bin/perl
# This is automatically generated by author/import-moose-test.pl.
# DO NOT EDIT THIS FILE. ANY CHANGES WILL BE LOST!!!
use t::lib::MooseCompat;

use strict;
use warnings;

use Test::Builder::Tester;
use Test::More;

BEGIN {
  use_ok('Test::Mouse');
}

{
    package Foo;
    use Mouse::Role;
}

{
    package Bar;
    use Mouse;

    with qw/Foo/;
}

{
    package Baz;
    use Mouse;
}

# class ok

test_out('ok 1 - does_ok class');

does_ok('Bar','Foo','does_ok class');

# class fail

test_out ('not ok 2 - does_ok class fail');
test_fail (+2);

does_ok('Baz','Foo','does_ok class fail');

# object ok

my $bar = Bar->new;

test_out ('ok 3 - does_ok object');

does_ok ($bar,'Foo','does_ok object');

# object fail

my $baz = Baz->new;

test_out ('not ok 4 - does_ok object fail');
test_fail (+2);

does_ok ($baz,'Foo','does_ok object fail');

test_test ('does_ok');

done_testing;
