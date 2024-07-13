\connect wedding;

insert into guests(invitation_id, first_name, last_name, plus_one_allowed)
values
('a34a3135-71b4-47e6-95cf-cb62cc594b5c', 'Bob', 'Couple', false),
('a34a3135-71b4-47e6-95cf-cb62cc594b5c', 'Taylor', 'Couple', false),
('8ff1cf66-91e2-45ea-a597-e43dcb8ad702', 'Ricky', 'PlusOne', true),
('f5d7118b-2ad0-4c93-8411-f57ad48b030f', 'Becky', 'NoPlusOne', false),
('2c29e01c-3ca3-431f-ba0c-60e43c9db7fd', 'Mom', 'Family', false),
('2c29e01c-3ca3-431f-ba0c-60e43c9db7fd', 'Dad', 'Family', false),
('2c29e01c-3ca3-431f-ba0c-60e43c9db7fd', 'Younger child', 'Family', false),
('2c29e01c-3ca3-431f-ba0c-60e43c9db7fd', 'Older child', 'Family', true);