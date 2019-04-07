use Test::More;

BEGIN {
  plan skip_all => "Class::Method::Modifiers not installed or too old"
    unless eval "use Class::Method::Modifiers 1.05; 1";
}

my $pass;
my $pass2;

BEGIN {
	package Local::Role;
	use Role::Tiny;
	around does => sub {
		my ($orig, $self, @args) = @_;
		$pass++;
		return $self->$orig(@args);
	};
	around DOES => sub {
		my ($orig, $self, @args) = @_;
		$pass2++;
		return $self->$orig(@args);
	};
}

BEGIN {
	package Local::Class;
	use Role::Tiny::With;
	with 'Local::Role';
}

ok(Local::Class->does('Local::Role'));
ok($pass);
ok(Local::Class->DOES('Local::Role'));
ok($pass2);
done_testing();
