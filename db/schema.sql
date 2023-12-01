create database guests;

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

create table guests(
    id uuid not null unique default uuid_generate_v4(),
    invitation_id text not null,
    first_name text not null,
    last_name text not null,
    attending bool not null default false,
    plus_one_allowed bool not null default false,
    plus_one_attending bool not null constraint plus_one_without_guest check (not (plus_one_attending and not attending)) default false,
    notes text,
    has_rsvpd bool not null default false,
    primary key ( first_name, last_name )
)